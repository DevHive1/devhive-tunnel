#!/usr/bin/env bash
# devhive-tunnel · one-command free VPS deploy
#
# Tested target: Ubuntu 22.04/24.04 on Oracle Cloud Always Free (amd64/arm64).
# Installs: devhive-hub, Caddy (automatic HTTPS), systemd unit, ufw rules,
#           optional DuckDNS dynamic DNS updater.
#
# Usage (as root):
#   DOMAIN=myhive.duckdns.org EMAIL=me@example.com REPO=DevHive1/devhive-tunnel \
#     bash scripts/deploy-vps.sh
#
# Or with a prebuilt binary instead of GitHub Releases:
#   DOMAIN=hub.example.com BINARY=./dist/.../devhive-hub bash scripts/deploy-vps.sh
#
# Optional variables:
#   REPO=owner/repo            GitHub repository with release assets
#   VERSION=latest             release tag to install (e.g. 0.2.0)
#   DUCKDNS_DOMAIN=myhive      DuckDNS subdomain (without .duckdns.org)
#   DUCKDNS_TOKEN=xxxx         DuckDNS token (pairs with DUCKDNS_DOMAIN)
#   CONTROL_PORT=7676          encrypted client port (must be reachable)
#   WEB_PORT=7600              local dashboard port (proxied by Caddy)
set -euo pipefail

DOMAIN="${DOMAIN:-}"
EMAIL="${EMAIL:-}"
REPO="${REPO:-}"
VERSION="${VERSION:-latest}"
BINARY="${BINARY:-}"
DUCKDNS_DOMAIN="${DUCKDNS_DOMAIN:-}"
DUCKDNS_TOKEN="${DUCKDNS_TOKEN:-}"
CONTROL_PORT="${CONTROL_PORT:-7676}"
WEB_PORT="${WEB_PORT:-7600}"

