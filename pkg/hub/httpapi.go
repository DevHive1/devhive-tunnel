package hub

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"devhive-tunnel/pkg/crypto"
)

// httpAPI serves the dashboard + REST API.
type api struct{ srv *Server }

func (a *api) mux() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", a.static)
	m.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"ok": "1"})
	})
	m.HandleFunc("POST /api/admin/login", a.adminLogin)
	m.HandleFunc("POST /api/gm/login", a.gmLogin)
	m.HandleFunc("POST /api/logout", a.logout)
	m.HandleFunc("GET /api/net", a.auth(a.adminNet))
	m.HandleFunc("POST /api/net/settings", a.auth(a.adminSettings))
	m.HandleFunc("POST /api/net/rotate-admin", a.auth(a.adminRotate))
	m.HandleFunc("GET /api/invites", a.auth(a.adminInvites))
	m.HandleFunc("POST /api/invites", a.auth(a.adminInviteCreate))
	m.HandleFunc("POST /api/invites/revoke", a.auth(a.adminInviteRevoke))
	m.HandleFunc("POST /api/peers/kick", a.auth(a.adminKick))
	m.HandleFunc("POST /api/peers/ban", a.auth(a.adminBan))
	m.HandleFunc("POST /api/peers/unban", a.auth(a.adminUnban))
	m.HandleFunc("POST /api/peers/promote", a.auth(a.adminPromote))
	m.HandleFunc("GET /api/audit", a.auth(a.adminAudit))
	m.HandleFunc("GET /api/gm/live", a.gmLive)
	m.HandleFunc("GET /api/gm/overview", a.authGM(a.gmOverview))
	m.HandleFunc("GET /api/gm/networks", a.authGM(a.gmNetworks))
	m.HandleFunc("GET /api/gm/network", a.authGM(a.gmNetwork))
	m.HandleFunc("GET /api/gm/analytics", a.authGM(a.gmAnalytics))
	m.HandleFunc("GET /api/gm/audit", a.authGM(a.gmAudit))
	m.HandleFunc("POST /api/gm/ban", a.authGM(a.gmBan))
	m.HandleFunc("POST /api/gm/unban", a.authGM(a.gmUnban))
	m.HandleFunc("POST /api/gm/kick", a.authGM(a.gmKick))
	m.HandleFunc("POST /api/gm/block-network", a.authGM(a.gmBlockNet))
	m.HandleFunc("POST /api/gm/unblock-network", a.authGM(a.gmUnblockNet))
	m.HandleFunc("POST /api/gm/rotate-admin", a.authGM(a.gmRotateAdmin))
	m.HandleFunc("POST /api/gm/delete-network", a.authGM(a.gmDeleteNet))
	return m
}

// writeJSON emits a JSON response with security headers.
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(dst)
}

// issueToken creates a session token valid for 12h.
func (s *Server) issueToken(kind, netID string) string {
	tok := crypto.NewSessionToken()
	s.mu.Lock()
	s.tokens[tok] = token{kind: kind, netID: netID, expiry: time.Now().Add(12 * time.Hour)}
	s.mu.Unlock()
	return tok
}

// checkToken validates a bearer token.
func (s *Server) checkToken(tok string) (token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[tok]
	if !ok {
		return token{}, false
	}
	if time.Now().After(t.expiry) {
		delete(s.tokens, tok)
		return token{}, false
	}
	return t, true
}

// loginLimiter throttles login attempts per IP: 10 per 2 minutes.
func (s *Server) loginAllowed(host string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	lt := s.loginTries[host]
	if lt.reset.Before(now) {
		lt = loginTry{count: 0, reset: now.Add(2 * time.Minute)}
	}
	lt.count++
	s.loginTries[host] = lt
	return lt.count <= 10
}

func clientHost(r *http.Request) string {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	return strings.TrimPrefix(h, "Bearer ")
}

// auth wraps admin-token endpoints and injects the network.
func (a *api) auth(h func(w http.ResponseWriter, r *http.Request, tk token)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tk, ok := a.srv.checkToken(bearer(r))
		if !ok || tk.kind != "admin" {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		h(w, r, tk)
	}
}

// authGM wraps GM-token endpoints.
func (a *api) authGM(h func(w http.ResponseWriter, r *http.Request, tk token)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tk, ok := a.srv.checkToken(bearer(r))
		if !ok || tk.kind != "gm" {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		h(w, r, tk)
	}
}

// adminLogin validates a network admin key and returns a token.
func (a *api) adminLogin(w http.ResponseWriter, r *http.Request) {
	if !a.srv.loginAllowed(clientHost(r)) {
		writeJSON(w, 429, map[string]string{"error": "rate_limited"})
		return
	}
	var req struct {
		Network  string `json:"network"`
		AdminKey string `json:"admin_key"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	n, ok := s.store.Lookup(req.Network)
	if !ok || !crypto.CheckAdminKey(req.AdminKey, n.AdminHash) {
		s.mu.Unlock()
		s.mu.Lock()
		s.stats.AuthFails++
		s.store.AppendAudit("dashboard", "ADMIN_LOGIN_FAIL", "network="+req.Network)
		_ = s.store.Save()
		s.mu.Unlock()
		writeJSON(w, 401, map[string]string{"error": "bad_credentials"})
		return
	}
	n.AppendAudit("dashboard", "ADMIN_LOGIN", "")
	_ = s.store.Save()
	s.mu.Unlock()
	tok := s.issueToken("admin", n.ID)
	writeJSON(w, 200, map[string]string{
		"token": tok, "network_id": n.ID, "network_name": n.Name, "kind": "admin",
	})
}

// gmLogin validates the hub superuser key.
func (a *api) gmLogin(w http.ResponseWriter, r *http.Request) {
	if !a.srv.loginAllowed(clientHost(r)) {
		writeJSON(w, 429, map[string]string{"error": "rate_limited"})
		return
	}
	var req struct {
		GMKey string `json:"gm_key"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad_json"})
		return
	}
	s := a.srv
	s.mu.Lock()
	ok := s.store.GMHash != "" && crypto.HashAdminKey(req.GMKey) == s.store.GMHash
	if !ok {
		s.stats.AuthFails++
		s.store.AppendAudit("dashboard", "GM_LOGIN_FAIL", "")
		_ = s.store.Save()
		s.mu.Unlock()
		writeJSON(w, 401, map[string]string{"error": "bad_credentials"})
		return
	}
	s.store.AppendAudit("dashboard", "GM_LOGIN", "")
	_ = s.store.Save()
	s.mu.Unlock()
	tok := s.issueToken("gm", "")
	writeJSON(w, 200, map[string]string{"token": tok, "kind": "gm"})
}

// logout invalidates the presented token.
func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	tok := bearer(r)
	a.srv.mu.Lock()
	delete(a.srv.tokens, tok)
	a.srv.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}
