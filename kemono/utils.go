package kemono

import (
	"path/filepath"
	"regexp"
	"strings"
)

func isImage(ext string) bool {
	switch ext {
	case ".apng", ".avif", ".bmp", ".gif", ".ico", ".cur", ".jpg", ".jpeg", ".jfif", ".pjpeg", ".pjp", ".png", ".svg", ".tif", ".tiff", ".webp", ".jpe":
		return true
	default:
		return false
	}
}

// ExtractImagesFromContent extracts image URLs from HTML content
// and returns them as File slice
func ExtractImagesFromContent(content string) []File {
	if content == "" {
		return nil
	}

	var files []File
	seen := make(map[string]bool)

	// Match <img src="..."> patterns
	// Handles both relative paths like /xx/yy/hash.jpg and full URLs
	imgRegex := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
	matches := imgRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		src := match[1]

		// Skip if already seen
		if seen[src] {
			continue
		}
		seen[src] = true

		// Skip external URLs (we only want kemono-hosted images)
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			continue
		}

		// Extract filename from path
		filename := filepath.Base(src)
		if filename == "" || filename == "." {
			continue
		}

		// Create File struct
		files = append(files, File{
			Name: filename,
			Path: src,
		})
	}

	return files
}
