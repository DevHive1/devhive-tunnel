#!/data/data/com.termux/files/usr/bin/bash
# One-shot bootstrap for Termux (no root): builds devtun + hub for Android.
set -euo pipefail

echo "🐝 devhive-tunnel Termux bootstrap"

if ! command -v go >/dev/null 2>&1; then
  echo "→ installing Go…"
  pkg update -y
  pkg install -y golang git
fi

echo "→ go version: $(go version)"

cd "$(dirname "$0")/.." 2>/dev/null || cd "$(pwd)"

echo "→ building for android/arm64 (CGO off)…"
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bin/devtun ./cmd/devtun
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bin/devhive-hub ./cmd/devhive-hub
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bin/dh-mcp ./cmd/dh-mcp

mkdir -p "$PREFIX/bin"
cp bin/devtun bin/devhive-hub bin/dh-mcp "$PREFIX/bin/"
chmod +x "$PREFIX/bin/devtun" "$PREFIX/bin/devhive-hub" "$PREFIX/bin/dh-mcp"

cat <<'EOF'

✓ installed: devtun, devhive-hub, dh-mcp

Next:
  devtun tui                     # wizard (Arabic/English)
  devhive-hub -listen :7676 -web :7600
  devtun ai                      # AI-guided network creation

Notes:
  • Chat, DMs and encrypted file sharing work fully on Termux (no root).
  • LAN mode needs /dev/net/tun (rooted device or proot-distro with TUN).
  • Keep the hub reachable: host it on a VPS or run it on a phone with a port.
EOF
