package hub

import (
	"encoding/json"
	"log"
	"net"
	"sort"
	"strings"
	"time"

	"devhive-tunnel/pkg/crypto"
)

func splitHostPort(s string) (string, string, error) { return net.SplitHostPort(s) }

func nowMinute() int64 { return time.Now().Unix() / 60 }

// joinRateOK limits attempts per source host per minute.
func (s *Server) joinRateOK(host string) bool {
	k := joinKey{host: host, minute: nowMinute()}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.joins[k]++
	return s.joins[k] <= 12
}

// joinNetRateOKLocked enforces the per-network join limit (settings).
// Caller must hold s.mu.
func (s *Server) joinNetRateOKLocked(n *NetRecord, host string) bool {
	if n.Settings.RateJoinPerMin <= 0 {
		return true
	}
	k := joinNetKey{netID: n.ID, host: host, minute: nowMinute()}
	s.joinsNet[k]++
	return s.joinsNet[k] <= n.Settings.RateJoinPerMin
}

// sweepJoins prunes stale rate-limit buckets.
func (s *Server) sweepJoins() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := nowMinute() - 2
	for k := range s.joins {
		if k.minute < cut {
			delete(s.joins, k)
		}
	}
	for k := range s.joinsNet {
		if k.minute < cut {
			delete(s.joinsNet, k)
		}
	}
}

// handleList returns publicly discoverable networks.
func (s *Server) handleList(sess *crypto.Session, req ListReq) error {
	q := strings.ToLower(strings.TrimSpace(req.Query))
	s.mu.Lock()
	out := make([]ListEntry, 0, 16)
	for _, n := range s.store.Nets {
		if !n.Settings.AllowAnonymous || n.Blocked {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(n.Name), q) {
			continue
		}
		out = append(out, ListEntry{ID: n.ID, Name: n.Name, Online: n.OnlinePeers()})
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > 50 {
		out = out[:50]
	}
	return s.reply(sess, byte(WListOK), ListResp{Nets: out})
}

// handleInvite creates a new invite key (network admins only).
func (s *Server) handleInvite(sess *crypto.Session, lk *link, req InviteReq) error {
	s.mu.Lock()
	n, ok := s.store.Nets[lk.netID]
	if !ok {
		s.mu.Unlock()
		return s.reply(sess, byte(WSysErr), sysErr{Code: "no_network"})
	}
	p := n.Peers[lk.fp]
	if p == nil || !p.Admin {
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "not_admin", Message: "admin rights required"})
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
		Note: sanitizeName(req.Note), UsesMax: req.UsesMax, Expires: exp, Role: role,
	})
	n.AppendAudit(lk.fp, "INVITE_CREATED", "role="+role)
	_ = s.store.Save()
	s.mu.Unlock()
	log.Printf("[hub] invite created on %q by %s", n.Name, lk.fp)
	return s.reply(sess, byte(WInviteOK), InviteResp{Code: secret, Role: role, Note: req.Note})
}

// handleRoute relays an opaque envelope to one member or the whole network.
func (s *Server) handleRoute(sess *crypto.Session, lk *link, env RouteEnvelope) error {
	s.mu.Lock()
	n, ok := s.store.Nets[lk.netID]
	if !ok {
		s.mu.Unlock()
		return s.reply(sess, byte(WSysErr), sysErr{Code: "no_network"})
	}
	if n.Blocked {
		s.mu.Unlock()
		return s.reply(sess, byte(WSysErr), sysErr{Code: "network_suspended"})
	}
	env.FromFP = lk.fp
	if env.ToFP == "" {
		for fp, l := range s.live[n.ID] {
			if fp == lk.fp {
				continue
			}
			s.stats.RoutedFrames++
			if err := s.sendTo(l, WRoute, env); err != nil {
				log.Printf("[hub] broadcast route to %s failed: %v", fp, err)
			}
		}
		s.mu.Unlock()
		return nil
	}
	target := s.live[n.ID][env.ToFP]
	if target == nil {
		s.mu.Unlock()
		return s.reply(sess, byte(WSysErr), sysErr{Code: "peer_offline", Message: "peer is not connected"})
	}
	s.stats.RoutedFrames++
	err := s.sendTo(target, WRoute, env)
	s.mu.Unlock()
	if err != nil {
		return s.reply(sess, byte(WSysErr), sysErr{Code: "route_failed"})
	}
	return nil
}

// handleRenew refreshes presence and returns a fresh directory.
func (s *Server) handleRenew(sess *crypto.Session, lk *link) error {
	s.mu.Lock()
	n, ok := s.store.Nets[lk.netID]
	if !ok {
		s.mu.Unlock()
		return s.reply(sess, byte(WSysErr), sysErr{Code: "no_network"})
	}
	p := n.Peers[lk.fp]
	if p != nil {
		p.Online = true
		p.LastSeen = nowRFC3339()
	}
	info := JoinInfo{
		NetworkID: n.ID, Name: n.Name, Settings: n.Settings,
		Peers: n.PeerMapSnapshot(), IsAdmin: p != nil && p.Admin,
		YourFP: lk.fp,
	}
	if p != nil {
		info.YourVLANIP = p.VLANIP
	}
	_ = s.store.Save()
	s.mu.Unlock()
	return s.reply(sess, byte(WJoinOK), JoinResp{JoinInfo: info})
}

// marshal helper for hub writes.
func marshal(v interface{}) ([]byte, error) { return json.Marshal(v) }
