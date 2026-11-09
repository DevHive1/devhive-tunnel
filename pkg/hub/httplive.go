package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"devhive-tunnel/pkg/conf"
)

// gmLivePeer is one live peer row for the GM console.
type gmLivePeer struct {
	NetworkID string `json:"network_id"`
	Network   string `json:"network"`
	FP        string `json:"fp"`
	Nickname  string `json:"nickname"`
	Platform  string `json:"platform"`
	VLANIP    string `json:"vlan_ip"`
	Online    bool   `json:"online"`
	Admin     bool   `json:"admin"`
	LastSeen  string `json:"last_seen"`
}

// gmLiveAudit augments an audit line with its network name.
type gmLiveAudit struct {
	Network string `json:"network"`
	AuditLine
}

// gmLiveSnapshot is pushed to GM dashboards once per second.
type gmLiveSnapshot struct {
	TS       int64          `json:"ts"`
	Overview gmOverview     `json:"overview"`
	Networks []gmNetSummary `json:"networks"`
	Peers    []gmLivePeer   `json:"peers"`
	Audit    []gmLiveAudit  `json:"audit"`
}

// overviewLocked builds the GM overview; caller must hold s.mu.
func (s *Server) overviewLocked() gmOverview {
	ov := gmOverview{
		Version: conf.Version, Started: s.started.UTC().Format(time.RFC3339),
		Stats: s.stats, Networks: len(s.store.Nets),
		GlobalBans: len(s.store.GlobalBans), HubFingerprint: s.id.Fingerprint(),
		Subnet: s.opts.Subnet,
	}
	for _, n := range s.store.Nets {
		ov.PeersTotal += len(n.Peers)
		ov.PeersOnline += n.OnlinePeers()
		if n.Blocked {
			ov.BlockedNets++
		}
	}
	ov.UptimeSec = int64(time.Since(s.started).Seconds())
	return ov
}

// summariesLocked builds the network list; caller must hold s.mu.
func (s *Server) summariesLocked() []gmNetSummary {
	out := make([]gmNetSummary, 0, len(s.store.Nets))
	for _, n := range s.store.Nets {
		adm := 0
		for _, p := range n.Peers {
			if p.Admin {
				adm++
			}
		}
		out = append(out, gmNetSummary{
			ID: n.ID, Name: n.Name, Created: n.Created, Blocked: n.Blocked,
			BlockReason: n.BlockReason, Total: len(n.Peers), Online: n.OnlinePeers(),
			Admins: adm, Settings: n.Settings, CreatorFP: n.CreatorFP,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// liveSnapshot copies the whole hub state for one SSE frame.
func (s *Server) liveSnapshot() gmLiveSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := gmLiveSnapshot{
		TS: time.Now().Unix(), Overview: s.overviewLocked(),
		Networks: s.summariesLocked(), Peers: make([]gmLivePeer, 0, 32),
	}
	const maxPeers = 1000
	for _, n := range s.store.Nets {
		for _, p := range n.Peers {
			if len(snap.Peers) >= maxPeers {
				break
			}
			snap.Peers = append(snap.Peers, gmLivePeer{
				NetworkID: n.ID, Network: n.Name, FP: p.Fingerprint,
				Nickname: p.Nickname, Platform: p.Platform, VLANIP: p.VLANIP,
				Online: p.Online, Admin: p.Admin, LastSeen: p.LastSeen,
			})
		}
	}
	sort.Slice(snap.Peers, func(i, j int) bool {
		if snap.Peers[i].Network != snap.Peers[j].Network {
			return snap.Peers[i].Network < snap.Peers[j].Network
		}
		return snap.Peers[i].Nickname < snap.Peers[j].Nickname
	})

	// Merge the newest audit lines from the hub and every network.
	const hubTail, netTail, maxAudit = 120, 40, 100
	all := make([]gmLiveAudit, 0, hubTail+netTail*len(s.store.Nets))
	hub := s.store.Audit
	if len(hub) > hubTail {
		hub = hub[len(hub)-hubTail:]
	}
	for _, l := range hub {
		all = append(all, gmLiveAudit{Network: "hub", AuditLine: l})
	}
	for _, n := range s.store.Nets {
		al := n.Audit
		if len(al) > netTail {
			al = al[len(al)-netTail:]
		}
		for _, l := range al {
			all = append(all, gmLiveAudit{Network: n.Name, AuditLine: l})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].TS > all[j].TS })
	if len(all) > maxAudit {
		all = all[:maxAudit]
	}
	snap.Audit = all
	return snap
}

// gmLive streams live hub metadata to the GM console via SSE.
// The token is passed as a query parameter because EventSource cannot set
// an Authorization header; run the dashboard over HTTPS in production.
func (a *api) gmLive(w http.ResponseWriter, r *http.Request) {
	tk, ok := a.srv.checkToken(r.URL.Query().Get("token"))
	if !ok || tk.kind != "gm" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	h.Set("X-Content-Type-Options", "nosniff")
	fmt.Fprint(w, "retry: 3000\n\n")
	fl.Flush()

	send := func() bool {
		b, err := json.Marshal(a.srv.liveSnapshot())
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return false
		}
		fl.Flush()
		return true
	}
	if !send() {
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if !send() {
				return
			}
		}
	}
}
