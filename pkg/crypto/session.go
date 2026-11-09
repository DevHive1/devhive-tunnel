package crypto

import (
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// Reserved frame types. All other values are free for application use.
const (
	TypePing byte = 0x01
	TypeBye  byte = 0x02
)

// Transport limits.
const (
	frameVersion  = 1
	maxFrameSize  = 8 << 20
	maxHShakeSize = 4096 + 512
)

var (
	// ErrCorrupt marks a tampered or malformed frame.
	ErrCorrupt = errors.New("devhive: corrupt frame")
	// ErrSessionClosed is returned when the peer closed the connection.
	ErrSessionClosed = errors.New("devhive: session closed")
)

// frameLayout: [ver u8][type u8][counter u64][ctlen u32][ ciphertext ]
const (
	headerLen  = 1 + 1 + 8 + 4
	tagOverhad = 16
)

// Session is an authenticated, encrypted, framed channel over a stream.
type Session struct {
	conn      net.Conn
	sendAEAD  cipher.AEAD
	recvAEAD  cipher.AEAD
	sendSeq   uint64
	recvHigh  uint64
	remoteX   [DHResultLen]byte
	remoteE   ed25519.PublicKey
	initiator bool
	blobSent  []byte
	blobRecv  []byte
}

// RemoteEd returns the peer's Ed25519 public key from the handshake.
func (s *Session) RemoteEd() ed25519.PublicKey { return s.remoteE }

// RemoteX returns the peer's X25519 static public key from the handshake.
func (s *Session) RemoteX() [DHResultLen]byte { return s.remoteX }

// SentBlob returns application data offered in the local handshake.
func (s *Session) SentBlob() []byte { return s.blobSent }

// ReceivedBlob returns data the peer offered in its handshake.
func (s *Session) ReceivedBlob() []byte { return s.blobRecv }

// IsInitiator reports whether this side sent the first handshake message.
func (s *Session) IsInitiator() bool { return s.initiator }

// RawConn exposes the underlying connection (for deadlines / tests).
func (s *Session) RawConn() net.Conn { return s.conn }

// Ping sends an empty keepalive frame.
func (s *Session) Ping() error { return s.Write(TypePing, nil) }

// Close performs a graceful encrypted close then closes the connection.
func (s *Session) Close() error {
	_ = s.Write(TypeBye, nil)
	return s.conn.Close()
}

// ---------------------------------------------------------------------------
// Handshake (Signed ephemeral / static scalar X25519, Noise-KK spirit)

// HandshakeInitiator runs the client side of the handshake on conn.
// expectPeer may be nil (TOFU). blob is authenticated application data
// delivered to the responder (e.g. join credential material).
func HandshakeInitiator(conn net.Conn, id *Identity, expectPeer ed25519.PublicKey, blob []byte) (*Session, error) {
	ephSec, ephPub, err := newX25519Keypair()
	if err != nil {
		return nil, err
	}
	ts := uint64(time.Now().Unix())
	digest1 := handshakeDigest("DH1", ts, ephPub, id.xPub, blob)
	msg1 := packHandshake("DH1", ts, ephPub, id.xPub, id.SignPub(), blob, id.Sign(digest1))
	if err := writePrefixed(conn, msg1); err != nil {
		return nil, err
	}
	raw, err := readPrefixed(conn, maxHShakeSize)
	if err != nil {
		return nil, err
	}
	role, ts2, eR, sR, edR, blobR, sigR, err := unpackHandshake(raw)
	if err != nil {
		return nil, err
	}
	if role != "DH2" {
		return nil, fmt.Errorf("%w: expected DH2 got %q", ErrBadHandshake, role)
	}
	digest2 := handshakeDigest("DH2", ts2, eR, sR, blobR, digest1)
	if !ed25519.Verify(edR, digest2, sigR) {
		return nil, fmt.Errorf("%w: responder signature invalid", ErrBadHandshake)
	}
	if expectPeer != nil && !edPubEqual(expectPeer, edR) {
		return nil, fmt.Errorf("%w: responder identity mismatch", ErrBadHandshake)
	}
	dhA, err := x25519Shared(ephSec, sR)
	if err != nil {
		return nil, err
	}
	dhB, err := x25519Shared(id.xPriv, eR)
	if err != nil {
		return nil, err
	}
	dhC, err := x25519Shared(ephSec, eR)
	if err != nil {
		return nil, err
	}
	s, err := newSession(conn, true, mixKeys(dhA, dhB, dhC))
	if err != nil {
		return nil, err
	}
	s.remoteX, s.remoteE = sR, edR
	s.blobSent, s.blobRecv = blob, blobR
	return s, nil
}

// HandshakeResponder runs the server side of the handshake on conn.
func HandshakeResponder(conn net.Conn, id *Identity, blob []byte) (*Session, error) {
	raw, err := readPrefixed(conn, maxHShakeSize)
	if err != nil {
		return nil, err
	}
	role, ts1, eI, sI, edI, blobI, sigI, err := unpackHandshake(raw)
	if err != nil {
		return nil, err
	}
	if role != "DH1" {
		return nil, fmt.Errorf("%w: expected DH1 got %q", ErrBadHandshake, role)
	}
	digest1 := handshakeDigest("DH1", ts1, eI, sI, blobI)
	if !ed25519.Verify(edI, digest1, sigI) {
		return nil, fmt.Errorf("%w: initiator signature invalid", ErrBadHandshake)
	}
	ephSec, ephPub, err := newX25519Keypair()
	if err != nil {
		return nil, err
	}
	ts2 := uint64(time.Now().Unix())
	digest2 := handshakeDigest("DH2", ts2, ephPub, id.xPub, blob, digest1)
	msg2 := packHandshake("DH2", ts2, ephPub, id.xPub, id.SignPub(), blob, id.Sign(digest2))
	if err := writePrefixed(conn, msg2); err != nil {
		return nil, err
	}
	dhB, err := x25519Shared(ephSec, sI)
	if err != nil {
		return nil, err
	}
	dhA, err := x25519Shared(id.xPriv, eI)
	if err != nil {
		return nil, err
	}
	dhC, err := x25519Shared(ephSec, eI)
	if err != nil {
		return nil, err
	}
	s, err := newSession(conn, false, mixKeys(dhA, dhB, dhC))
	if err != nil {
		return nil, err
	}
	s.remoteX, s.remoteE = sI, edI
	s.blobSent, s.blobRecv = blob, blobI
	return s, nil
}

func edPubEqual(a, b ed25519.PublicKey) bool {
	if len(a) != ed25519.PublicKeySize || len(b) != ed25519.PublicKeySize {
		return false
	}
	return string(a) == string(b)
}

// handshakeDigest binds the role, timestamps, ephemerals, static keys and
// any extra application material into one transcript digest to be signed.
func handshakeDigest(role string, ts uint64, eph, st [DHResultLen]byte, extra ...[]byte) []byte {
	h := sha256.New()
	h.Write([]byte("devhive-tunnel/" + role + "/v1"))
	var tb [8]byte
	binary.BigEndian.PutUint64(tb[:], ts)
	h.Write(tb[:])
	h.Write(eph[:])
	h.Write(st[:])
	for _, x := range extra {
		binary.BigEndian.PutUint16(tb[:2], uint16(len(x)))
		h.Write(tb[:2])
		h.Write(x)
	}
	return h.Sum(nil)
}

// packHandshake: "DHT" | role[3] | ts[8] | eph[32] | st[32] | edPub[32] |
// blob u16+len | sig u16+len
func packHandshake(role string, ts uint64, eph, st [DHResultLen]byte, ed ed25519.PublicKey, blob, sig []byte) []byte {
	out := make([]byte, 0, 6+8+3*DHResultLen+4+len(blob)+len(sig))
	out = append(out, 'D', 'H', 'T')
	out = append(out, role...)
	var tb [8]byte
	binary.BigEndian.PutUint64(tb[:], ts)
	out = append(out, tb[:]...)
	out = append(out, eph[:]...)
	out = append(out, st[:]...)
	out = append(out, ed...)
	out = append(out, byte(len(blob)>>8), byte(len(blob)))
	out = append(out, blob...)
	out = append(out, byte(len(sig)>>8), byte(len(sig)))
	out = append(out, sig...)
	return out
}

func unpackHandshake(msg []byte) (role string, ts uint64, eph, st [DHResultLen]byte, ed ed25519.PublicKey, blob, sig []byte, err error) {
	bad := fmt.Errorf("%w: bad layout", ErrBadHandshake)
	if len(msg) < 6+8+3*DHResultLen+ed25519.PublicKeySize+4 {
		err = bad
		return
	}
	if string(msg[0:3]) != "DHT" {
		err = bad
		return
	}
	role = string(msg[3:6])
	off := 6
	ts = binary.BigEndian.Uint64(msg[off : off+8])
	off += 8
	copy(eph[:], msg[off:off+DHResultLen])
	off += DHResultLen
	copy(st[:], msg[off:off+DHResultLen])
	off += DHResultLen
	ed = make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(ed, msg[off:off+ed25519.PublicKeySize])
	off += ed25519.PublicKeySize
	if off+2 > len(msg) {
		err = bad
		return
	}
	bl := int(binary.BigEndian.Uint16(msg[off : off+2]))
	off += 2
	if bl > 4096 || off+bl > len(msg) {
		err = bad
		return
	}
	blob = append([]byte(nil), msg[off:off+bl]...)
	off += bl
	if off+2 > len(msg) {
		err = bad
		return
	}
	sl := int(binary.BigEndian.Uint16(msg[off : off+2]))
	off += 2
	if sl != ed25519.SignatureSize || off+sl > len(msg) {
		err = bad
		return
	}
	sig = make([]byte, sl)
	copy(sig, msg[off:off+sl])
	return role, ts, eph, st, ed, blob, sig, nil
}

func writePrefixed(conn io.Writer, msg []byte) error {
	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(len(msg)))
	if _, err := conn.Write(lb[:]); err != nil {
		return err
	}
	_, err := conn.Write(msg)
	return err
}

