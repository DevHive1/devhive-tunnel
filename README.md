# devhive-tunnel 🐝

Private, end-to-end encrypted hive networks for every device you own — Windows, Linux and
Termux/Android. Create a network in one command, invite people, chat, share files, and (on
desktops) play LAN games over a real virtual LAN.

No accounts, no sign-up, no cloud dependency: you run the hub, you own the keys.

> 📘 **دليل عربي مفصّل خطوة بخطوة (بناء + تشغيل + استكشاف أخطاء):**
> [`docs/AR-BUILD-RUN.md`](docs/AR-BUILD-RUN.md)

```
            ┌──────────────┐        hub (your VPS or phone)
  device A ─┤              ├─ device B        • coordinates peers
  device C ─┤  encrypted   ├─ device D        • auth: Argon2id passwords, admin/invite keys
  device E ─┤  TCP mesh    ├─ …               • never sees plaintext chat/files/IP traffic
            └──────────────┘
```

## Features

- **Create / join in seconds** — `devtun tui` wizard (English + العربية, RTL dashboard), or
  `devtun ai` for a guided, hardened plan.
- **Built-in AI network generator** — profiles `private`, `team`, `community`, `paranoid`;
  picks subnet, max peers, relay caps, invite policy and gives a security checklist. Fully
  offline rules engine (no external LLM/API).
- **Owner dashboard** — web UI (phone + PC) for members, invites, bans, audit log, settings,
  admin-key rotation. Only the owner holds the dashboard/admin key.
- **GM superuser** — hub-level supervisor console: global ban/unban, kick, suspend/delete
  networks, analytics, full audit stream. The console is installable as a PWA and streams
  live hub metadata (SSE, 1s) — no refresh needed.
- **Free to host** — Oracle Cloud Always Free + DuckDNS + Caddy guide, one-command
  `scripts/deploy-vps.sh`, automatic HTTPS, systemd hardening, GitHub Actions release builds
  and free GitHub Releases distribution.
- **E2E chat, DMs, files** — XChaCha20-Poly1305; group messages keyed by the network PSK,
  DMs/files by pairwise X25519 static-static keys; files hashed (SHA-256) and verified.
- **LAN mode** — real TUN adapter (Linux; Windows via Wintun) on the hive subnet
  `100.x.0.0/16`, with broadcast/multicast replication so LAN discovery works for games and
  legacy apps. Termux/Android keeps chat/files without root.
- **MCP server** — `dh-mcp` exposes networks/chat/files to AI agents (Claude Desktop, Cursor,
  opencode, …) over stdio.
- **Termux-first** — prebuilt android/arm64 binaries, one-shot bootstrap script.
- **Free downloads** — releases at https://github.com/DevHive1/devhive-tunnel/releases
  (built by GitHub Actions, free download counters per asset).

## Quick start

### 1. Start a hub (anywhere reachable)

```bash
go build -o devhive-hub ./cmd/devhive-hub
./devhive-hub -listen :7676 -web :7600 -dir ~/.devhive-hub
```

On first run the hub prints and saves the **GM key** (`~/.devhive-hub/GM-KEY.txt`). Keep it
offline. Dashboard: `http://<hub>:7600` (GM login uses this key; owners log in with their
network admin key).

### 2. Create a network (owner)

```bash
go build -o devtun ./cmd/devtun
./devtun tui                 # guided wizard
# or
./devtun ai                  # AI-guided creation
# or
./devtun create --hub <hub>:7676 --name my-hive --profile private --members 6 --yes
```

The owner receives the **network ID**, **password** and **admin key** once. The admin key
opens the web dashboard and mints invites.

### 3. Invite & join

```bash
# owner mints an invite (single-use, 7 days; --role admin for an admin invite)
./devtun invite

# new member
./devtun join --hub <hub>:7676 --password '<network-password>' --admin-key '<invite-key>' my-hive
```

Already-enrolled devices reconnect with just the password; invites gate new devices only.

### 4. Talk / share / play

```bash
./devtun chat my-hive        # group chat REPL: /dm /send /invite /peers
./devtun send alice ./photo.jpg my-hive
./devtun listen my-hive      # background receive
sudo ./devtun lan my-hive    # desktop LAN mode (real TUN)
```

## Commands

| command | purpose |
|---|---|
| `devtun setup` | first-run wizard (create or join) |
| `devtun tui` | full-screen wizard + live chat UI |
| `devtun ai` | AI-guided network creation |
| `devtun create` | create with flags (`--profile`, `--members`, `--lan`, …) |
| `devtun join <name\|id>` | join a network (`--password`, `--admin-key`/invite) |
| `devtun list` | public networks on the hub |
| `devtun status` | identity fingerprint, hub, saved networks |
| `devtun chat [net]` | encrypted chat / DMs / file send REPL |
| `devtun listen [net]` | receive messages and files |
| `devtun send <peer> <file>` | encrypted file transfer |
| `devtun invite` | mint a join invite (owner) |
| `devtun lan [net]` | virtual LAN with TUN (root/Admin) |
| `devhive-hub` | coordination server + web dashboard |
| `dh-mcp` | MCP server for AI agents |

