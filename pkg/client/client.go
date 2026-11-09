// Package client implements the devhive-tunnel device client: hub control
// session, peer directory, end-to-end chat and encrypted file transfer.
package client

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"devhive-tunnel/pkg/conf"
	"devhive-tunnel/pkg/crypto"
	"devhive-tunnel/pkg/hub"
)

// Event kinds delivered to applications (TUI, MCP, CLI).
const (
	EvPeerEvent = "peer_event"
	EvGroupChat = "group_chat"
	EvDM        = "dm"
	EvFileOffer = "file_offer"
	EvFileDone  = "file_done"
	EvError     = "error"
	EvConnected = "connected"
	EvIP        = "ip"
)

// Event is one asynchronous client notification.
type Event struct {
	Type   string          `json:"type"`
	Peer   *hub.PeerRecord `json:"peer,omitempty"`
	From   string          `json:"from,omitempty"` // fingerprint
	Text   string          `json:"text,omitempty"`
	File   *FileOffer      `json:"file,omitempty"`
	Note   string          `json:"note,omitempty"`
	Err    string          `json:"err,omitempty"`
	Packet []byte          `json:"packet,omitempty"` // EvIP raw IPv4 packet
	TS     time.Time       `json:"ts"`
}

// FileOffer describes an incoming (or outgoing) file transfer.
type FileOffer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Chunks  int    `json:"chunks"`
	ChunkSz int    `json:"chunk_size"`
	From    string `json:"from,omitempty"`
	SavedTo string `json:"saved_to,omitempty"`
}

// Config is everything needed to connect one client to one network.
type Config struct {
	Hub         string
	Identity    *crypto.Identity
	Network     *conf.Network
	Nickname    string
	Platform    string
	DownloadDir string
	OnEvent     func(Event) // optional synchronous hook (also delivered on Events)
}

// Client is a live joined network connection.
type Client struct {
	cfg  Config
	sess *crypto.Session
	psk  [32]byte
	grp  *groupSeal

	wmu sync.Mutex

	mu    sync.RWMutex
	info  hub.JoinInfo
	peers map[string]*hub.PeerRecord
	files *fileStore

	inviteCh chan inviteOutcome

	Events chan Event
	closed chan struct{}
	once   sync.Once
}

// inviteOutcome carries a pending invite RPC result.
type inviteOutcome struct {
	resp hub.InviteResp
	err  error
}