func readPrefixed(conn io.Reader, max int) ([]byte, error) {
	var lb [4]byte
	if _, err := io.ReadFull(conn, lb[:]); err != nil {
		return nil, mapErr(err)
	}
	n := binary.BigEndian.Uint32(lb[:])
	if n > uint32(max) {
		return nil, fmt.Errorf("%w: message too big (%d)", ErrCorrupt, n)
	}
	msg := make([]byte, n)
	if _, err := io.ReadFull(conn, msg); err != nil {
		return nil, mapErr(err)
	}
	return msg, nil
}

func newX25519Keypair() (priv, pub [DHResultLen]byte, err error) {
	if _, err := rand.Read(priv[:]); err != nil {
		return priv, pub, err
	}
	p, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return priv, pub, err
	}
	copy(pub[:], p)
	return priv, pub, nil
}

func x25519Shared(priv, peer [DHResultLen]byte) (out [DHResultLen]byte, err error) {
	s, err := curve25519.X25519(priv[:], peer[:])
	if err != nil {
		return out, fmt.Errorf("%w: weak dh", ErrBadHandshake)
	}
	copy(out[:], s)
	return out, nil
}

// sessionKeys: initiator→responder key is k12, responder→initiator is k21.
type sessionKeys struct {
	k12 [chacha20poly1305.KeySize]byte
	k21 [chacha20poly1305.KeySize]byte
}

