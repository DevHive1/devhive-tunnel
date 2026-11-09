package hub

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"devhive-tunnel/pkg/crypto"
)

// gmOverview is the superuser dashboard landing payload.
type gmOverview struct {
	Version        string `json:"version"`
	UptimeSec      int64  `json:"uptime_sec"`
	Started        string `json:"started"`
	Stats          Stats  `json:"stats"`
	Networks       int    `json:"networks"`
	PeersTotal     int    `json:"peers_total"`
	PeersOnline    int    `json:"peers_online"`
	GlobalBans     int    `json:"global_bans"`
	BlockedNets    int    `json:"blocked_nets"`
	HubFingerprint string `json:"hub_fingerprint"`
	Subnet         string `json:"default_subnet"`
}

func (a *api) gmOverview(w http.ResponseWriter, r *http.Request, _ token) {
	s := a.srv
	s.mu.Lock()
	ov := s.overviewLocked()
	s.mu.Unlock()
	writeJSON(w, 200, ov)
}

// gmNetSummary is a compact network row for the GM list.
type gmNetSummary struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Created     string      `json:"created"`
	Blocked     bool        `json:"blocked"`
	BlockReason string      `json:"block_reason,omitempty"`
	Total       int         `json:"total"`
	Online      int         `json:"online"`
	Admins      int         `json:"admins"`
	Settings    NetSettings `json:"settings"`
	CreatorFP   string      `json:"creator_fp"`
}

func (a *api) gmNetworks(w http.ResponseWriter, r *http.Request, _ token) {
	s := a.srv
	s.mu.Lock()
	out := s.summariesLocked()
	s.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"networks": out})
}

// gmNetwork returns a single network with peers and audit.
func (a *api) gmNetwork(w http.ResponseWriter, r *http.Request, _ token) {
	id := r.URL.Query().Get("network")
	a.srv.mu.Lock()
	n, ok := a.srv.store.Lookup(id)
	if !ok {
		a.srv.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	v := netViewOf(n)
	a.srv.mu.Unlock()
	writeJSON(w, 200, v)
}

// gmAnalytics aggregates usage + security signals.
type gmAnalytics struct {
	EventsPerHour map[string]int `json:"events_per_hour"`
	TopEvents     []eventCount   `json:"top_events"`
	NetworkUsage  []netUsage     `json:"network_usage"`
	Security      securityStats  `json:"security"`
}

type eventCount struct {
	Event string `json:"event"`
	Count int    `json:"count"`
}

type netUsage struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Online  int    `json:"online"`
	Total   int    `json:"total"`
	Blocked bool   `json:"blocked"`
}

type securityStats struct {
	AuthFails    uint64 `json:"auth_fails"`
	GlobalBans   int    `json:"global_bans"`
	BlockedNets  int    `json:"blocked_nets"`
	JoinAttempts uint64 `json:"join_attempts"`
}

func (a *api) gmAnalytics(w http.ResponseWriter, r *http.Request, _ token) {
	s := a.srv
	s.mu.Lock()
	defer s.mu.Unlock()
	res := gmAnalytics{
		EventsPerHour: map[string]int{},
		Security: securityStats{
			AuthFails:    s.stats.AuthFails,
			GlobalBans:   len(s.store.GlobalBans),
			JoinAttempts: s.stats.JoinAttempts,
		},
	}
	counts := map[string]int{}
	nowH := time.Now().UTC().Truncate(time.Hour)
	for _, line := range s.store.Audit {
		counts[line.Event]++
		if t, err := time.Parse(time.RFC3339, line.TS); err == nil {
			d := nowH.Sub(t.Truncate(time.Hour))
			if d >= 0 && d < 24*time.Hour {
				res.EventsPerHour[t.Truncate(time.Hour).Format("15:04")]++
			}
		}
	}
	for _, n := range s.store.Nets {
		for _, line := range n.Audit {
			counts[line.Event]++
		}
		res.NetworkUsage = append(res.NetworkUsage, netUsage{
			ID: n.ID, Name: n.Name, Online: n.OnlinePeers(), Total: len(n.Peers), Blocked: n.Blocked,
		})
		if n.Blocked {
			res.Security.BlockedNets++
		}
	}
	for ev, c := range counts {
		res.TopEvents = append(res.TopEvents, eventCount{Event: ev, Count: c})
	}
	sort.Slice(res.TopEvents, func(i, j int) bool { return res.TopEvents[i].Count > res.TopEvents[j].Count })
	if len(res.TopEvents) > 12 {
		res.TopEvents = res.TopEvents[:12]
	}
	writeJSON(w, 200, res)
}

// gmAudit returns hub or per-network audit lines.
func (a *api) gmAudit(w http.ResponseWriter, r *http.Request, _ token) {
	netID := r.URL.Query().Get("network")
	a.srv.mu.Lock()
	var lines []AuditLine
	if netID == "" {
		lines = append([]AuditLine(nil), a.srv.store.Audit...)
	} else if n, ok := a.srv.store.Lookup(netID); ok {
		lines = append([]AuditLine(nil), n.Audit...)
	}
	a.srv.mu.Unlock()
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}
	writeJSON(w, 200, map[string]interface{}{"audit": lines})
}