// Create opens a hub session and creates a new network.
func Create(cfg Config, req hub.CreateReq) (*Client, hub.CreateResp, error) {
	if req.Nickname == "" {
		req.Nickname = cfg.Nickname
	}
	req.Platform = cfg.Platform
	conn, err := net.DialTimeout("tcp", cfg.Hub, 15*time.Second)
	if err != nil {
		return nil, hub.CreateResp{}, fmt.Errorf("hub unreachable: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(90 * time.Second))
	_ = conn.SetDeadline(time.Time{})
	sess, err := crypto.HandshakeInitiator(conn, cfg.Identity, nil, []byte("devhive-client/v1"))
	if err != nil {
		_ = conn.Close()
		return nil, hub.CreateResp{}, err
	}
	payload, _ := json.Marshal(req)
	if err := sess.Write(byte(hub.WCreate), payload); err != nil {
		_ = conn.Close()
		return nil, hub.CreateResp{}, err
	}
	tp, pb, err := sess.Read()
	if err != nil {
		_ = conn.Close()
		return nil, hub.CreateResp{}, err
	}
	switch hub.WireType(tp) {
	case hub.WCreateOK:
		var resp hub.CreateResp
		if err := json.Unmarshal(pb, &resp); err != nil {
			_ = conn.Close()
			return nil, hub.CreateResp{}, err
		}
		c, err := newClient(cfg, sess, resp.JoinInfo)
		if err != nil {
			_ = conn.Close()
			return nil, hub.CreateResp{}, err
		}
		return c, resp, nil
	case hub.WAuthFail, hub.WSysErr:
		_ = conn.Close()
		return nil, hub.CreateResp{}, decodeErr(pb)
	default:
		_ = conn.Close()
		return nil, hub.CreateResp{}, fmt.Errorf("unexpected reply 0x%02x", tp)
	}
}

// Join opens a hub session and joins an existing network.
func Join(cfg Config) (*Client, error) {
	req := hub.JoinReq{
		Network:  cfg.Network.ID,
		Password: cfg.Network.Password,
		AdminKey: cfg.Network.AdminKey,
		Nickname: cfg.Nickname,
		Platform: cfg.Platform,
	}
	if req.Network == "" {
		req.Network = cfg.Network.Name
	}
	conn, err := net.DialTimeout("tcp", cfg.Hub, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("hub unreachable: %w", err)
	}
	sess, err := crypto.HandshakeInitiator(conn, cfg.Identity, nil, []byte("devhive-client/v1"))
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	payload, _ := json.Marshal(req)
	if err := sess.Write(byte(hub.WJoin), payload); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = sess.RawConn().SetReadDeadline(time.Now().Add(90 * time.Second))
	tp, pb, err := sess.Read()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = sess.RawConn().SetReadDeadline(time.Time{})
	switch hub.WireType(tp) {
	case hub.WJoinOK:
		var resp hub.JoinResp
		if err := json.Unmarshal(pb, &resp); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return newClient(cfg, sess, resp.JoinInfo)
	case hub.WAuthFail, hub.WSysErr:
		_ = conn.Close()
		return nil, decodeErr(pb)
	default:
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected reply 0x%02x", tp)
	}
}

func decodeErr(b []byte) error {
	var se struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(b, &se)
	if se.Message != "" {
		return fmt.Errorf("%s: %s", se.Code, se.Message)
	}
	if se.Code != "" {
		return errors.New(se.Code)
	}
	return errors.New("request rejected")
}

// List queries the public network list on a hub.
func List(hubAddr string, id *crypto.Identity, query string) ([]hub.ListEntry, error) {
	conn, err := net.DialTimeout("tcp", hubAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("hub unreachable: %w", err)
	}
	defer conn.Close()
	sess, err := crypto.HandshakeInitiator(conn, id, nil, []byte("devhive-client/v1"))
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(hub.ListReq{Query: query})
	if err := sess.Write(byte(hub.WList), payload); err != nil {
		return nil, err
	}
	_ = sess.RawConn().SetReadDeadline(time.Now().Add(20 * time.Second))
	tp, pb, err := sess.Read()
	if err != nil {
		return nil, err
	}
	if hub.WireType(tp) != hub.WListOK {
		return nil, decodeErr(pb)
	}
	var resp hub.ListResp
	if err := json.Unmarshal(pb, &resp); err != nil {
		return nil, err
	}
	return resp.Nets, nil
}

func newClient(cfg Config, sess *crypto.Session, info hub.JoinInfo) (*Client, error) {
	psk := crypto.DeriveNetworkPSK(info.NetworkID, cfg.Network.Password)
	grp, err := newGroupSeal(info.NetworkID, psk)
	if err != nil {
		return nil, err
	}
	c := &Client{
		cfg: cfg, sess: sess, psk: psk, grp: grp,
		info: info, peers: map[string]*hub.PeerRecord{},
		files:    newFileStore(cfg.DownloadDir),
		inviteCh: make(chan inviteOutcome, 1),
		Events:   make(chan Event, 256), closed: make(chan struct{}),
	}
	c.absorbPeers(info.Peers)
	logf("connected to %q (%s) as %s", info.Name, info.NetworkID, info.YourFP)
	go c.readLoop()
	go c.keepaliveLoop()
	c.emit(Event{Type: EvConnected, Note: info.NetworkID, TS: time.Now()})
	return c, nil
}

// Info returns the current network state snapshot.
func (c *Client) Info() hub.JoinInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info
}

// Peers returns a snapshot of known peers (excluding self).
func (c *Client) Peers() []*hub.PeerRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*hub.PeerRecord, 0, len(c.peers))
	for fp, p := range c.peers {
		if fp == c.info.YourFP {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out
}

// Peer finds one peer by fingerprint or nickname.
func (c *Client) Peer(idOrNick string) *hub.PeerRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for fp, p := range c.peers {
		if fp == idOrNick || p.Nickname == idOrNick {
			cp := *p
			return &cp
		}
	}
	return nil
}

