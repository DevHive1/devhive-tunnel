package hub

import (
	"net/http"
	"time"

	"devhive-tunnel/pkg/crypto"
)

// netView is the admin-visible network snapshot.
type netView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Created  string        `json:"created"`
	Blocked  bool          `json:"blocked"`
	Settings NetSettings   `json:"settings"`
	Peers    []*PeerRecord `json:"peers"`
	Invites  []inviteView  `json:"invites"`
	Audit    []AuditLine   `json:"audit"`
	Online   int           `json:"online"`
	Total    int           `json:"total"`
}

type inviteView struct {
	Code    string `json:"code"`
	Note    string `json:"note"`
	UsesMax int    `json:"uses_max"`
	Uses    int    `json:"uses"`
	Expires string `json:"expires"`
	Role    string `json:"role"`
	Active  bool   `json:"active"`
}

func netViewOf(n *NetRecord) netView {
	v := netView{
		ID: n.ID, Name: n.Name, Created: n.Created, Blocked: n.Blocked,
		Settings: n.Settings, Peers: n.PeerMapSnapshot(), Online: n.OnlinePeers(),
		Total: len(n.Peers), Audit: append([]AuditLine(nil), n.Audit...),
	}
	for _, inv := range n.Invites {
		v.Invites = append(v.Invites, inviteView{
			Code: "****" + inv.Code, Note: inv.Note, UsesMax: inv.UsesMax,
			Uses: inv.Uses, Expires: inv.Expires, Role: inv.Role, Active: inv.Active(),
		})
	}
	return v
}

// adminNet returns the network snapshot for the admin dashboard.
func (a *api) adminNet(w http.ResponseWriter, r *http.Request, tk token) {
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	v := netViewOf(n)
	a.srv.mu.Unlock()
	writeJSON(w, 200, v)
}

// adminSettings updates the advanced network options.
func (a *api) adminSettings(w http.ResponseWriter, r *http.Request, tk token) {
	var upd NetSettings
	if err := readJSON(r, &upd); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	if upd.SubnetCIDR != "" {
		if _, err := netipParse(upd.SubnetCIDR); err != nil {
			a.srv.mu.Unlock()
			writeJSON(w, 400, map[string]string{"error": "bad_subnet"})
			return
		}
		n.Settings.SubnetCIDR = upd.SubnetCIDR
	}
	n.Settings.RequireInviteKey = upd.RequireInviteKey
	n.Settings.AllowAnonymous = upd.AllowAnonymous
	n.Settings.AllowRelay = upd.AllowRelay
	if upd.MaxPeers >= 0 {
		n.Settings.MaxPeers = upd.MaxPeers
	}
	if upd.MaxUploadMbps >= 0 {
		n.Settings.MaxUploadMbps = upd.MaxUploadMbps
	}
	n.Settings.RateJoinPerMin = upd.RateJoinPerMin
	n.AppendAudit("dashboard", "SETTINGS_UPDATED", "")
	_ = a.srv.store.Save()
	v := netViewOf(n)
	a.srv.mu.Unlock()
	writeJSON(w, 200, v)
}

// adminRotate generates a new admin key (returned once).
func (a *api) adminRotate(w http.ResponseWriter, r *http.Request, tk token) {
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	key := crypto.NewAdminKey()
	n.AdminHash = crypto.HashAdminKey(key)
	n.AppendAudit("dashboard", "ADMIN_KEY_ROTATED", "")
	_ = a.srv.store.Save()
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]string{"admin_key": key})
}

// adminInvites lists invites.
func (a *api) adminInvites(w http.ResponseWriter, r *http.Request, tk token) {
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	v := netViewOf(n)
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"invites": v.Invites})
}

// adminInviteCreate adds a new invite.
func (a *api) adminInviteCreate(w http.ResponseWriter, r *http.Request, tk token) {
	var req InviteReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	secret := crypto.NewAdminKey()
	role := req.Role
	if role != "admin" {
		role = "member"
	}
	exp := ""
	if req.ExpiresDays > 0 {
		exp = time.Now().UTC().AddDate(0, 0, req.ExpiresDays).Format(time.RFC3339)
	}
	n.Invites = append(n.Invites, Invite{
		Code: secret[len(secret)-4:], KeyHash: crypto.HashAdminKey(secret),
		Note: req.Note, UsesMax: req.UsesMax, Expires: exp, Role: role,
	})
	n.AppendAudit("dashboard", "INVITE_CREATED", "role="+role)
	_ = a.srv.store.Save()
	a.srv.mu.Unlock()
	writeJSON(w, 200, InviteResp{Code: secret, Role: role, Note: req.Note})
}

