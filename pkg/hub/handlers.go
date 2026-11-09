package hub

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"

	"devhive-tunnel/pkg/crypto"
)

func edB64(pk ed25519.PublicKey) string {
	return base64.RawStdEncoding.EncodeToString(pk)
}

func xB64(x [32]byte) string {
	return base64.RawStdEncoding.EncodeToString(x[:])
}

// dropLink detaches a control link and marks its peer offline.
func (s *Server) dropLink(sess *crypto.Session, lk *link) {
	if lk.netID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if mm := s.live[lk.netID]; mm != nil {
		if cur, ok := mm[lk.fp]; ok && cur == lk {
			delete(mm, lk.fp)
		}
		if len(mm) == 0 {
			delete(s.live, lk.netID)
		}
	}
	n, ok := s.store.Nets[lk.netID]
	if !ok {
		return
	}
	p := n.Peers[lk.fp]
	if p == nil || !p.Online {
		return
	}
	p.Online = false
	p.LastSeen = nowRFC3339()
	n.AppendAudit(lk.fp, "DISCONNECT", "")
	_ = s.store.Save()
	s.broadcastEventLocked(n.ID, lk.fp, "peer_offline", p)
}

// sendTo writes one JSON frame to a live link (no s.mu required).
func (s *Server) sendTo(l *link, t WireType, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.reply(l.sess, byte(t), jsonLike(b))
}

// jsonLike avoids re-marshal in reply.
func jsonLike(b []byte) interface{} { return rawJSON(b) }

type rawJSON []byte

func (r rawJSON) MarshalJSON() ([]byte, error) { return []byte(r), nil }

// broadcastEventLocked pushes a peer event to all members except `exceptFP`.
func (s *Server) broadcastEventLocked(netID, exceptFP, event string, p *PeerRecord) {
	ev := PeerEvent{Event: event, Peer: p}
	for fp, l := range s.live[netID] {
		if fp == exceptFP {
			continue
		}
		s.stats.EventsPushed++
		if err := s.sendTo(l, WPeerEvent, ev); err != nil {
			log.Printf("[hub] push to %s failed: %v", fp, err)
		}
	}
}

// handleCreate registers a new network and its creator as admin peer.
func (s *Server) handleCreate(sess *crypto.Session, lk *link, req CreateReq) error {
	name := sanitizeName(req.Name)
	if len([]rune(name)) < 3 {
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "bad_name", Message: "network name must be at least 3 characters"})
	}
	if len(req.Password) < 6 {
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "weak_password", Message: "password must be at least 6 characters"})
	}
	fp := lk.fp
	nick := sanitizeName(req.Nickname)
	if nick == "" {
		nick = "peer-" + fp[:6]
	}

	s.mu.Lock()
	if _, taken := s.store.Names[strings.ToLower(name)]; taken {
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "name_taken", Message: "network name already exists"})
	}
	nid := crypto.NewNetworkID()
	for s.store.Nets[nid] != nil {
		nid = crypto.NewNetworkID()
	}
	settings := NetSettings{AllowRelay: true}
	if req.Settings != nil {
		settings = *req.Settings
	}
	if settings.SubnetCIDR == "" {
		settings.SubnetCIDR = s.opts.Subnet
	}
	adminKey := crypto.NewAdminKey()
	n := &NetRecord{
		ID:        nid,
		Name:      name,
		PSKHash:   crypto.HashPSK(crypto.DeriveNetworkPSK(nid, req.Password)),
		AdminHash: crypto.HashAdminKey(adminKey),
		CreatorFP: fp,
		Created:   nowRFC3339(),
		Settings:  settings,
		Peers:     map[string]*PeerRecord{},
		NextVLAN:  1,
	}
	vip := NextVLANIP(n.Settings.SubnetCIDR, nil, 1)
	n.Peers[fp] = &PeerRecord{
		Fingerprint: fp,
		EdPub:       edB64(sess.RemoteEd()),
		XPub:        xB64(sess.RemoteX()),
		Nickname:    nick,
		Platform:    req.Platform,
		Admin:       true,
		VLANIP:      vip,
		Created:     nowRFC3339(),
		LastSeen:    nowRFC3339(),
		Online:      true,
	}
	n.AppendAudit(fp, "CREATE", "name="+name)
	s.store.Nets[nid] = n
	s.store.Reindex()
	if s.live[nid] == nil {
		s.live[nid] = map[string]*link{}
	}
	s.live[nid][fp] = lk
	lk.netID = nid
	err := s.store.Save()
	if err != nil {
		delete(s.store.Nets, nid)
		s.store.Reindex()
		delete(s.live, nid)
		s.mu.Unlock()
		return s.reply(sess, byte(WSysErr), sysErr{Code: "save_failed"})
	}
	info := JoinInfo{
		NetworkID: n.ID, Name: n.Name, Settings: n.Settings,
		Peers: n.PeerMapSnapshot(), IsAdmin: true,
		YourFP: fp, YourVLANIP: vip,
	}
	s.mu.Unlock()

	log.Printf("[hub] network %q created (%s) by %s", name, nid, fp)
	return s.reply(sess, byte(WCreateOK), CreateResp{JoinInfo: info, AdminKey: adminKey})
}