func (c *Client) absorbPeers(list []*hub.PeerRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range list {
		cp := *p
		c.peers[p.Fingerprint] = &cp
	}
}

// Close leaves the network gracefully.
func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closed)
		err = c.sess.Close()
	})
	return err
}

// readLoop dispatches hub frames to application events.
func (c *Client) readLoop() {
	defer c.emit(Event{Type: EvError, Err: "disconnected from hub", TS: time.Now()})
	for {
		tp, pb, err := c.sess.Read()
		if err != nil {
			return
		}
		switch hub.WireType(tp) {
		case hub.WPeerEvent:
			var ev hub.PeerEvent
			if json.Unmarshal(pb, &ev) == nil && ev.Peer != nil {
				c.mu.Lock()
				cp := *ev.Peer
				c.peers[ev.Peer.Fingerprint] = &cp
				c.mu.Unlock()
				c.emit(Event{Type: EvPeerEvent, Peer: &cp, Note: ev.Event, TS: time.Now()})
			}
		case hub.WRoute:
			var env hub.RouteEnvelope
			if json.Unmarshal(pb, &env) == nil {
				c.handleEnvelope(env)
			}
		case hub.WJoinOK:
			var resp hub.JoinResp
			if json.Unmarshal(pb, &resp) == nil {
				c.mu.Lock()
				c.info = resp.JoinInfo
				c.mu.Unlock()
				c.absorbPeers(resp.JoinInfo.Peers)
			}
		case hub.WInviteOK:
			var resp hub.InviteResp
			if json.Unmarshal(pb, &resp) == nil {
				select {
				case c.inviteCh <- inviteOutcome{resp: resp}:
				default:
				}
			}
		case hub.WSysErr, hub.WAuthFail:
			var se struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if json.Unmarshal(pb, &se) == nil {
				c.emit(Event{Type: EvError, Err: se.Code, Note: se.Message, TS: time.Now()})
			}
		}
	}
}

// keepaliveLoop renews presence every 45s.
func (c *Client) keepaliveLoop() {
	tk := time.NewTicker(45 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-tk.C:
			c.wmu.Lock()
			err := c.sess.Write(byte(hub.WRenew), []byte("{}"))
			c.wmu.Unlock()
			if err != nil {
				return
			}
			_ = c.sess.Ping()
		}
	}
}

// emit delivers an event to the channel and the optional hook.
func (c *Client) emit(ev Event) {
	select {
	case c.Events <- ev:
	default:
	}
	if c.cfg.OnEvent != nil {
		c.cfg.OnEvent(ev)
	}
}

// Invite creates a join invite key (owner/admin only).
func (c *Client) Invite(req hub.InviteReq) (hub.InviteResp, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return hub.InviteResp{}, err
	}
	c.wmu.Lock()
	err = c.sess.Write(byte(hub.WInvite), payload)
	c.wmu.Unlock()
	if err != nil {
		return hub.InviteResp{}, err
	}
	select {
	case out := <-c.inviteCh:
		return out.resp, out.err
	case <-time.After(15 * time.Second):
		return hub.InviteResp{}, errors.New("invite timed out")
	case <-c.closed:
		return hub.InviteResp{}, errors.New("client closed")
	}
}

// sendRoute writes one hub-routed envelope.
func (c *Client) sendRoute(env hub.RouteEnvelope) error {
	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.sess.Write(byte(hub.WRoute), payload)
}

// decodeXPub parses the peer X25519 key.
func decodeXPub(p *hub.PeerRecord) ([32]byte, error) {
	var out [32]byte
	raw, err := base64.RawStdEncoding.DecodeString(p.XPub)
	if err != nil || len(raw) != 32 {
		return out, fmt.Errorf("peer has no valid encryption key")
	}
	copy(out[:], raw)
	return out, nil
}
