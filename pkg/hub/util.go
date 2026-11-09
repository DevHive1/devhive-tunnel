package hub

import (
	"os"
	"path/filepath"
	"strings"

	"devhive-tunnel/pkg/crypto"
)

func openHubIdentity(dir string) (*crypto.Identity, error) {
	path := filepath.Join(dir, "hub-identity.key")
	b, err := os.ReadFile(path)
	if err == nil {
		return crypto.LoadSeedHexText(strings.TrimSpace(string(b)))
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	id, err := crypto.NewIdentity()
	if err != nil {
		return nil, err
	}
	if err2 := os.WriteFile(path, []byte(id.SaveSeedHexText()), 0o600); err2 != nil {
		return nil, err2
	}
	return id, nil
}

func sanitizeName(raw string) string {
	n := strings.TrimSpace(raw)
	n = strings.Map(func(r rune) rune {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == ' ' ||
			(r >= 0x0600 && r <= 0x06FF) || r == '،'
		if ok {
			return r
		}
		return -1
	}, n)
	for strings.Contains(n, "  ") {
		n = strings.ReplaceAll(n, "  ", " ")
	}
	if len([]rune(n)) > 32 {
		n = string([]rune(n)[:32])
	}
	return strings.TrimSpace(n)
}
