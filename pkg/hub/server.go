package hub

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"devhive-tunnel/pkg/crypto"
)

// ServerOptions configures a hub instance.
type ServerOptions struct {
	Dir    string
	Listen string
	Web    string
	Subnet string
}

// Server is the running hub coordinator. Server.mu serializes all store
// mutations and live-link bookkeeping.
type Server struct {
	opts  ServerOptions
	store *Store
	id    *crypto.Identity

	mu         sync.Mutex
	live       map[string]map[string]*link // netID -> fp -> link
	joins      map[joinKey]int
	joinsNet   map[joinNetKey]int
	tokens     map[string]token
	loginTries map[string]loginTry
	stats      Stats
	started    time.Time
}

// token is a dashboard session credential.
type token struct {
	kind   string // "gm" | "admin"
	netID  string
	expiry time.Time
}

type loginTry struct {
	count int
	reset time.Time
}

// Stats are live hub analytics.
type Stats struct {
	JoinAttempts uint64
	AuthFails    uint64
	RoutedFrames uint64
	EventsPushed uint64
	PeakOnline   int
}

type link struct {
	sess  *crypto.Session
	fp    string
	netID string
}

type joinKey struct {
	host   string
	minute int64
}

type joinNetKey struct {
	netID  string
	host   string
	minute int64
}

var (
	ErrNetworkFull   = errors.New("devhive-hub: network is full")
	ErrBanned        = errors.New("devhive-hub: peer is banned")
	ErrNeedInvite    = errors.New("devhive-hub: invite key required")
	ErrWrongPassword = errors.New("devhive-hub: wrong password")
	ErrNameTaken     = errors.New("devhive-hub: name taken")
	ErrTooManyJoins  = errors.New("devhive-hub: too many attempts")
	ErrNotAdmin      = errors.New("devhive-hub: not an admin")
	ErrNetBlocked    = errors.New("devhive-hub: network suspended by GM")
)

// OpenServer loads state + identity (+ creates the GM key on first run).
func OpenServer(opts ServerOptions) (*Server, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Subnet == "" {
		opts.Subnet = "100.72.0.0/16"
	}
	if opts.Listen == "" {
		opts.Listen = ":7676"
	}
	if opts.Web == "" {
		opts.Web = ":7600"
	}
	store, err := Open(opts.Dir)
	if err != nil {
		return nil, err
	}
	id, err := openHubIdentity(opts.Dir)
	if err != nil {
		return nil, err
	}
	s := &Server{
		opts: opts, store: store, id: id,
		live:       map[string]map[string]*link{},
		joins:      map[joinKey]int{},
		joinsNet:   map[joinNetKey]int{},
		tokens:     map[string]token{},
		loginTries: map[string]loginTry{},
		started:    time.Now(),
	}
	if err := s.ensureGMKey(); err != nil {
		return nil, err
	}
	return s, nil
}

// ensureGMKey creates + persists the GM key on first run.
func (s *Server) ensureGMKey() error {
	if s.store.GMHash != "" {
		return nil
	}
	gmKey := crypto.NewGMKey()
	s.store.GMHash = crypto.HashAdminKey(gmKey)
	path := filepath.Join(s.opts.Dir, "GM-KEY.txt")
	if err := os.WriteFile(path, []byte("devhive GM key (keep secret):\n"+gmKey+"\n"), 0o600); err != nil {
		return err
	}
	if err := s.store.Save(); err != nil {
		return err
	}
	log.Printf("[hub] ==================================================")
	log.Printf("[hub] GM superuser key created. Store it safely now;")
	log.Printf("[hub] it is also written to %s", path)
	log.Printf("[hub] GM key: %s", gmKey)
	log.Printf("[hub] ==================================================")
	return nil
}

// Store returns the underlying store.
func (s *Server) Store() *Store { return s.store }

// Identity returns the hub identity.
func (s *Server) Identity() *crypto.Identity { return s.id }
