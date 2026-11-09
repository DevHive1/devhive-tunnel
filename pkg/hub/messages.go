package hub

import (
	"time"
)

// CreateReq creates a new network (first member becomes admin).
type CreateReq struct {
	Name     string       `json:"name"`
	Password string       `json:"password"`
	Nickname string       `json:"nickname"`
	Platform string       `json:"platform,omitempty"`
	Settings *NetSettings `json:"settings,omitempty"`
}

// CreateResp returns the network + one-time admin key.
type CreateResp struct {
	JoinInfo
	AdminKey string `json:"admin_key"`
}

// JoinReq authenticates against a network (name OR ID).
type JoinReq struct {
	Network  string `json:"network"`
	Password string `json:"password"`
	AdminKey string `json:"admin_key,omitempty"`
	Nickname string `json:"nickname"`
	Platform string `json:"platform,omitempty"`
}

// JoinResp is the full post-join state.
type JoinResp struct {
	JoinInfo
}

// ListReq searches public networks.
type ListReq struct {
	Query string `json:"query,omitempty"`
}

// ListResp lists public networks (dashboard-safe shape).
type ListResp struct {
	Nets []ListEntry `json:"nets"`
}

// ListEntry is one searchable network summary.
type ListEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Online int    `json:"online"`
}

// InviteReq asks for a new invite (admins only).
type InviteReq struct {
	Note        string `json:"note,omitempty"`
	UsesMax     int    `json:"uses_max,omitempty"` // 0 = unlimited
	ExpiresDays int    `json:"expires_days,omitempty"`
	Role        string `json:"role,omitempty"` // member|admin
}

// InviteResp returns created invite credentials.
type InviteResp struct {
	Code string `json:"code"`
	Role string `json:"role"`
	Note string `json:"note,omitempty"`
}

// PeerEvent is pushed to members on directory changes.
type PeerEvent struct {
	Event string      `json:"event"`
	Peer  *PeerRecord `json:"peer,omitempty"`
}

// nowRFC3339 is the server timestamp helper.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
