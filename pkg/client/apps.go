package client

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"devhive-tunnel/pkg/hub"
)

// FileChunkSize is the plaintext chunk size used for routed transfers.
const FileChunkSize = 192 << 10 // 192 KiB (< hub frame cap after b64)

// chatMsg is the inner plaintext of chat/DM messages.
type chatMsg struct {
	Nick string `json:"nick"`
	Text string `json:"text"`
	TS   int64  `json:"ts"`
}

// SendGroupChat broadcasts an encrypted message to the whole network.
func (c *Client) SendGroupChat(text string) error {
	inner, _ := json.Marshal(chatMsg{Nick: c.cfg.Nickname, Text: text, TS: time.Now().Unix()})
	blob, err := seal(c.grp.aead, inner)
	if err != nil {
		return err
	}
	return c.sendRoute(hub.RouteEnvelope{Kind: "gchat", Blob: blob})
}

// SendDM sends an end-to-end encrypted direct message to one peer.
func (c *Client) SendDM(peerID, text string) error {
	p := c.Peer(peerID)
	if p == nil {
		return fmt.Errorf("peer %q not found", peerID)
	}
	x, err := decodeXPub(p)
	if err != nil {
		return err
	}
	aead, err := pairwiseSeal(c.cfg.Identity, x, c.Info().NetworkID, c.Info().YourFP, p.Fingerprint)
	if err != nil {
		return err
	}
	inner, _ := json.Marshal(chatMsg{Nick: c.cfg.Nickname, Text: text, TS: time.Now().Unix()})
	blob, err := seal(aead, inner)
	if err != nil {
		return err
	}
	return c.sendRoute(hub.RouteEnvelope{Kind: "dm", ToFP: p.Fingerprint, Blob: blob})
}

// SendFile streams an encrypted file to one peer.
func (c *Client) SendFile(peerID, path string) (*FileOffer, error) {
	p := c.Peer(peerID)
	if p == nil {
		return nil, fmt.Errorf("peer %q not found", peerID)
	}
	x, err := decodeXPub(p)
	if err != nil {
		return nil, err
	}
	aead, err := pairwiseSeal(c.cfg.Identity, x, c.Info().NetworkID, c.Info().YourFP, p.Fingerprint)
	if err != nil {
		return nil, err
	}
	sum, size, err := hashFile(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	offer := &FileOffer{
		ID:      fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(4)),
		Name:    filepath.Base(path),
		Size:    size,
		SHA256:  sum,
		Chunks:  int((size + FileChunkSize - 1) / FileChunkSize),
		ChunkSz: FileChunkSize,
	}
	offerJSON, _ := json.Marshal(offer)
	blob, err := seal(aead, offerJSON)
	if err != nil {
		return nil, err
	}
	if err := c.sendRoute(hub.RouteEnvelope{Kind: "foffer", ToFP: p.Fingerprint, Blob: blob}); err != nil {
		return nil, err
	}
	buf := make([]byte, FileChunkSize)
	for idx := 0; idx < offer.Chunks; idx++ {
		n, rerr := io.ReadFull(f, buf)
		if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {
			return nil, rerr
		}
		chunk := struct {
			ID   string `json:"id"`
			Idx  int    `json:"idx"`
			Data string `json:"data"`
		}{offer.ID, idx, base64.StdEncoding.EncodeToString(buf[:n])}
		inner, _ := json.Marshal(chunk)
		cb, err := seal(aead, inner)
		if err != nil {
			return nil, err
		}
		if err := c.sendRoute(hub.RouteEnvelope{Kind: "fchunk", ToFP: p.Fingerprint, Blob: cb}); err != nil {
			return nil, err
		}
	}
	c.emit(Event{Type: EvFileDone, Peer: p, File: offer, Note: "sent", TS: time.Now()})
	return offer, nil
}

