// Package mcp implements a Model Context Protocol server (stdio JSON-RPC
// 2.0) exposing devhive-tunnel networks, chat, files and invites to AI agents.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"devhive-tunnel/pkg/aigen"
	"devhive-tunnel/pkg/client"
	"devhive-tunnel/pkg/conf"
	"devhive-tunnel/pkg/crypto"
	"devhive-tunnel/pkg/hub"
)

// Server speaks MCP over an io.ReadWriteCloser (usually stdin/stdout).
type Server struct {
	in      *bufio.Scanner
	out     io.Writer
	version string
	wmu     sync.Mutex
}

// New builds a server.
func New(in io.Reader, out io.Writer, version string) *Server {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	return &Server{in: sc, out: out, version: version}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Run processes requests until EOF.
func (s *Server) Run() error {
	for s.in.Scan() {
		line := strings.TrimSpace(s.in.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.write(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		s.handle(req)
	}
	return s.in.Err()
}

func (s *Server) write(resp rpcResponse) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	resp.JSONRPC = "2.0"
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	fmt.Fprintln(s.out, string(b))
}

func (s *Server) handle(req rpcRequest) {
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"
	reply := func(result any) {
		if !isNotification {
			s.write(rpcResponse{ID: req.ID, Result: result})
		}
	}
	fail := func(code int, msg string) {
		if !isNotification {
			s.write(rpcResponse{ID: req.ID, Error: &rpcError{Code: code, Message: msg}})
		}
	}
	switch req.Method {
	case "initialize":
		reply(map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "devhive-tunnel", "version": s.version},
			"instructions": "devhive-tunnel gives you encrypted private networks: create/join a hive, " +
				"chat, send files, list peers and mint invites. Start with dh_ai_plan or dh_network_create, " +
				"then dh_network_join, then dh_chat_send / dh_chat_read / dh_file_send.",
		})
	case "notifications/initialized", "notifications/cancelled":
		// no response for notifications
	case "ping":
		reply(map[string]any{})
	case "tools/list":
		reply(map[string]any{"tools": toolsList()})
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(-32602, "invalid params")
			return
		}
		text, err := s.callTool(p.Name, p.Arguments)
		if err != nil {
			reply(map[string]any{
				"content": []map[string]any{{"type": "text", "text": "error: " + err.Error()}},
				"isError": true,
			})
			return
		}
		reply(map[string]any{"content": []map[string]any{{"type": "text", "text": text}}})
	default:
		fail(-32601, "method not found: "+req.Method)
	}
}