// adminInviteRevoke removes an invite by its display code suffix.
func (a *api) adminInviteRevoke(w http.ResponseWriter, r *http.Request, tk token) {
	var req struct {
		Code string `json:"code"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	keep := n.Invites[:0]
	revoked := false
	for _, inv := range n.Invites {
		if inv.Code == req.Code || ("****"+inv.Code) == req.Code {
			revoked = true
			continue
		}
		keep = append(keep, inv)
	}
	n.Invites = keep
	if revoked {
		n.AppendAudit("dashboard", "INVITE_REVOKED", "")
		_ = a.srv.store.Save()
	}
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"revoked": revoked})
}

// kickLocked closes live links for fp inside netID (mu must be held by caller).
func (s *Server) kickLocked(netID, fp string) bool {
	kicked := false
	if netID != "" {
		if l := s.live[netID][fp]; l != nil {
			_ = l.sess.RawConn().Close()
			delete(s.live[netID], fp)
			kicked = true
		}
	} else {
		for _, mm := range s.live {
			if l := mm[fp]; l != nil {
				_ = l.sess.RawConn().Close()
				delete(mm, fp)
				kicked = true
			}
		}
	}
	if n, ok := s.store.Nets[netID]; ok {
		if p := n.Peers[fp]; p != nil {
			p.Online = false
			p.LastSeen = nowRFC3339()
		}
	}
	return kicked
}

// peerAction validates and applies kick/ban/unban/promote.
func (a *api) peerAction(w http.ResponseWriter, r *http.Request, tk token, apply func(n *NetRecord, p *PeerRecord) error) {
	var req struct {
		FP     string `json:"fp"`
		Reason string `json:"reason"`
		Admin  *bool  `json:"admin"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	p := n.Peers[req.FP]
	if p == nil {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "peer_not_found"})
		return
	}
	if err := apply(n, p); err != nil {
		a.srv.mu.Unlock()
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	_ = a.srv.store.Save()
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func (a *api) adminKick(w http.ResponseWriter, r *http.Request, tk token) {
	var req struct {
		FP string `json:"fp"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok || n.Peers[req.FP] == nil {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "peer_not_found"})
		return
	}
	kicked := a.srv.kickLocked(tk.netID, req.FP)
	n.AppendAudit("dashboard", "KICK", req.FP)
	_ = a.srv.store.Save()
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"ok": "1", "kicked": kicked})
}

func (a *api) adminBan(w http.ResponseWriter, r *http.Request, tk token) {
	var req struct {
		FP     string `json:"fp"`
		Reason string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok || n.Peers[req.FP] == nil {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "peer_not_found"})
		return
	}
	p := n.Peers[req.FP]
	p.Banned = true
	p.BanReason = req.Reason
	a.srv.kickLocked(tk.netID, req.FP)
	n.AppendAudit("dashboard", "BAN", req.FP+" "+req.Reason)
	_ = a.srv.store.Save()
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func (a *api) adminUnban(w http.ResponseWriter, r *http.Request, tk token) {
	var req struct {
		FP string `json:"fp"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok || n.Peers[req.FP] == nil {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "peer_not_found"})
		return
	}
	p := n.Peers[req.FP]
	p.Banned = false
	p.BanReason = ""
	n.AppendAudit("dashboard", "UNBAN", req.FP)
	_ = a.srv.store.Save()
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func (a *api) adminPromote(w http.ResponseWriter, r *http.Request, tk token) {
	var req struct {
		FP    string `json:"fp"`
		Admin bool   `json:"admin"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok || n.Peers[req.FP] == nil {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "peer_not_found"})
		return
	}
	p := n.Peers[req.FP]
	if p.Fingerprint == n.CreatorFP && !req.Admin {
		a.srv.mu.Unlock()
		writeJSON(w, 400, map[string]string{"error": "cannot_demote_creator"})
		return
	}
	p.Admin = req.Admin
	n.AppendAudit("dashboard", "ROLE_CHANGE", req.FP)
	_ = a.srv.store.Save()
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// adminAudit returns the latest network audit lines.
func (a *api) adminAudit(w http.ResponseWriter, r *http.Request, tk token) {
	a.srv.mu.Lock()
	n, ok := a.srv.store.Nets[tk.netID]
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "network_gone"})
		return
	}
	lines := append([]AuditLine(nil), n.Audit...)
	a.srv.mu.Unlock()
	if len(lines) > 300 {
		lines = lines[len(lines)-300:]
	}
	writeJSON(w, 200, map[string]interface{}{"audit": lines})
}
