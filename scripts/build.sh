#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
PROJECT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
mkdir -p "$PROJECT_DIR/dist"
cd "$PROJECT_DIR"

build() {
	name="$1"; os="$2"; arch="$3"; arm="${4:-}"; mips="${5:-}"
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" GOARM="$arm" GOMIPS="$mips" \
		go build -trimpath -ldflags="-s -w -X main.version=$VERSION" \
		-o "dist/santak-nut-bridge-$name" .
}

build linux-amd64 linux amd64
build linux-arm64 linux arm64
build linux-armv7 linux arm 7
build linux-mipsle-softfloat linux mipsle "" softfloat

(
	cd dist
	sha256sum santak-nut-bridge-* > SHA256SUMS
)