// mixKeys expands the triple DH exchange into both directional session keys.
func mixKeys(a, b, c [DHResultLen]byte) *sessionKeys {
	ikm := make([]byte, 0, 3*DHResultLen)
	ikm = append(ikm, a[:]...)
	ikm = append(ikm, b[:]...)
	ikm = append(ikm, c[:]...)

	h := hkdf.New(sha256.New, ikm, []byte(HKDFSalt), []byte("devhive-session/v1"))
	buf := make([]byte, 2*chacha20poly1305.KeySize)
	if _, err := h.Read(buf); err != nil {
		panic("hkdf expand failed: " + err.Error())
	}
	var k sessionKeys
	copy(k.k12[:], buf[:chacha20poly1305.KeySize])
	copy(k.k21[:], buf[chacha20poly1305.KeySize:])
	return &k
}

func newSession(conn net.Conn, initiator bool, keys *sessionKeys) (*Session, error) {
	s := &Session{conn: conn, initiator: initiator}
	var kA, kB [chacha20poly1305.KeySize]byte
	copy(kA[:], keys.k12[:])
	copy(kB[:], keys.k21[:])
	if !initiator {
		kA, kB = kB, kA
	}
	var err error
	if s.sendAEAD, err = chacha20poly1305.NewX(kA[:]); err != nil {
		return nil, err
	}
	if s.recvAEAD, err = chacha20poly1305.NewX(kB[:]); err != nil {
		return nil, err
	}
	return s, nil
}

