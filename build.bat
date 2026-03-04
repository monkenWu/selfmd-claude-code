@echo off
if not exist bin mkdir bin

echo Building linux/arm64...
set GOOS=linux& set GOARCH=arm64& go build -o ./bin/selfmd-linux-arm64 .

echo Building linux/amd64...
set GOOS=linux& set GOARCH=amd64& go build -o ./bin/selfmd-linux-amd64 .

echo Building darwin/arm64...
set GOOS=darwin& set GOARCH=arm64& go build -o ./bin/selfmd-macos-arm64 .

echo Building darwin/amd64...
set GOOS=darwin& set GOARCH=amd64& go build -o ./bin/selfmd-macos-amd64 .

echo Building windows/amd64...
set GOOS=windows& set GOARCH=amd64& go build -o ./bin/selfmd-windows-amd64.exe .

echo Building windows/arm64...
set GOOS=windows& set GOARCH=arm64& go build -o ./bin/selfmd-windows-arm64.exe .

echo.
echo All builds complete:
dir /b bin\