func (s *Server) callTool(name string, raw json.RawMessage) (string, error) {
	switch name {
	case "dh_status":
		return toolStatus()
	case "dh_networks_list":
		return toolNetworks()
	case "dh_ai_plan":
		return toolAIPlan(raw)
	case "dh_network_create":
		return toolCreate(raw)
	case "dh_network_join":
		return toolJoin(raw)
	case "dh_peers":
		return toolPeers(raw)
	case "dh_chat_send":
		return toolChatSend(raw)
	case "dh_chat_read":
		return toolChatRead(raw)
	case "dh_dm_send":
		return toolDMSend(raw)
	case "dh_file_send":
		return toolFileSend(raw)
	case "dh_invite_create":
		return toolInvite(raw)
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func toolsList() []map[string]any {
	str := map[string]any{"type": "string"}
	integer := map[string]any{"type": "integer"}
	boolean := map[string]any{"type": "boolean"}
	tool := func(name, desc string, props map[string]any, required ...string) map[string]any {
		schema := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		return map[string]any{"name": name, "description": desc, "inputSchema": schema}
	}
	return []map[string]any{
		tool("dh_status", "Show this device identity, hub address and saved networks.", nil),
		tool("dh_networks_list", "List saved networks with role and hub.", nil),
		tool("dh_ai_plan", "Generate (without creating) a hardened network plan from a few answers. Profiles: private, team, community, paranoid.",
			map[string]any{"name": str, "profile": str, "members": integer, "lan_games": boolean}, "name"),
		tool("dh_network_create", "Create an encrypted hive network and become its owner. Returns network id, admin key and password (show them once).",
			map[string]any{"name": str, "password": str, "profile": str, "members": integer, "lan_games": boolean}, "name"),
		tool("dh_network_join", "Join a network and save it on this device.",
			map[string]any{"network": str, "password": str, "key": str}, "network", "password"),
		tool("dh_peers", "List peers of a saved network (online state, virtual LAN IP, admin).",
			map[string]any{"network": str}, "network"),
		tool("dh_chat_send", "Send an end-to-end encrypted message to the whole network.",
			map[string]any{"network": str, "text": str}, "network", "text"),
		tool("dh_chat_read", "Connect for a while and collect incoming messages, DMs and file offers.",
			map[string]any{"network": str, "seconds": integer}, "network"),
		tool("dh_dm_send", "Send a private encrypted direct message to one peer.",
			map[string]any{"network": str, "to": str, "text": str}, "network", "to", "text"),
		tool("dh_file_send", "Send a file (encrypted chunks) to a peer on the network.",
			map[string]any{"network": str, "to": str, "path": str}, "network", "to", "path"),
		tool("dh_invite_create", "Mint a join invite key (owner/admin only).",
			map[string]any{"network": str, "uses": integer, "days": integer, "note": str}, "network"),
	}
}

// ---------------- helpers ----------------

func load() (*conf.Client, *crypto.Identity, error) {
	cfg, err := conf.Load("")
	if err != nil {
		return nil, nil, err
	}
	seed, err := cfg.LoadSeedHex()
	if err != nil {
		return nil, nil, err
	}
	if seed == "" {
		id, err := crypto.NewIdentity()
		if err != nil {
			return nil, nil, err
		}
		if err := cfg.StoreSeedHex(id.SaveSeedHexText()); err != nil {
			return nil, nil, err
		}
		return cfg, id, nil
	}
	id, err := crypto.LoadSeedHexText(seed)
	if err != nil {
		return nil, nil, err
	}
	return cfg, id, nil
}

func connect(cfg *conf.Client, id *crypto.Identity, network string) (*client.Client, error) {
	net := cfg.FindNetwork(network)
	if net == nil {
		return nil, fmt.Errorf("network %q is not saved — join it first", network)
	}
	nick := cfg.Nickname
	if nick == "" {
		nick = "agent-" + id.Fingerprint()[:6]
	}
	cc := client.Config{
		Hub: cfg.Hub, Identity: id, Network: net, Nickname: nick,
		Platform: "mcp", DownloadDir: cfg.DownloadsDir(),
	}
	return client.Join(cc)
}

func pretty(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return string(b), err
}

// ---------------- tools ----------------

func toolStatus() (string, error) {
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	return pretty(map[string]any{
		"version": conf.Version, "identity": id.Fingerprint(),
		"hub": cfg.Hub, "config_dir": conf.DefaultDir(),
		"networks": len(cfg.Networks),
	})
}

func toolNetworks() (string, error) {
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	out := []map[string]any{}
	for _, n := range cfg.Networks {
		out = append(out, map[string]any{
			"id": n.ID, "name": n.Name, "hub": hubOrDefault(n.Hub, cfg.Hub),
			"owner": n.Created || n.AdminKey != "",
		})
	}
	_ = id
	return pretty(out)
}

func hubOrDefault(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func toolAIPlan(raw json.RawMessage) (string, error) {
	var in struct {
		Name     string `json:"name"`
		Profile  string `json:"profile"`
		Members  int    `json:"members"`
		LANGames bool   `json:"lan_games"`
	}
	_ = json.Unmarshal(raw, &in)
	cfg, _, err := load()
	if err != nil {
		return "", err
	}
	plan := aigen.Generate(aigen.Input{
		Name: in.Name, Profile: aigen.Profile(in.Profile),
		Members: in.Members, LANGames: in.LANGames, FileSharing: true,
		Language: cfg.Language,
	})
	return pretty(plan)
}

func toolCreate(raw json.RawMessage) (string, error) {
	var in struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Profile  string `json:"profile"`
		Members  int    `json:"members"`
		LANGames bool   `json:"lan_games"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	if in.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	plan := aigen.Generate(aigen.Input{
		Name: in.Name, Profile: aigen.Profile(in.Profile), Members: in.Members,
		LANGames: in.LANGames, FileSharing: true, Language: cfg.Language,
	})
	if in.Password != "" {
		plan.Password = in.Password
	}
	nick := cfg.Nickname
	if nick == "" {
		nick = "owner-" + id.Fingerprint()[:6]
	}
	cc := client.Config{
		Hub: cfg.Hub, Identity: id, Nickname: nick,
		Platform: "mcp", DownloadDir: cfg.DownloadsDir(),
		Network: &conf.Network{Name: plan.Name, Password: plan.Password},
	}
	_, resp, err := client.Create(cc, hub.CreateReq{
		Name: plan.Name, Password: plan.Password, Nickname: nick, Settings: &plan.Settings,
	})
	if err != nil {
		return "", err
	}
	cfg.AddNetwork(&conf.Network{
		ID: resp.NetworkID, Name: resp.Name, Password: plan.Password,
		AdminKey: resp.AdminKey, Created: true,
	})
	if cfg.Nickname == "" {
		cfg.Nickname = nick
	}
	if err := cfg.Save(); err != nil {
		return "", err
	}
	return pretty(map[string]any{
		"network_id": resp.NetworkID, "name": resp.Name,
		"admin_key": resp.AdminKey, "password": plan.Password,
		"subnet": plan.SubnetCIDR, "security_score": plan.SecurityScore,
		"warning": "Save the admin key and password now; the key is never shown again.",
	})
}

func toolJoin(raw json.RawMessage) (string, error) {
	var in struct {
		Network  string `json:"network"`
		Password string `json:"password"`
		Key      string `json:"key"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	nick := cfg.Nickname
	if nick == "" {
		nick = "agent-" + id.Fingerprint()[:6]
	}
	cc := client.Config{
		Hub: cfg.Hub, Identity: id,
		Network:  &conf.Network{ID: in.Network, Name: in.Network, Password: in.Password, AdminKey: in.Key},
		Nickname: nick, Platform: "mcp", DownloadDir: cfg.DownloadsDir(),
	}
	c, err := client.Join(cc)
	if err != nil {
		return "", err
	}
	defer c.Close()
	info := c.Info()
	cfg.AddNetwork(&conf.Network{
		ID: info.NetworkID, Name: info.Name, Password: in.Password,
		AdminKey: in.Key, Hub: cfg.Hub,
	})
	if err := cfg.Save(); err != nil {
		return "", err
	}
	return pretty(info)
}

func toolPeers(raw json.RawMessage) (string, error) {
	var in struct {
		Network string `json:"network"`
	}
	_ = json.Unmarshal(raw, &in)
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	c, err := connect(cfg, id, in.Network)
	if err != nil {
		return "", err
	}
	defer c.Close()
	return pretty(c.Peers())
}

func toolChatSend(raw json.RawMessage) (string, error) {
	var in struct {
		Network string `json:"network"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	c, err := connect(cfg, id, in.Network)
	if err != nil {
		return "", err
	}
	defer c.Close()
	if err := c.SendGroupChat(in.Text); err != nil {
		return "", err
	}
	return `{"sent":true}`, nil
}

func toolChatRead(raw json.RawMessage) (string, error) {
	var in struct {
		Network string `json:"network"`
		Seconds int    `json:"seconds"`
	}
	_ = json.Unmarshal(raw, &in)
	if in.Seconds <= 0 {
		in.Seconds = 15
	}
	if in.Seconds > 120 {
		in.Seconds = 120
	}
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	c, err := connect(cfg, id, in.Network)
	if err != nil {
		return "", err
	}
	defer c.Close()
	msgs := []map[string]any{}
	deadline := time.After(time.Duration(in.Seconds) * time.Second)
	for {
		select {
		case <-deadline:
			return pretty(msgs)
		case ev := <-c.Events:
			switch ev.Type {
			case client.EvGroupChat, client.EvDM:
				msgs = append(msgs, map[string]any{
					"type": ev.Type, "from": ev.From, "text": ev.Text,
					"at": ev.TS.Format(time.RFC3339),
				})
			case client.EvFileOffer:
				msgs = append(msgs, map[string]any{
					"type": "file_offer", "from": ev.From, "note": ev.Note,
					"file": ev.File, "at": ev.TS.Format(time.RFC3339),
				})
			case client.EvFileDone:
				msgs = append(msgs, map[string]any{
					"type": "file_done", "from": ev.From, "path": ev.Note,
					"at": ev.TS.Format(time.RFC3339),
				})
			}
		}
	}
}

func toolDMSend(raw json.RawMessage) (string, error) {
	var in struct {
		Network string `json:"network"`
		To      string `json:"to"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	c, err := connect(cfg, id, in.Network)
	if err != nil {
		return "", err
	}
	defer c.Close()
	if err := c.SendDM(in.To, in.Text); err != nil {
		return "", err
	}
	return `{"sent":true}`, nil
}

func toolFileSend(raw json.RawMessage) (string, error) {
	var in struct {
		Network string `json:"network"`
		To      string `json:"to"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	c, err := connect(cfg, id, in.Network)
	if err != nil {
		return "", err
	}
	defer c.Close()
	offer, err := c.SendFile(in.To, in.Path)
	if err != nil {
		return "", err
	}
	return pretty(offer)
}

func toolInvite(raw json.RawMessage) (string, error) {
	var in struct {
		Network string `json:"network"`
		Uses    int    `json:"uses"`
		Days    int    `json:"days"`
		Note    string `json:"note"`
	}
	_ = json.Unmarshal(raw, &in)
	if in.Uses == 0 {
		in.Uses = 1
	}
	if in.Days == 0 {
		in.Days = 7
	}
	cfg, id, err := load()
	if err != nil {
		return "", err
	}
	c, err := connect(cfg, id, in.Network)
	if err != nil {
		return "", err
	}
	defer c.Close()
	resp, err := c.Invite(hub.InviteReq{Note: in.Note, UsesMax: in.Uses, ExpiresDays: in.Days, Role: "member"})
	if err != nil {
		return "", err
	}
	return pretty(resp)
}

// Stdin convenience for cmd/dh-mcp.
var _ = os.Stdin
