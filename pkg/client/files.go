package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// recvFile tracks one inbound transfer.
type recvFile struct {
	offer    FileOffer
	f        *os.File
	partPath string
	next     int
	h        hash.Hash
}

// fileStore manages inbound transfers for a client.
type fileStore struct {
	mu   sync.Mutex
	dir  string
	recv map[string]*recvFile
}

func newFileStore(dir string) *fileStore {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "devhive-downloads")
	}
	_ = os.MkdirAll(dir, 0o700)
	return &fileStore{dir: dir, recv: map[string]*recvFile{}}
}

// begin prepares a .part file for an accepted offer.
func (fs *fileStore) begin(offer *FileOffer) (savedPath string, err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if _, exists := fs.recv[offer.ID]; exists {
		return "", fmt.Errorf("duplicate file id")
	}
	name := filepath.Base(offer.Name)
	if name == "" || name == "." || name == ".." {
		name = "file-" + offer.ID
	}
	final := filepath.Join(fs.dir, name)
	for i := 1; ; i++ {
		if _, statErr := os.Stat(final); os.IsNotExist(statErr) {
			break
		}
		final = filepath.Join(fs.dir, fmt.Sprintf("%s (%d)%s", trimExt(name), i, filepath.Ext(name)))
	}
	part := final + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	fs.recv[offer.ID] = &recvFile{offer: *offer, f: f, partPath: part, h: sha256.New()}
	return final, nil
}

func trimExt(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}

// chunk appends the next chunk; returns done + final path when complete.
func (fs *fileStore) chunk(id string, idx int, data []byte) (bool, string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	rf := fs.recv[id]
	if rf == nil {
		return false, "", fmt.Errorf("unknown transfer")
	}
	if idx != rf.next {
		return false, "", fmt.Errorf("chunk %d out of order (expected %d)", idx, rf.next)
	}
	if _, err := rf.f.Write(data); err != nil {
		return false, "", err
	}
	rf.h.Write(data)
	rf.next++
	if rf.next < rf.offer.Chunks {
		return false, "", nil
	}
	if err := rf.f.Close(); err != nil {
		delete(fs.recv, id)
		return false, "", err
	}
	sum := hex.EncodeToString(rf.h.Sum(nil))
	final := rf.partPath[:len(rf.partPath)-len(".part")]
	delete(fs.recv, id)
	if sum != rf.offer.SHA256 {
		_ = os.Remove(rf.partPath)
		return false, "", fmt.Errorf("checksum mismatch (expected %s got %s)", rf.offer.SHA256, sum)
	}
	if err := os.Rename(rf.partPath, final); err != nil {
		return false, "", err
	}
	return true, final, nil
}

// abort cleans up a transfer.
func (fs *fileStore) abort(id string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if rf := fs.recv[id]; rf != nil {
		_ = rf.f.Close()
		_ = os.Remove(rf.partPath)
		delete(fs.recv, id)
	}
}

// hashFile computes sha256 and size of a local file.
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
