// Package hub implements the devhive-hub coordination server: network
// registry, join authentication, peer directory, admin invitations, audit
// logging and optional routed signaling for peer-to-peer sessions.
package hub

import "time"

// WireType is the framed application message type over the control link.
type WireType byte

// Control-link message types.
const (
	WCreate    WireType = 0x10 // create network
	WCreateOK  WireType = 0x11
	WJoin      WireType = 0x12 // join by name or id
	WJoinOK    WireType = 0x13
	WAuthFail  WireType = 0x14 // wrong credentials / banned
	WList      WireType = 0x20 // searchable network list (public nets only)
	WListOK    WireType = 0x21
	WRenew     WireType = 0x25 // keepalive/state sync → WJoinOK again
	WPeers     WireType = 0x27 // full directory refresh
	WInvite    WireType = 0x30 // create invite (admin) → WInviteOK
	WInviteOK  WireType = 0x31
	WPeerEvent WireType = 0x40 // push: peer join/leave/update (hub→client)
	WRoute     WireType = 0x50 // client→hub→client signaling envelope
	WSysErr    WireType = 0xF0
)

// NetSettings are the advanced options for a network.
type NetSettings struct {
	RequireInviteKey bool   `json:"require_invite_key"` // join needs admin key
	InviteKeyHash    string `json:"invite_key_hash,omitempty"`
	SubnetCIDR       string `json:"subnet_cidr,omitempty"` // e.g. 100.72.0.0/16
	MaxPeers         int    `json:"max_peers,omitempty"`
	MaxUploadMbps    int    `json:"max_upload_mbps,omitempty"` // relay shaping
	AllowAnonymous   bool   `json:"allow_anonymous_discovery"` // listed on hub
	RateJoinPerMin   int    `json:"rate_join_per_min,omitempty"`
	AllowRelay       bool   `json:"allow_relay"`
}

// NetRecord stored on the hub (never contains raw passwords).
type NetRecord struct {
	ID          string                 `json:"id"`         // unique "dh-XXXXXXXX"
	Name        string                 `json:"name"`       // joinable name
	PSKHash     string                 `json:"psk_hash"`   // salted hash of derived PSK
	AdminHash   string                 `json:"admin_hash"` // hash of admin key
	CreatorFP   string                 `json:"creator_fp"` // fingerprint of the creator
	Created     string                 `json:"created"`    // RFC3339
	Blocked     bool                   `json:"blocked"`    // GM suspended this network
	BlockReason string                 `json:"block_reason,omitempty"`
	BlockedAt   string                 `json:"blocked_at,omitempty"`
	Settings    NetSettings            `json:"settings"`
	Invites     []Invite               `json:"invites,omitempty"`
	Peers       map[string]*PeerRecord `json:"peers,omitempty"`
	Audit       []AuditLine            `json:"audit,omitempty"`
	NextVLAN    int                    `json:"next_vlan"`
}

// Invite grants an extra admission ticket (single or multi-use, expiring).
type Invite struct {
	Code    string `json:"code"`     // display code (not secret)
	KeyHash string `json:"key_hash"` // hash of the secret invite key
	Note    string `json:"note,omitempty"`
	UsesMax int    `json:"uses_max"` // 0 = unlimited
	Uses    int    `json:"uses"`
	Expires string `json:"expires,omitempty"` // RFC3339 or ""
	Role    string `json:"role"`              // member | admin
}

// Active reports whether the invite can still be used.
func (inv *Invite) Active() bool {
	if inv.Expires != "" {
		if t, err := time.Parse(time.RFC3339, inv.Expires); err != nil || time.Now().After(t) {
			return false
		}
	}
	if inv.UsesMax > 0 && inv.Uses >= inv.UsesMax {
		return false
	}
	return true
}

// PeerRecord is a member identity.
type PeerRecord struct {
	Fingerprint string `json:"fingerprint"` // hex of sha256(ed)[..6]
	EdPub       string `json:"ed_pub"`      // b64 Ed25519 public key
	XPub        string `json:"x_pub"`       // b64 X25519 static key
	Nickname    string `json:"nickname"`
	Admin       bool   `json:"admin,omitempty"`
	Banned      bool   `json:"banned,omitempty"`
	BanReason   string `json:"ban_reason,omitempty"`
	VLANIP      string `json:"vlan_ip,omitempty"` // assigned VIP in subnet
	Created     string `json:"created"`
	LastSeen    string `json:"last_seen"`
	Online      bool   `json:"online"`
	Platform    string `json:"platform,omitempty"`
}

// AuditLine is one signed audit trail entry.
type AuditLine struct {
	TS     string `json:"ts"`
	Actor  string `json:"actor"` // fingerprint or "hub"
	Event  string `json:"event"` // JOIN_OK, KICK, INVITE_CREATED, ...
	Detail string `json:"detail,omitempty"`
}

// ServerInfo in WCreateOK / WJoinOK payload.
type JoinInfo struct {
	NetworkID  string        `json:"network_id"`
	Name       string        `json:"network_name"`
	Settings   NetSettings   `json:"settings"`
	Peers      []*PeerRecord `json:"peers"`
	IsAdmin    bool          `json:"is_admin"`
	YourFP     string        `json:"your_fingerprint"`
	YourVLANIP string        `json:"your_vlan_ip,omitempty"`
}

// RouteEnvelop wraps signaling and app frames routed through the hub.
type RouteEnvelope struct {
	ToFP   string `json:"to_fp,omitempty"`   // empty = broadcast to network
	FromFP string `json:"from_fp,omitempty"` // filled by hub
	Kind   string `json:"kind"`              // signal names: punch, offer, chat, file-chunk...
	Blob   []byte `json:"blob,omitempty"`    // e2e-encrypted opaque payload
}
