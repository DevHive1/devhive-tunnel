package hub

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store persists all hub state as one JSON document (atomic writes).
// Locking: all access is serialized by Server.mu; Store itself is lock-free.
type Store struct {
	dir string

	GMHash     string                `json:"gm_hash"`
	GlobalBans map[string]*GlobalBan `json:"global_bans"` // fp -> ban
	Audit      []AuditLine           `json:"audit"`
	Nets       map[string]*NetRecord `json:"nets"`
	Names      map[string]string     `json:"-"` // lower(name) -> netID (derived)
}

// GlobalBan blocks a fingerprint on every network of this hub.
type GlobalBan struct {
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
	TS          string `json:"ts"`
	Actor       string `json:"actor"`
}

type jsonDoc struct {
	Version    int                   `json:"version"`
	GMHash     string                `json:"gm_hash"`
	GlobalBans map[string]*GlobalBan `json:"global_bans"`
	Audit      []AuditLine           `json:"audit"`
	Nets       map[string]*NetRecord `json:"nets"`
}

// Open loads (or initializes) the store in dir.
func Open(dir string) (*Store, error) {
	s := &Store{
		dir:        dir,
		GlobalBans: map[string]*GlobalBan{},
		Nets:       map[string]*NetRecord{},
		Names:      map[string]string{},
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.docPath())
	if err == nil {
		var doc jsonDoc
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("hub store corrupt: %w", err)
		}
		s.GMHash = doc.GMHash
		s.GlobalBans = doc.GlobalBans
		s.Audit = doc.Audit
		s.Nets = doc.Nets
		if s.GlobalBans == nil {
			s.GlobalBans = map[string]*GlobalBan{}
		}
		if s.Nets == nil {
			s.Nets = map[string]*NetRecord{}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	s.Reindex()
	return s, nil
}

func (s *Store) docPath() string { return filepath.Join(s.dir, "hub.json") }

// Save atomically persists (caller must hold Server.mu).
func (s *Store) Save() error {
	doc := jsonDoc{
		Version: 1, GMHash: s.GMHash, GlobalBans: s.GlobalBans,
		Audit: s.Audit, Nets: s.Nets,
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.docPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.docPath())
}

// Reindex rebuilds the derived name index (caller holds Server.mu).
func (s *Store) Reindex() {
	s.Names = map[string]string{}
	for id, n := range s.Nets {
		if n.Name != "" {
			s.Names[strings.ToLower(n.Name)] = id
		}
	}
}

// Lookup resolves by exact ID or lower-cased name (caller holds Server.mu).
func (s *Store) Lookup(idOrName string) (*NetRecord, bool) {
	if n, ok := s.Nets[idOrName]; ok {
		return n, true
	}
	if id, ok := s.Names[strings.ToLower(strings.TrimSpace(idOrName))]; ok {
		return s.Nets[id], true
	}
	return nil, false
}

// AppendAudit adds a hub-level audit line (GM actions, bans...).
func (s *Store) AppendAudit(actor, event, detail string) {
	s.Audit = append(s.Audit, AuditLine{TS: time.Now().UTC().Format(time.RFC3339), Actor: actor, Event: event, Detail: detail})
	if len(s.Audit) > 5000 {
		s.Audit = s.Audit[len(s.Audit)-5000:]
	}
}

// NextVLANIP returns the next free host address inside cidr.
func NextVLANIP(cidr string, taken map[string]bool, startHost int) string {
	pfx, err := netip.ParsePrefix(cidr)
	if err != nil || !pfx.Addr().Is4() {
		return ""
	}
	a4 := pfx.Addr().As4()
	if startHost < 1 {
		startHost = 1
	}
	for i := startHost; i < 0xFFFE; i++ {
		ip := a4
		ip[2] = byte((i >> 8) & 0xFF)
		ip[3] = byte(i & 0xFF)
		cand := netip.AddrFrom4(ip)
		if !pfx.Contains(cand) {
			break
		}
		if ip[3] == 0 || ip[3] == 255 {
			continue
		}
		if !taken[cand.String()] {
			return cand.String()
		}
	}
	return ""
}

// PubToFP derives the peer fingerprint from a base64 Ed25519 key.
func PubToFP(pubB64 string) string {
	raw, err := base64.RawStdEncoding.DecodeString(pubB64)
	if err != nil || len(raw) != 32 {
		return "badkey"
	}
	h := sha256.Sum256(raw)
	return fmt.Sprintf("%x", h[:])[:12]
}

// OnlinePeers counts online peers in one network.
func (n *NetRecord) OnlinePeers() int {
	c := 0
	for _, p := range n.Peers {
		if p.Online {
			c++
		}
	}
	return c
}

// PeerMapSnapshot returns a sorted copy of peers.
func (n *NetRecord) PeerMapSnapshot() []*PeerRecord {
	out := make([]*PeerRecord, 0, len(n.Peers))
	for _, p := range n.Peers {
		cp := *p
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Online != out[j].Online {
			return out[i].Online
		}
		return out[i].Nickname < out[j].Nickname
	})
	return out
}

// AppendAudit adds a per-network audit line.
func (n *NetRecord) AppendAudit(actor, event, detail string) {
	n.Audit = append(n.Audit, AuditLine{TS: time.Now().UTC().Format(time.RFC3339), Actor: actor, Event: event, Detail: detail})
	if len(n.Audit) > 1000 {
		n.Audit = n.Audit[len(n.Audit)-1000:]
	}
}