log() { printf '\033[1;33m[deploy]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[deploy] error:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root (sudo -i, then rerun)"
[ -n "$DOMAIN" ] || die "DOMAIN is required (e.g. myhive.duckdns.org)"
[ -n "$BINARY" ] || [ -n "$REPO" ] || die "set REPO=owner/repo or BINARY=/path/to/devhive-hub"

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

log "installing base packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl ca-certificates ufw debian-keyring debian-archive-keyring \
  apt-transport-https gpg lsb-release

if ! command -v caddy >/dev/null 2>&1; then
  log "adding the official Caddy repository"
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' |
    gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    >/etc/apt/sources.list.d/caddy-stable.list
  apt-get update -qq
  apt-get install -y -qq caddy
fi

log "creating the devhive service account"
id -u devhive >/dev/null 2>&1 || useradd --system --home /var/lib/devhive-hub \
  --shell /usr/sbin/nologin devhive
install -d -m 0755 -o devhive -g devhive /opt/devhive /var/lib/devhive-hub

if [ -n "$BINARY" ]; then
  [ -f "$BINARY" ] || die "BINARY not found: $BINARY"
  install -m 0755 "$BINARY" /opt/devhive/devhive-hub
else
  BASE="https://github.com/$REPO/releases/latest/download"
  if [ "$VERSION" != "latest" ]; then
    BASE="https://github.com/$REPO/releases/download/v${VERSION#v}"
  fi
  log "downloading devhive-hub-linux-$ARCH from $BASE"
  TMP="$(mktemp)"
  curl -fL --retry 3 --connect-timeout 15 -o "$TMP" "$BASE/devhive-hub-linux-$ARCH" ||
    die "download failed; check REPO/VERSION or use BINARY="
  install -m 0755 "$TMP" /opt/devhive/devhive-hub
  rm -f "$TMP"
fi
chown root:root /opt/devhive/devhive-hub

log "writing the systemd unit"
cat >/etc/systemd/system/devhive-hub.service <<UNIT
[Unit]
Description=devhive-tunnel hub (encrypted mesh control plane)
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=devhive
Group=devhive
ExecStart=/opt/devhive/devhive-hub -dir /var/lib/devhive-hub -listen :$CONTROL_PORT -web 127.0.0.1:$WEB_PORT
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/devhive-hub
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now devhive-hub

if [ -n "$DUCKDNS_DOMAIN" ] || [ -n "$DUCKDNS_TOKEN" ]; then
  [ -n "$DUCKDNS_DOMAIN" ] && [ -n "$DUCKDNS_TOKEN" ] ||
    die "set both DUCKDNS_DOMAIN and DUCKDNS_TOKEN, or neither"
  log "configuring DuckDNS ($DUCKDNS_DOMAIN.duckdns.org)"
  install -d -m 0755 /etc/devhive
  cat >/etc/devhive/duckdns.env <<EOF
DUCKDNS_DOMAIN=$DUCKDNS_DOMAIN
DUCKDNS_TOKEN=$DUCKDNS_TOKEN
EOF
  chmod 600 /etc/devhive/duckdns.env
  cat >/usr/local/bin/devhive-duckdns <<'DNS'
#!/usr/bin/env bash
set -euo pipefail
[ -f /etc/devhive/duckdns.env ] && . /etc/devhive/duckdns.env
: "${DUCKDNS_DOMAIN:?}" "${DUCKDNS_TOKEN:?}"
curl -fsS --max-time 15 \
  "https://www.duckdns.org/update?domains=${DUCKDNS_DOMAIN}&token=${DUCKDNS_TOKEN}&ip=" >/dev/null
DNS
  chmod 755 /usr/local/bin/devhive-duckdns
  cat >/etc/systemd/system/devhive-duckdns.service <<'UNIT'
[Unit]
Description=DuckDNS dynamic DNS update for devhive-tunnel
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
ExecStart=/usr/local/bin/devhive-duckdns
UNIT
  cat >/etc/systemd/system/devhive-duckdns.timer <<'UNIT'
[Unit]
Description=Refresh DuckDNS record every 5 minutes
[Timer]
OnBootSec=1min
OnUnitActiveSec=5min
AccuracySec=30s
[Install]
WantedBy=timers.target
UNIT
  systemctl daemon-reload
  systemctl enable --now devhive-duckdns.timer
  /usr/local/bin/devhive-duckdns || log "warning: first DuckDNS update failed; the timer will retry"
  sleep 3
fi

log "writing the Caddyfile for $DOMAIN"
{
  if [ -n "$EMAIL" ]; then
    printf '{\n\temail %s\n}\n\n' "$EMAIL"
  fi
  cat <<CADDY
$DOMAIN {
    encode zstd gzip
    reverse_proxy 127.0.0.1:$WEB_PORT {
        flush_interval -1
    }
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "no-referrer"
    }
}
CADDY
} >/etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null
systemctl enable caddy
systemctl restart caddy

log "configuring the firewall"
ufw allow 22/tcp >/dev/null || true
ufw allow 80/tcp >/dev/null
ufw allow 443/tcp >/dev/null
ufw allow "$CONTROL_PORT/tcp" >/dev/null
ufw --force enable >/dev/null
log "firewall: 22, 80, 443, $CONTROL_PORT/tcp open"

if command -v systemctl >/dev/null 2>&1; then
  sleep 1
  systemctl --no-pager --lines=0 status devhive-hub >/dev/null 2>&1 ||
    die "devhive-hub failed to start; run: journalctl -u devhive-hub -n 50"
fi

cat <<DONE

  devhive-tunnel is live.

  Dashboard : https://$DOMAIN
  Control   : $DOMAIN:$CONTROL_PORT  (clients use --hub $DOMAIN:$CONTROL_PORT)
  GM key    : sudo cat /var/lib/devhive-hub/GM-KEY.txt

  Save the GM key offline. Next steps:
    1. In the dashboard, sign in with the GM key and create a network.
    2. Share the network name + password with your subscribers.
    3. Open $CONTROL_PORT/tcp in your cloud firewall/VCN security list too
       (Oracle Cloud: Networking -> VCN -> Security Lists -> Ingress Rules).

  Updates: rerun this script (or: systemctl stop devhive-hub, replace
  /opt/devhive/devhive-hub, systemctl start devhive-hub). Data is kept in
  /var/lib/devhive-hub; back it up.

DONE
