package kemono

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/elvis972602/kemono-scraper/utils"
)

// FetchCreators fetch Creator list
func (k *Kemono) FetchCreators() (creators []Creator, err error) {
	k.log.Print("fetching creator list...")
	url := fmt.Sprintf("https://%s.cr/api/v1/creators", k.Site)

	resp, err := k.Downloader.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch creator list error: %s", err)
	}

	reader, err := handleCompressedHTTPResponse(resp)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("fetch creator list error: %s", err)
	}

	// DEBUG: inspect what the server actually sent
	//	fmt.Println("==== DEBUG RAW CREATOR RESPONSE ====")
	//	fmt.Println("URL:", url)
	//	fmt.Println("Length:", len(data))
	//	if len(data) > 300 {
	//		fmt.Println(string(data[:300]))
	//		fmt.Println("... (truncated)")
	//	} else {
	//		fmt.Println(string(data))
	//	}
	//	fmt.Println("==== END DEBUG RAW CREATOR RESPONSE ====")

	// now try to parse it
	err = json.Unmarshal(data, &creators)
	if err != nil {
		return nil, fmt.Errorf("unmarshal creator list error: %s", err)
	}
	return
}

// FetchPosts fetch post list
func (k *Kemono) FetchPosts(service, id string) (posts []Post, err error) {
	url := fmt.Sprintf("https://%s.cr/api/v1/%s/user/%s/posts", k.Site, service, id)
	perUnit := 50

	fetch := func(page int) (err error, finish bool) {
		k.log.Printf("fetching post list page %d...", page)
		purl := fmt.Sprintf("%s?o=%d", url, page*perUnit)

		retryCount := 0
		for retryCount < k.retry {
			resp, err := k.Downloader.Get(purl)
			if err != nil {
				k.log.Printf("fetch post list error: %v", err)
				time.Sleep(k.retryInterval)
				retryCount++
				continue
			}

			// decompress + read body
			reader, err := handleCompressedHTTPResponse(resp)
			if err != nil {
				k.log.Printf("handle response error: %v", err)
				time.Sleep(k.retryInterval)
				retryCount++
				continue
			}

			data, err := ioutil.ReadAll(reader)
			reader.Close()
			if err != nil {
				k.log.Printf("read post list error: %v", err)
				time.Sleep(k.retryInterval)
				retryCount++
				continue
			}

			// handle non-200 responses
			if resp.StatusCode != http.StatusOK {
				msg := string(data)

				// 400: "Offset X is bigger than total count Y." → no more pages
				if resp.StatusCode == http.StatusBadRequest &&
					strings.Contains(msg, "Offset") &&
					strings.Contains(msg, "bigger than total count") {
					k.log.Printf("no more posts (page %d): %s", page, msg)
					return nil, true
				}

				// 404: treat as "no posts" for this creator
				if resp.StatusCode == http.StatusNotFound {
					k.log.Printf("no posts for %s/%s (page %d): %s", service, id, page, msg)
					return nil, true
				}

				// anything else: real error
				return fmt.Errorf("fetch post list error: status %d, body: %s", resp.StatusCode, msg), true
			}

			// normal OK path
			var pr []PostRaw
			if err := json.Unmarshal(data, &pr); err != nil {
				return fmt.Errorf("unmarshal post list error: %s", err), true
			}

			if len(pr) == 0 {
				// final page
				return nil, true
			}

			for _, p := range pr {
				posts = append(posts, p.ParasTime())
			}

			// if fewer than perUnit, we’re on last page anyway
			if len(pr) < perUnit {
				return nil, true
			}

			return nil, false
		}

		return fmt.Errorf("fetch post list error: maximum retry count exceeded"), true
	}

	for i := 0; ; i++ {
		err, finish := fetch(i)
		if err != nil {
			return nil, err
		}
		if finish {
			break
		}
	}
	return
}

// DownloadPosts download posts
func (k *Kemono) DownloadPosts(creator Creator, posts []Post) (err error) {
	for _, post := range posts {
		k.log.Printf("download post: %s", utils.ValidDirectoryName(post.Title))
		if post.Content != "" {
			err = k.Downloader.WriteContent(creator, post, post.Content)
			if err != nil {
				k.log.Printf("write content error: %s", err)
			}
		}
		if len(post.Attachments) == 0 {
			// no attachment
			continue
		}
		attachmentsChan := make(chan FileWithIndex, len(post.Attachments))
		for _, a := range AddIndexToAttachments(post.Attachments) {
			attachmentsChan <- a
		}
		errChan := k.Downloader.Download(attachmentsChan, creator, post)
		for i := 0; i < len(errChan); i++ {
			err, ok := <-errChan
			if ok {
				k.log.Printf("download post error: %s", err)
				// TODO: record error...
			} else {
				break
			}
		}
	}
	return
}

func handleCompressedHTTPResponse(resp *http.Response) (io.ReadCloser, error) {
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return reader, nil
	default:
		return resp.Body, nil
	}
}

func AddIndexToAttachments(attachments []File) []FileWithIndex {
	var files []FileWithIndex
	images := 0
	others := 0
	for _, a := range attachments {
		ext := filepath.Ext(a.Name)
		if ext == "" {
			ext = filepath.Ext(a.Path)
		}
		if isImage(ext) {
			files = append(files, a.Index(images))
			images++
		} else {
			files = append(files, a.Index(others))
			others++
		}
	}
	return files
}