// gmBan adds a global ban and kicks the peer everywhere.
func (a *api) gmBan(w http.ResponseWriter, r *http.Request, _ token) {
	var req struct {
		FP     string `json:"fp"`
		Reason string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	s.store.GlobalBans[req.FP] = &GlobalBan{
		Fingerprint: req.FP, Reason: req.Reason, TS: nowRFC3339(), Actor: "gm",
	}
	kicked := s.kickLocked("", req.FP)
	s.store.AppendAudit("gm", "GLOBAL_BAN", req.FP+" "+req.Reason)
	_ = s.store.Save()
	s.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"ok": "1", "kicked": kicked})
}

// gmUnban lifts a global ban.
func (a *api) gmUnban(w http.ResponseWriter, r *http.Request, _ token) {
	var req struct {
		FP string `json:"fp"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	_, existed := s.store.GlobalBans[req.FP]
	delete(s.store.GlobalBans, req.FP)
	if existed {
		s.store.AppendAudit("gm", "GLOBAL_UNBAN", req.FP)
		_ = s.store.Save()
	}
	s.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"unbanned": existed})
}

// gmKick force-disconnects a peer (optionally only inside one network).
func (a *api) gmKick(w http.ResponseWriter, r *http.Request, _ token) {
	var req struct {
		FP      string `json:"fp"`
		Network string `json:"network,omitempty"`
	}
	if err := readJSON(r, &req); err != nil || req.FP == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	netID := ""
	if req.Network != "" {
		if n, ok := s.store.Lookup(req.Network); ok {
			netID = n.ID
		}
	}
	kicked := s.kickLocked(netID, req.FP)
	s.store.AppendAudit("gm", "GM_KICK", req.FP)
	_ = s.store.Save()
	s.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"ok": "1", "kicked": kicked})
}

// gmBlockNet suspends a whole network and evicts its members.
func (a *api) gmBlockNet(w http.ResponseWriter, r *http.Request, _ token) {
	var req struct {
		Network string `json:"network"`
		Reason  string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil || req.Network == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	n, ok := s.store.Lookup(req.Network)
	if !ok {
		s.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	n.Blocked = true
	n.BlockReason = req.Reason
	n.BlockedAt = nowRFC3339()
	evicted := 0
	for fp := range s.live[n.ID] {
		if s.kickLocked(n.ID, fp) {
			evicted++
		}
	}
	n.AppendAudit("gm", "NETWORK_SUSPENDED", req.Reason)
	s.store.AppendAudit("gm", "NETWORK_SUSPENDED", n.Name+": "+req.Reason)
	_ = s.store.Save()
	s.mu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"ok": "1", "evicted": evicted})
}

// gmUnblockNet reactivates a suspended network.
func (a *api) gmUnblockNet(w http.ResponseWriter, r *http.Request, _ token) {
	var req struct {
		Network string `json:"network"`
	}
	if err := readJSON(r, &req); err != nil || req.Network == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	n, ok := s.store.Lookup(req.Network)
	if !ok {
		s.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	n.Blocked = false
	n.BlockReason = ""
	n.BlockedAt = ""
	n.AppendAudit("gm", "NETWORK_UNSUSPENDED", "")
	s.store.AppendAudit("gm", "NETWORK_UNSUSPENDED", n.Name)
	_ = s.store.Save()
	s.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// gmRotateAdmin issues a new network admin key (GM privilege).
func (a *api) gmRotateAdmin(w http.ResponseWriter, r *http.Request, _ token) {
	var req struct {
		Network string `json:"network"`
	}
	if err := readJSON(r, &req); err != nil || req.Network == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	n, ok := s.store.Lookup(req.Network)
	if !ok {
		s.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	key := crypto.NewAdminKey()
	n.AdminHash = crypto.HashAdminKey(key)
	n.AppendAudit("gm", "ADMIN_KEY_ROTATED", "")
	s.store.AppendAudit("gm", "ADMIN_KEY_ROTATED", n.Name)
	_ = s.store.Save()
	s.mu.Unlock()
	writeJSON(w, 200, map[string]string{"admin_key": key})
}

// gmDeleteNet permanently removes a network (GM privilege).
func (a *api) gmDeleteNet(w http.ResponseWriter, r *http.Request, _ token) {
	var req struct {
		Network string `json:"network"`
	}
	if err := readJSON(r, &req); err != nil || req.Network == "" {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	n, ok := s.store.Lookup(req.Network)
	if !ok {
		s.mu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	for fp := range s.live[n.ID] {
		s.kickLocked(n.ID, fp)
	}
	delete(s.live, n.ID)
	delete(s.store.Nets, n.ID)
	s.store.Reindex()
	s.store.AppendAudit("gm", "NETWORK_DELETED", n.Name+" ("+n.ID+")")
	_ = s.store.Save()
	s.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// gmSearch is a tiny helper used by the static UI hooks (kept for future).
func gmSearch(nets []gmNetSummary, q string) []gmNetSummary {
	q = strings.ToLower(q)
	out := nets[:0]
	for _, n := range nets {
		if strings.Contains(strings.ToLower(n.Name), q) || strings.Contains(n.ID, q) {
			out = append(out, n)
		}
	}
	return out
}
