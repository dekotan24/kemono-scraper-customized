cd %~dp0main
set GOOS=windows
set GOARCH=amd64
"C:\Program Files\Go\bin\go.exe" build -o kemono-scraper.exe
pause