func frameNonce(seq uint64) []byte {
	nonce := make([]byte, chacha20poly1305.NonceSizeX) // 24 zero bytes
	binary.BigEndian.PutUint64(nonce[16:], seq)
	return nonce
}

func overheadLen() int { return tagOverhad }

// Write encrypts and frames (msgType, payload).
func (s *Session) Write(msgType byte, payload []byte) error {
	if s.sendSeq == (1<<63)-1 {
		return fmt.Errorf("%w: counter exhausted", ErrCorrupt)
	}
	s.sendSeq++
	maxLen := maxFrameSize - overheadLen()
	if len(payload) > maxLen {
		return fmt.Errorf("payload too large (%d > %d)", len(payload), maxLen)
	}
	var hdr [headerLen]byte
	hdr[0] = frameVersion
	hdr[1] = msgType
	binary.BigEndian.PutUint64(hdr[2:10], s.sendSeq)
	binary.BigEndian.PutUint32(hdr[10:14], uint32(len(payload))+uint32(tagOverhad))
	nonce := frameNonce(s.sendSeq)
	ct := s.sendAEAD.Seal(nil, nonce, payload, hdr[:])
	out := make([]byte, 0, headerLen+len(ct))
	out = append(out, hdr[:]...)
	out = append(out, ct...)
	_, err := s.conn.Write(out)
	return err
}

var errClosedLegacy = map[string]bool{"EOF": true, "connection reset by peer": true, "broken pipe": true}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrSessionClosed) || errors.Is(err, ErrCorrupt) {
		return err
	}
	if errors.Is(err, net.ErrClosed) {
		return ErrSessionClosed
	}
	if err.Error() == "EOF" || errClosedLegacy[err.Error()] {
		return ErrSessionClosed
	}
	return err
}

// Read decrypts and returns the next (msgType, payload).
func (s *Session) Read() (byte, []byte, error) {
	hdr := make([]byte, headerLen)
	if _, err := io.ReadFull(s.conn, hdr); err != nil {
		return 0, nil, mapErr(err)
	}
	if hdr[0] != frameVersion {
		return 0, nil, fmt.Errorf("%w: unsupported version %d", ErrCorrupt, hdr[0])
	}
	msgType := hdr[1]
	ctr := binary.BigEndian.Uint64(hdr[2:10])
	if ctr == 0 || ctr <= s.recvHigh {
		// desync/tamper: kill the socket so the peer's writer never blocks
		_ = s.conn.Close()
		return 0, nil, fmt.Errorf("%w: replayed or desynced frame", ErrCorrupt)
	}
	l := binary.BigEndian.Uint32(hdr[10:14])
	if l < uint32(tagOverhad) || l > uint32(maxFrameSize) {
		_ = s.conn.Close()
		return 0, nil, fmt.Errorf("%w: bad length %d", ErrCorrupt, l)
	}
	ct := make([]byte, l)
	if _, err := io.ReadFull(s.conn, ct); err != nil {
		return 0, nil, mapErr(err)
	}
	nonce := frameNonce(ctr)
	pt, err := s.recvAEAD.Open(nil, nonce, ct, hdr[:])
	if err != nil {
		_ = s.conn.Close()
		return 0, nil, fmt.Errorf("%w: aead rejected frame", ErrCorrupt)
	}
	s.recvHigh = ctr
	return msgType, pt, nil
}
