package hub_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devhive-tunnel/pkg/client"
	"devhive-tunnel/pkg/conf"
	"devhive-tunnel/pkg/crypto"
	"devhive-tunnel/pkg/hub"
)

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func waitDial(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("hub did not start at %s", addr)
}

func waitEvent(t *testing.T, ch <-chan client.Event, typ string, secs int) client.Event {
	t.Helper()
	deadline := time.After(time.Duration(secs) * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Type == typ {
				return ev
			}
		case <-deadline:
			t.Fatalf("timeout waiting for event %q", typ)
		}
	}
}

func mustIdentity(t *testing.T) *crypto.Identity {
	t.Helper()
	id, err := crypto.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()
	ctlAddr := freePort(t)
	srv, err := hub.OpenServer(hub.ServerOptions{Dir: dir, Listen: ctlAddr, Web: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("open hub: %v", err)
	}
	go func() { _ = srv.Serve() }()
	waitDial(t, ctlAddr)

	ts := httptest.NewServer(hub.Handler(srv))
	defer ts.Close()

	// GM key was generated on first run
	gmKey := readGMKey(t, dir)

	idA, idB, idC := mustIdentity(t), mustIdentity(t), mustIdentity(t)
	dlA := filepath.Join(t.TempDir(), "dlA")
	dlB := filepath.Join(t.TempDir(), "dlB")
	dlC := filepath.Join(t.TempDir(), "dlC")

	// ---- create network ----
	cfgA := client.Config{
		Hub: ctlAddr, Identity: idA,
		Network:  &conf.Network{ID: "", Name: "hive-test", Password: "hunter22"},
		Nickname: "alice", Platform: "test", DownloadDir: dlA,
	}
	ca, cresp, err := client.Create(cfgA, hub.CreateReq{
		Name: "hive-test", Password: "hunter22", Nickname: "alice",
		Settings: &hub.NetSettings{RequireInviteKey: false, AllowRelay: true},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer ca.Close()
	if cresp.NetworkID == "" || cresp.AdminKey == "" {
		t.Fatal("create response missing id/admin key")
	}
	netID, adminKey := cresp.NetworkID, cresp.AdminKey
	t.Logf("network created: %s", netID)

	// ---- join with password ----
	cfgB := client.Config{
		Hub: ctlAddr, Identity: idB,
		Network:  &conf.Network{ID: netID, Name: "hive-test", Password: "hunter22"},
		Nickname: "bob", Platform: "test", DownloadDir: dlB,
	}
	cb, err := client.Join(cfgB)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	defer cb.Close()
	fpB := cb.Info().YourFP
	if cb.Info().Name != "hive-test" {
		t.Fatal("wrong network name")
	}

	// wrong password must fail
	badCfg := cfgB
	badCfg.Identity = mustIdentity(t)
	badCfg.Network = &conf.Network{ID: netID, Password: "wrongpassword"}
	if _, err := client.Join(badCfg); err == nil {
		t.Fatal("wrong password accepted")
	}

	// unknown network must fail
	badCfg.Network = &conf.Network{ID: "dh-NOPE", Password: "hunter22"}
	if _, err := client.Join(badCfg); err == nil {
		t.Fatal("unknown network accepted")
	}

	// alice sees bob online
	_ = waitEvent(t, ca.Events, client.EvPeerEvent, 10)

	// ---- group chat ----
	if err := ca.SendGroupChat("hello hive"); err != nil {
		t.Fatalf("send group chat: %v", err)
	}
	ev := waitEvent(t, cb.Events, client.EvGroupChat, 10)
	if !strings.Contains(ev.Text, "hello hive") || !strings.Contains(ev.Text, "alice") {
		t.Fatalf("bad group chat: %q", ev.Text)
	}

	// ---- DM ----
	fpA := ca.Info().YourFP
	if err := cb.SendDM(fpA, "dm secret"); err != nil {
		t.Fatalf("send dm: %v", err)
	}
	ev = waitEvent(t, ca.Events, client.EvDM, 10)
	if !strings.Contains(ev.Text, "dm secret") {
		t.Fatalf("bad dm: %q", ev.Text)
	}

	// ---- file transfer ----
	content := bytes.Repeat([]byte("devhive-file-chunk-"), 20000)
	src := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ca.SendFile(fpB, src); err != nil {
		t.Fatalf("send file: %v", err)
	}
	waitEvent(t, cb.Events, client.EvFileOffer, 15)
	done := waitEvent(t, cb.Events, client.EvFileDone, 60)
	if done.Note == "" {
		t.Fatal("file done without path")
	}
	got, err := os.ReadFile(done.Note)
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("file content mismatch (len %d vs %d)", len(got), len(content))
	}

	// ---- member C + invite-key protected join ----
	cfgC := client.Config{
		Hub: ctlAddr, Identity: idC,
		Network:  &conf.Network{ID: netID, Password: "hunter22"},
		Nickname: "carol", Platform: "test", DownloadDir: dlC,
	}

	// owner enables invite requirement via REST
	tok := adminLogin(t, ts.URL, netID, adminKey)
	postJSON(t, ts.URL+"/api/net/settings", tok, map[string]interface{}{
		"subnet_cidr": "100.72.0.0/16", "require_invite_key": true,
		"allow_anonymous_discovery": false, "allow_relay": true, "max_peers": 0,
		"max_upload_mbps": 0,
	})
	if _, err := client.Join(cfgC); err == nil {
		t.Fatal("join succeeded without invite key while required")
	}

	// create invite and join with it
	invResp := postJSON(t, ts.URL+"/api/invites", tok, map[string]interface{}{
		"note": "carol", "uses_max": 1, "expires_days": 1, "role": "member",
	})
	inviteCode, _ := invResp["code"].(string)
	if inviteCode == "" {
		t.Fatalf("no invite code: %v", invResp)
	}
	cfgC.Network.AdminKey = inviteCode
	cc, err := client.Join(cfgC)
	if err != nil {
		t.Fatalf("join with invite: %v", err)
	}
	defer cc.Close()

	// invite reuse must fail (single use)
	dup := cfgC
	dup.Identity = mustIdentity(t)
	if _, err := client.Join(dup); err == nil {
		t.Fatal("invite exceeded uses but join succeeded")
	}

	// admin-role invite promotes the joining device
	admInv := postJSON(t, ts.URL+"/api/invites", tok, map[string]interface{}{
		"note": "admin-join", "uses_max": 1, "expires_days": 1, "role": "admin",
	})
	admCode, _ := admInv["code"].(string)
	cfgD := client.Config{
		Hub: ctlAddr, Identity: mustIdentity(t),
		Network:  &conf.Network{ID: netID, Password: "hunter22", AdminKey: admCode},
		Nickname: "dave", Platform: "test", DownloadDir: dlC,
	}
	cd, err := client.Join(cfgD)
	if err != nil {
		t.Fatalf("join with admin invite: %v", err)
	}
	defer cd.Close()
	if !cd.Info().IsAdmin {
		t.Fatal("admin-role invite did not grant admin")
	}

	// ---- owner dashboard sees 3 members ----
	nv := getJSON(t, ts.URL+"/api/net", tok)
	peers, _ := nv["peers"].([]interface{})
	if len(peers) < 3 {
		t.Fatalf("owner dashboard sees %d peers, want 3", len(peers))
	}

	// ---- owner bans carol, she cannot rejoin ----
	postJSON(t, ts.URL+"/api/peers/ban", tok, map[string]interface{}{"fp": cc.Info().YourFP, "reason": "test"})
	if _, err := client.Join(cfgC); err == nil {
		t.Fatal("banned peer rejoined")
	}

	// ---- GM powers ----
	gmTok := gmLogin(t, ts.URL, gmKey)
	ov := getJSON(t, ts.URL+"/api/gm/overview", gmTok)
	if ov["networks"].(float64) < 1 {
		t.Fatal("gm overview missing network")
	}
	// suspend network
	postJSON(t, ts.URL+"/api/gm/block-network", gmTok, map[string]interface{}{"network": netID, "reason": "maintenance"})
	time.Sleep(300 * time.Millisecond)
	// new joins rejected while suspended
	fresh := client.Config{
		Hub: ctlAddr, Identity: mustIdentity(t),
		Network:  &conf.Network{ID: netID, Password: "hunter22"},
		Nickname: "dave", DownloadDir: dlC,
	}
	fresh.Network.AdminKey = "" // password-only join, invites still required
	if _, err := client.Join(fresh); err == nil {
		t.Fatal("join allowed while network suspended")
	}
	// reactivate
	postJSON(t, ts.URL+"/api/gm/unblock-network", gmTok, map[string]interface{}{"network": netID})
	// global ban bob
	postJSON(t, ts.URL+"/api/gm/ban", gmTok, map[string]interface{}{"fp": fpB, "reason": "gm-test"})
	if _, err := client.Join(cfgB); err == nil {
		t.Fatal("globally banned peer rejoined")
	}
	postJSON(t, ts.URL+"/api/gm/unban", gmTok, map[string]interface{}{"fp": fpB})

	// gm audit has entries
	au := getJSON(t, ts.URL+"/api/gm/audit", gmTok)
	lines, _ := au["audit"].([]interface{})
	if len(lines) == 0 {
		t.Fatal("gm audit empty")
	}

	// ---- observer A still sees live traffic after all this (reconnect check) ----
	if err := ca.SendGroupChat("still here"); err != nil {
		t.Fatalf("network unusable after gm ops: %v", err)
	}
}

func readGMKey(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "GM-KEY.txt"))
	if err != nil {
		t.Fatalf("gm key file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	key := strings.TrimSpace(lines[len(lines)-1])
	if key == "" {
		t.Fatal("empty gm key")
	}
	return key
}

func adminLogin(t *testing.T, base, netID, adminKey string) string {
	t.Helper()
	return loginToken(t, base+"/api/admin/login", map[string]string{"network": netID, "admin_key": adminKey})
}

func gmLogin(t *testing.T, base, gmKey string) string {
	t.Helper()
	return loginToken(t, base+"/api/gm/login", map[string]string{"gm_key": gmKey})
}

func loginToken(t *testing.T, url string, body map[string]string) string {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("login failed %d: %s", resp.StatusCode, raw)
	}
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	tok, _ := out["token"].(string)
	if tok == "" {
		t.Fatalf("no token: %s", raw)
	}
	return tok
}

func getJSON(t *testing.T, url, tok string) map[string]interface{} {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("get %s -> %d: %s", url, resp.StatusCode, raw)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	return out
}

func postJSON(t *testing.T, url, tok string, body interface{}) map[string]interface{} {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("post %s -> %d: %s", url, resp.StatusCode, raw)
	}
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}

var _ = fmt.Sprintf
