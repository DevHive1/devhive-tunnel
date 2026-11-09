#!/usr/bin/env bash
# Refresh a DuckDNS A record with this server's public IP.
# Environment (from /etc/devhive/duckdns.env):
#   DUCKDNS_DOMAIN=myhive          (subdomain only, no .duckdns.org)
#   DUCKDNS_TOKEN=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
set -euo pipefail
[ -f /etc/devhive/duckdns.env ] && . /etc/devhive/duckdns.env
: "${DUCKDNS_DOMAIN:?}" "${DUCKDNS_TOKEN:?}"
curl -fsS --max-time 15 \
  "https://www.duckdns.org/update?domains=${DUCKDNS_DOMAIN}&token=${DUCKDNS_TOKEN}&ip=" \
  >/dev/null