// handleJoin authenticates and registers a member.
func (s *Server) handleJoin(sess *crypto.Session, lk *link, req JoinReq) error {
	host := "?"
	if addr := sess.RawConn().RemoteAddr(); addr != nil {
		if h, _, err := splitHostPort(addr.String()); err == nil {
			host = h
		}
	}
	if !s.joinRateOK(host) {
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "rate_limited", Message: "too many attempts, try again later"})
	}
	fp := lk.fp
	s.mu.Lock()
	s.stats.JoinAttempts++

	if gb, banned := s.store.GlobalBans[fp]; banned {
		s.stats.AuthFails++
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "gm_banned", Message: gb.Reason})
	}
	n, ok := s.store.Lookup(req.Network)
	if !ok {
		s.stats.AuthFails++
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "not_found", Message: "network name or id not found"})
	}
	if n.Blocked {
		s.stats.AuthFails++
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "network_suspended", Message: n.BlockReason})
	}
	if !s.joinNetRateOKLocked(n, host) {
		s.stats.AuthFails++
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "rate_limited", Message: "too many join attempts for this network, try again later"})
	}
	if p := n.Peers[fp]; p != nil && p.Banned {
		s.stats.AuthFails++
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "banned", Message: p.BanReason})
	}
	if !crypto.PSKMatches(n.ID, req.Password, n.PSKHash) {
		s.stats.AuthFails++
		n.AppendAudit(fp, "JOIN_FAIL", "bad password")
		_ = s.store.Save()
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "bad_password", Message: "wrong network name, id or password"})
	}
	isAdmin := req.AdminKey != "" && crypto.CheckAdminKey(req.AdminKey, n.AdminHash)
	if n.Settings.RequireInviteKey && !isAdmin {
		// Already-enrolled devices (this fingerprint joined before) may
		// reconnect with just the password; invites gate new devices only.
		if n.Peers[fp] == nil {
			match, inv := matchInvite(n, req.AdminKey)
			if !match {
				s.stats.AuthFails++
				n.AppendAudit(fp, "JOIN_FAIL", "invite key rejected")
				_ = s.store.Save()
				s.mu.Unlock()
				return s.reply(sess, byte(WAuthFail), sysErr{Code: "invite_required", Message: "this network requires an admin access key"})
			}
			if inv != nil {
				inv.Uses++
				if inv.Role == "admin" {
					isAdmin = true
				}
			}
		}
	}
	if n.Settings.MaxPeers > 0 && n.OnlinePeers() >= n.Settings.MaxPeers {
		s.mu.Unlock()
		return s.reply(sess, byte(WAuthFail), sysErr{Code: "network_full", Message: "network is full"})
	}
	nick := sanitizeName(req.Nickname)
	if nick == "" {
		nick = "peer-" + fp[:6]
	}
	p := n.Peers[fp]
	if p == nil {
		p = &PeerRecord{Fingerprint: fp, Created: nowRFC3339()}
		n.Peers[fp] = p
	}
	if isAdmin {
		p.Admin = true
	}
	p.EdPub = edB64(sess.RemoteEd())
	p.XPub = xB64(sess.RemoteX())
	p.Nickname = nick
	if req.Platform != "" {
		p.Platform = req.Platform
	}
	p.Online = true
	p.LastSeen = nowRFC3339()
	if p.VLANIP == "" && n.Settings.SubnetCIDR != "" {
		taken := map[string]bool{}
		for _, other := range n.Peers {
			if other.VLANIP != "" && other.Fingerprint != fp {
				taken[other.VLANIP] = true
			}
		}
		p.VLANIP = NextVLANIP(n.Settings.SubnetCIDR, taken, n.NextVLAN)
		n.NextVLAN++
	}
	n.AppendAudit(fp, "JOIN", "nickname="+p.Nickname)
	if s.live[n.ID] == nil {
		s.live[n.ID] = map[string]*link{}
	}
	s.live[n.ID][fp] = lk
	lk.netID = n.ID
	online := 0
	for _, q := range n.Peers {
		if q.Online {
			online++
		}
	}
	if online > s.stats.PeakOnline {
		s.stats.PeakOnline = online
	}
	info := JoinInfo{
		NetworkID: n.ID, Name: n.Name, Settings: n.Settings,
		Peers: n.PeerMapSnapshot(), IsAdmin: p.Admin,
		YourFP: fp, YourVLANIP: p.VLANIP,
	}
	_ = s.store.Save()
	s.broadcastEventLocked(n.ID, fp, "peer_online", clonePeer(p))
	s.mu.Unlock()

	log.Printf("[hub] %s joined %q (%s) admin=%v", nick, n.Name, n.ID, p.Admin)
	return s.reply(sess, byte(WJoinOK), JoinResp{JoinInfo: info})
}

func clonePeer(p *PeerRecord) *PeerRecord { cp := *p; return &cp }

func matchInvite(n *NetRecord, key string) (bool, *Invite) {
	if key == "" {
		return false, nil
	}
	h := crypto.HashAdminKey(key)
	for i := range n.Invites {
		inv := &n.Invites[i]
		if inv.KeyHash == h && inv.Active() {
			return true, inv
		}
	}
	return false, nil
}