// handleEnvelope decrypts one inbound routed envelope.
func (c *Client) handleEnvelope(env hub.RouteEnvelope) {
	p := c.Peer(env.FromFP)
	from := env.FromFP
	if p != nil {
		from = p.Nickname
	}
	switch env.Kind {
	case "gchat":
		pt, err := open(c.grp.aead, env.Blob)
		if err != nil {
			c.emit(Event{Type: EvError, From: env.FromFP, Err: err.Error(), TS: time.Now()})
			return
		}
		var msg chatMsg
		if json.Unmarshal(pt, &msg) != nil {
			return
		}
		c.emit(Event{Type: EvGroupChat, From: env.FromFP, Text: msg.Nick + ": " + msg.Text, TS: time.Now()})
	case "dm":
		aead, err := c.peerAEAD(env.FromFP)
		if err != nil {
			c.emit(Event{Type: EvError, From: env.FromFP, Err: err.Error(), TS: time.Now()})
			return
		}
		pt, err := open(aead, env.Blob)
		if err != nil {
			c.emit(Event{Type: EvError, From: env.FromFP, Err: err.Error(), TS: time.Now()})
			return
		}
		var msg chatMsg
		if json.Unmarshal(pt, &msg) != nil {
			return
		}
		c.emit(Event{Type: EvDM, From: env.FromFP, Text: msg.Nick + ": " + msg.Text, TS: time.Now()})
	case "foffer":
		aead, err := c.peerAEAD(env.FromFP)
		if err != nil {
			c.emit(Event{Type: EvError, From: env.FromFP, Err: err.Error(), TS: time.Now()})
			return
		}
		pt, err := open(aead, env.Blob)
		if err != nil {
			c.emit(Event{Type: EvError, From: env.FromFP, Err: err.Error(), TS: time.Now()})
			return
		}
		var offer FileOffer
		if json.Unmarshal(pt, &offer) != nil {
			return
		}
		offer.From = env.FromFP
		saved, err := c.files.begin(&offer)
		if err != nil {
			c.emit(Event{Type: EvError, From: env.FromFP, Err: err.Error(), TS: time.Now()})
			return
		}
		logf("receiving %q (%d bytes) from %s", offer.Name, offer.Size, from)
		c.emit(Event{Type: EvFileOffer, From: env.FromFP, File: &offer, Note: saved, TS: time.Now()})
	case "fchunk":
		aead, err := c.peerAEAD(env.FromFP)
		if err != nil {
			return
		}
		pt, err := open(aead, env.Blob)
		if err != nil {
			return
		}
		var chunk struct {
			ID   string `json:"id"`
			Idx  int    `json:"idx"`
			Data string `json:"data"`
		}
		if json.Unmarshal(pt, &chunk) != nil {
			return
		}
		data, err := base64.StdEncoding.DecodeString(chunk.Data)
		if err != nil {
			return
		}
		done, final, err := c.files.chunk(chunk.ID, chunk.Idx, data)
		if err != nil {
			c.emit(Event{Type: EvError, From: env.FromFP, Err: err.Error(), TS: time.Now()})
			c.files.abort(chunk.ID)
			return
		}
		if done {
			logf("file saved to %s", final)
			c.emit(Event{Type: EvFileDone, From: env.FromFP, Note: final, TS: time.Now()})
		}
	case "ip":
		aead, err := c.peerAEAD(env.FromFP)
		if err != nil {
			return
		}
		pt, err := open(aead, env.Blob)
		if err != nil {
			return
		}
		if len(pt) >= 20 {
			c.emit(Event{Type: EvIP, From: env.FromFP, Packet: pt, TS: time.Now()})
		}
	default:
		c.emit(Event{Type: EvError, From: env.FromFP, Err: "unknown message kind " + env.Kind, TS: time.Now()})
	}
}

// SendIP seals one raw IPv4 packet to a peer (LAN mode).
func (c *Client) SendIP(toFP string, pkt []byte) error {
	p := c.Peer(toFP)
	if p == nil {
		return fmt.Errorf("unknown peer %q", toFP)
	}
	aead, err := c.peerAEAD(p.Fingerprint)
	if err != nil {
		return err
	}
	blob, err := seal(aead, pkt)
	if err != nil {
		return err
	}
	return c.sendRoute(hub.RouteEnvelope{Kind: "ip", ToFP: p.Fingerprint, Blob: blob})
}

// PeerByVLANIP returns the online peer owning a virtual LAN address.
func (c *Client) PeerByVLANIP(ip string) *hub.PeerRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for fp, p := range c.peers {
		if fp == c.info.YourFP {
			continue
		}
		if p.VLANIP == ip && p.Online {
			cp := *p
			return &cp
		}
	}
	return nil
}

func (c *Client) peerAEAD(fp string) (cipher.AEAD, error) {
	p := c.Peer(fp)
	if p == nil {
		return nil, fmt.Errorf("sender is not a known peer")
	}
	x, err := decodeXPub(p)
	if err != nil {
		return nil, err
	}
	return pairwiseSeal(c.cfg.Identity, x, c.Info().NetworkID, c.Info().YourFP, p.Fingerprint)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

var logf = log.Printf