## Security model

- **Identity** — Ed25519 signing + X25519 static key per device, persisted at
  `~/.devhive-tunnel/identity.key`. Peers pin fingerprints; sessions are signed, ephemeral
  X25519 handshakes (forward secrecy).
- **Passwords** — Argon2id hashed on the hub; network PSK derived locally via Argon2id and
  never sent. The hub relays opaque blobs only.
- **Transport** — custom framed protocol over TCP, XChaCha20-Poly1305 per frame with counter,
  replay/tamper kills the session. Optional TLS between client/hub (`-tls-cert/-tls-key`).
- **Roles** — owner: admin key (dashboard, invites, bans, settings, rotation). Members: chat,
  files, LAN. GM: hub-wide supervision only (no plaintext access).
- **Audit** — per-network and hub-wide audit trails surfaced in dashboards.

## AI network generator

`pkg/aigen` is a deterministic expert system (no API keys, works offline):

| profile | invite key | public listing | max peers | join rate |
|---|---|---|---|---|
| private | required | no | ≈ members | 10/min |
| team | required | no | members + 50% | 10/min |
| community | required | yes (anonymous opt-in) | 2× members | 6/min |
| paranoid | required | no | ≈ members (min 3) | 3/min |

It also suggests a typeable strong password, a collision-resistant `/16`, relay caps, and a
bilingual rationale + checklist. Same engine powers the CLI wizard and MCP tools.

## MCP (AI agents)

`dh-mcp` speaks MCP over stdio. Example config (Claude Desktop / Cursor / opencode):

```json
{
  "mcpServers": {
    "devhive-tunnel": {
      "command": "/path/to/dh-mcp",
      "env": {}
    }
  }
}
```

Tools: `dh_status`, `dh_networks_list`, `dh_ai_plan`, `dh_network_create`,
`dh_network_join`, `dh_peers`, `dh_chat_send`, `dh_chat_read`, `dh_dm_send`,
`dh_file_send`, `dh_invite_create`.

The server uses the same `~/.devhive-tunnel` profile, so an agent can operate your device's
networks without ever receiving the admin key unless a tool returns it.

## LAN mode notes

- Linux: needs `/dev/net/tun` + root or `CAP_NET_ADMIN`; IPs are configured via `ip`.
- Windows: needs Administrator + `wintun.dll` from <https://www.wintun.net/> next to
  `devtun.exe`.
- Subnet: assigned per network (e.g. `100.78.0.0/16`). Peers are reachable at their virtual
  IPs; broadcast/multicast (LAN discovery) is replicated to all online members.
- Termux/Android: no TUN without root — chat, DMs and file transfer remain fully encrypted.
- The relay is layer-3; modern UDP/IP LAN discovery works, raw layer-2 protocols do not.

## Web dashboard

- Owner login: network ID + admin key → members, invites, bans, audit, settings, key rotation.
- GM login: GM key → global networks, analytics, bans, suspend/delete, audit.
- CSP-hardened vanilla JS, works on phone and desktop, English/العربية with RTL.
- GM console pushes live snapshots over SSE (`/api/gm/live`) and installs as a PWA
  (`/manifest.json`, offline shell). Metadata only — chat/files stay end-to-end encrypted.
- Free hosting: see [docs/AR-BUILD-RUN.md](docs/AR-BUILD-RUN.md) section 19 and
  `deploy/` + `scripts/deploy-vps.sh` (Oracle Cloud Always Free, Caddy, DuckDNS, systemd).

## Build & test

```bash
make build          # bin/devtun, bin/devhive-hub, bin/dh-mcp
make test           # unit + end-to-end suite
make e2e            # hub integration test (create→join→chat→files→invites→GM)
make dist           # cross-compile linux/windows/android + arm64
scripts/termux-bootstrap.sh   # Termux (no root)
DOMAIN=myhive.duckdns.org REPO=DevHive1/devhive-tunnel \
  scripts/deploy-vps.sh      # free VPS deploy (root; Ubuntu/Oracle)
```

## Layout

```
cmd/devtun       client CLI + TUI + LAN mode
cmd/devhive-hub  coordination server + dashboards
cmd/dh-mcp       MCP stdio server
pkg/crypto       identity, Argon2id KDF, signed sessions, XChaCha20-Poly1305
pkg/hub          store, control protocol, routing, REST (owner/GM), embedded web UI
pkg/client       hub session, peer directory, group/DM seals, file transfer, IP relay
pkg/mesh         TUN ⇄ hive bridge with broadcast replication
pkg/tun          Linux TUN / Windows Wintun adapters
pkg/aigen        offline AI network planner
pkg/tui          bubbletea wizard + chat UI
pkg/mcp          Model Context Protocol server
pkg/conf         per-user config/identity store
pkg/i18n         English/Arabic strings
```

## License

MIT — do not use for harm. You are responsible for the networks you run.
