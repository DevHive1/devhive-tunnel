#!/usr/bin/env bash
# Build devhive-tunnel for every supported target.
# Usage: scripts/build-all.sh [version]
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-0.1.0}"
GO="${GO:-go}"
OUT="dist"
rm -rf "$OUT"
mkdir -p "$OUT"

build() {
  local os="$1" arch="$2" ext="$3" targets="$4"
  local dir="$OUT/devhive-tunnel-$VERSION-$os-$arch"
  mkdir -p "$dir"
  echo "→ $os/$arch"
  for bin in $targets; do
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 "$GO" build \
      -trimpath -ldflags "-s -w -X devhive-tunnel/pkg/conf.Version=$VERSION" \
      -o "$dir/$bin$ext" "./cmd/$bin"
  done
}

# Optional subset: TARGETS="linux/arm64 windows/amd64 android/arm64"
TARGETS="${TARGETS:-linux/amd64 linux/arm64 windows/amd64 android/arm64}"
for t in $TARGETS; do
  os="${t%/*}"; arch="${t#*/}"; ext=""
  [ "$os" = "windows" ] && ext=".exe"
  build "$os" "$arch" "$ext" "devtun devhive-hub dh-mcp"
done

# Termux users run the android/arm64 binaries. TUN needs root; chat/files do not.
echo
echo "done → $OUT"
echo "Windows LAN mode also needs wintun.dll next to devtun.exe (https://www.wintun.net/)."
