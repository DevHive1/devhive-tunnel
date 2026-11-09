// Package conf manages client configuration + identity storage.
package conf

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Version of the devhive-tunnel suite.
const Version = "0.1.0"

// DefaultHub is the placeholder coordination address shown in the wizard.
const DefaultHub = "127.0.0.1:7676"

// Network holds credentials for one joined/created network.
type Network struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Password string `json:"password"`
	AdminKey string `json:"admin_key,omitempty"`
	Created  bool   `json:"created"`
	Hub      string `json:"hub,omitempty"`
}

// Client is the persistent client configuration.
type Client struct {
	Language string     `json:"language"`
	Hub      string     `json:"hub"`
	Nickname string     `json:"nickname"`
	Networks []*Network `json:"networks"`
	dataDir  string
}

// Dir returns the configuration directory.
func (c *Client) Dir() string { return c.dataDir }

// IdentityPath is the file holding the hex identity seed.
func (c *Client) IdentityPath() string { return filepath.Join(c.dataDir, "identity.key") }

// DownloadsDir is the default place for received files.
func (c *Client) DownloadsDir() string { return filepath.Join(c.dataDir, "downloads") }

// DefaultDir returns the per-OS config directory.
// - Termux native (android): $HOME/.devhive-tunnel
// - Windows: %APPDATA%\devhive-tunnel
// - Linux/other: ~/.devhive-tunnel
func DefaultDir() string {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "devhive-tunnel")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".devhive-tunnel")
}

// Load reads config, creating a fresh one when missing.
func Load(dir string) (*Client, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	c := &Client{dataDir: dir, Hub: DefaultHub}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.DownloadsDir(), 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil // first run
		}
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("config corrupt (%s): %w", path, err)
	}
	c.dataDir = dir
	if c.Hub == "" {
		c.Hub = DefaultHub
	}
	return c, nil
}

// Save persists config with restrictive permissions.
func (c *Client) Save() error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dataDir, "config.json"), b, 0o600)
}

// AddNetwork registers credentials for a network (upsert by ID).
func (c *Client) AddNetwork(n *Network) {
	for i, old := range c.Networks {
		if old.ID == n.ID {
			c.Networks[i] = n
			return
		}
	}
	c.Networks = append(c.Networks, n)
}

// FindNetwork looks up by ID first, then joinable name (case-insensitive).
func (c *Client) FindNetwork(idOrName string) *Network {
	for _, n := range c.Networks {
		if n.ID == idOrName || lower(n.Name) == lower(idOrName) {
			return n
		}
	}
	return nil
}

// ResetIdentity removes the stored identity seed (logout of tunnel identity).
func (c *Client) ResetIdentity() error {
	err := os.Remove(c.IdentityPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// StoreSeedHex persists the identity seed (0600).
func (c *Client) StoreSeedHex(hexSeed string) error {
	return os.WriteFile(c.IdentityPath(), []byte(hexSeed), 0o600)
}

// LoadSeedHex reads the stored identity seed ("" if missing).
func (c *Client) LoadSeedHex() (string, error) {
	b, err := os.ReadFile(c.IdentityPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// RandomToken returns a fresh random hex token.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, 0, 2*n)
	for _, x := range b {
		out = append(out, hexDigits[int(x)%len(hexDigits)])
	}
	return string(out), nil
}

const hexDigits = "0123456789abcdef"

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
