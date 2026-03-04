#!/bin/bash
set -e

mkdir -p ./bin

PLATFORMS=(
  "linux/arm64"
  "linux/amd64"
  "darwin/arm64"
  "darwin/amd64"
  "windows/amd64"
  "windows/arm64"
)

for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  os_name="$GOOS"
  if [ "$GOOS" = "darwin" ]; then
    os_name="macos"
  fi
  output="./bin/selfmd-${os_name}-${GOARCH}"
  if [ "$GOOS" = "windows" ]; then
    output="${output}.exe"
  fi
  echo "Building ${output}..."
  GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$output" .
done

echo ""
echo "All builds complete:"
ls -lh ./bin/
