#!/usr/bin/env bash
# Edge Android 交叉编译：arm64 或 arm
set -e
ARCH=${1:-arm64}
if [[ "$ARCH" != "arm64" && "$ARCH" != "arm" ]]; then
	echo "usage: $0 arm64|arm"
	exit 1
fi
export GOOS=android
if [[ "$ARCH" == "arm64" ]]; then
	export GOARCH=arm64
else
	export GOARCH=arm
fi
mkdir -p build
OUT="build/edge-android-${ARCH}"
cd "$(dirname "$0")/.."
go build -ldflags="-s -w" -o "$OUT" ./cmd/edge
echo "built $OUT"
