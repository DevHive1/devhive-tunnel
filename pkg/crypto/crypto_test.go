package crypto

import (
	"crypto/ed25519"
	"errors"
	"net"
	"testing"
	"time"
)

func mustIdentity(t *testing.T) *Identity {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	return id
}

func pipeWithDeadlines(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	_ = a.SetDeadline(time.Now().Add(10 * time.Second))
	_ = b.SetDeadline(time.Now().Add(10 * time.Second))
	return a, b
}

func TestIdentitySeedRoundtrip(t *testing.T) {
	id := mustIdentity(t)
	id2, err := LoadSeedHexText(id.SaveSeedHexText())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	x1, x2 := id2.XPub(), id.XPub()
	if string(x1[:]) != string(x2[:]) {
		t.Fatal("xpub mismatch after roundtrip")
	}
	if string(id2.SignPub()) != string(id.SignPub()) {
		t.Fatal("signpub mismatch after roundtrip")
	}
	if id.Fingerprint() != id2.Fingerprint() {
		t.Fatal("fingerprint mismatch after roundtrip")
	}
}

func TestSignVerify(t *testing.T) {
	id := mustIdentity(t)
	sig := id.Sign([]byte("body"))
	if !ed25519.Verify(id.SignPub(), []byte("body"), sig) {
		t.Fatal("verify of valid signature failed")
	}
	if ed25519.Verify(id.SignPub(), []byte("bodY"), sig) {
		t.Fatal("tampered message verified")
	}
}

func TestDeriveNetworkPSK(t *testing.T) {
	if DeriveNetworkPSK("dh-X", "pw") != DeriveNetworkPSK("dh-X", "pw") {
		t.Fatal("kdf not deterministic")
	}
	if DeriveNetworkPSK("dh-X", "pw") == DeriveNetworkPSK("dh-X", "other") {
		t.Fatal("password ignored by kdf")
	}
	if DeriveNetworkPSK("dh-X", "pw") == DeriveNetworkPSK("dh-Y", "pw") {
		t.Fatal("network id ignored by kdf")
	}
}

func TestAdminKeys(t *testing.T) {
	k := NewAdminKey()
	if len(k) == 0 {
		t.Fatal("empty admin key")
	}
	h := HashAdminKey(k)
	if !CheckAdminKey(k, h) || CheckAdminKey(k+"x", h) {
		t.Fatal("admin key check broken")
	}
	id := NewNetworkID()
	if len(id) < 6 || id[:2] != IDPrefix {
		t.Fatalf("bad network id %q body: "+id, id)
	}
}

// byteFliper wraps a conn and flips one byte of one chosen write.
type byteFliper struct {
	inner   net.Conn
	at      int // 1-based index of write to corrupt
	offset  int // offset from END of buffer
	written int
}

func (c *byteFliper) Write(b []byte) (int, error) {
	c.written++
	if c.written == c.at && len(b) > c.offset {
		z := append([]byte(nil), b...)
		z[len(z)-c.offset] ^= 0x40
		return c.inner.Write(z)
	}
	return c.inner.Write(b)
}
func (c *byteFliper) Read(b []byte) (int, error)          { return c.inner.Read(b) }
func (c *byteFliper) Close() error                       { return c.inner.Close() }
func (c *byteFliper) LocalAddr() net.Addr                { return c.inner.LocalAddr() }
func (c *byteFliper) RemoteAddr() net.Addr               { return c.inner.RemoteAddr() }
func (c *byteFliper) SetDeadline(t time.Time) error      { return c.inner.SetDeadline(t) }
func (c *byteFliper) SetReadDeadline(t time.Time) error  { return c.inner.SetReadDeadline(t) }
func (c *byteFliper) SetWriteDeadline(t time.Time) error { return c.inner.SetWriteDeadline(t) }

// replayConn stores a snapshot of the last write, can replay it verbatim.
type replayConn struct {
	inner net.Conn
	last  []byte
}

func (c *replayConn) Write(b []byte) (int, error) {
	c.last = append(c.last[:0], b...)
	return c.inner.Write(b)
}
func (c *replayConn) Read(b []byte) (int, error)          { return c.inner.Read(b) }
func (c *replayConn) Close() error                       { return c.inner.Close() }
func (c *replayConn) LocalAddr() net.Addr                { return c.inner.LocalAddr() }
func (c *replayConn) RemoteAddr() net.Addr               { return c.inner.RemoteAddr() }
func (c *replayConn) SetDeadline(t time.Time) error      { return c.inner.SetDeadline(t) }
func (c *replayConn) SetReadDeadline(t time.Time) error  { return c.inner.SetReadDeadline(t) }
func (c *replayConn) SetWriteDeadline(t time.Time) error { return c.inner.SetWriteDeadline(t) }

func handshakePair(t *testing.T) (*Session, *Session, *Identity, *Identity) {
	t.Helper()
	ia, ib := mustIdentity(t), mustIdentity(t)
	aC, bC := pipeWithDeadlines(t)
	var sB *Session
	errB := make(chan error, 1)
	go func() {
		s, err := HandshakeResponder(bC, ib, []byte("resp-blob"))
		if err != nil {
			errB <- err
			return
		}
		sB = s
		close(errB)
	}()
	sA, err := HandshakeInitiator(aC, ia, nil, []byte("init-blob"))
	if err != nil {
		t.Fatalf("initiator handshake: %v", err)
	}
	if err := <-errB; err != nil {
		t.Fatalf("responder handshake: %v", err)
	}
	return sA, sB, ia, ib
}

func TestHandshakeAndBlobExchange(t *testing.T) {
	a, b, _, ib := handshakePair(t)
	defer a.Close()
	defer b.Close()
	if string(a.ReceivedBlob()) != "resp-blob" || string(b.ReceivedBlob()) != "init-blob" {
		t.Fatalf("blob exchange broken: got %q / %q", a.ReceivedBlob(), b.ReceivedBlob())
	}
	if a.IsInitiator() != true || b.IsInitiator() != false {
		t.Fatal("roles wrong")
	}
	if string(a.RemoteEd()) != string(ib.SignPub()) {
		t.Fatal("remote ed mismatch")
	}
}

func TestSessionFramesBothDirections(t *testing.T) {
	a, b, _, _ := handshakePair(t)
	defer a.Close()
	defer b.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 4; i++ {
			rt, rp, err := b.Read()
			if err != nil || rt != 5 {
				t.Errorf("responder read %d: %v", i, err)
				return
			}
			if string(rp) != "ping"+string(rune('0'+i)) {
				t.Errorf("payload mismatch %q", rp)
				return
			}
			if err := b.Write(6, []byte("echo-back")); err != nil {
				t.Errorf("responder write: %v", err)
				return
			}
		}
	}()
	for i := 0; i < 4; i++ {
		if err := a.Write(5, []byte("ping"+string(rune('0'+i)))); err != nil {
			t.Fatalf("initiator write: %v", err)
		}
		rt, rp, err := a.Read()
		if err != nil || rt != 6 || string(rp) != "echo-back" {
			t.Fatalf("initiator read: type=%d payload=%q err=%v", rt, rp, err)
		}
	}
	<-done
}

func TestSessionLargePayload(t *testing.T) {
	a, b, _, _ := handshakePair(t)
	defer a.Close()
	defer b.Close()
	big := make([]byte, 64<<10)
	for i := range big {
		big[i] = byte(i)
	}
	errC := make(chan error, 1)
	go func() {
		t2, p, err := b.Read()
		if err == nil && (t2 != 9 || len(p) != len(big)) {
			err = errors.New("large payload mismatch")
		}
		errC <- err
	}()
	if err := a.Write(9, big); err != nil {
		t.Fatalf("write large: %v", err)
	}
	if err := <-errC; err != nil {
		t.Fatalf("read large: %v", err)
	}
}

func TestSessionRejectsOversize(t *testing.T) {
	a, b, _, _ := handshakePair(t)
	defer a.Close()
	defer b.Close()
	if err := a.Write(3, make([]byte, maxFrameSize)); err == nil {
		t.Fatal("oversize write accepted")
	}
}

func TestTamperedFrameRejected(t *testing.T) {
	// roles: A tampers its FIRST data frame (write #3: len, msg1, frame)
	ia, ib := mustIdentity(t), mustIdentity(t)
	aC, bC := pipeWithDeadlines(t)
	dst := &byteFliper{inner: bC}
	// responder reads whatever arrives
	errB := make(chan error, 2)
	go func() {
		s, err := HandshakeResponder(dst, ib, nil)
		if err != nil {
			errB <- err
			return
		}
		_, _, err = s.Read()
		errB <- err
	}()
	aC2 := &replayConn{inner: aC}
	// initiator sends 1 handshake (2 writes) + 1 tamper-free frame; the tamper is
	// applied by dst wrapper -- flip a byte in write#1 of b? no: dst flips WRITES
	// it makes to initiator. Responder only writes handshake msg2 then nothing,
	// so instead test tampering the incoming handshake by flipping initiator's
	// msg2 read? Signature covers msg1; a flipped byte inside any write to b
	// corrupts msg1. We tamper A's msg1 (write #2 on aC2) instead:
	aC2WriteTamper := &byteFliper{inner: aC2, at: 2, offset: 40}
	sA, err := HandshakeInitiator(aC2WriteTamper, ia, nil, nil)
	if err == nil {
		sA.Close()
		t.Fatal("handshake accepted with corrupted msg1")
	}
	t.Logf("responder side: %v", <-errB)
}

func TestReplayRejected(t *testing.T) {
	ia, ib := mustIdentity(t), mustIdentity(t)
	aC, bC := pipeWithDeadlines(t)
	var sB *Session
	errB := make(chan error, 1)
	go func() {
		s, err := HandshakeResponder(bC, ib, nil)
		if err == nil {
			sB = s
		}
		errB <- err
	}()
	rc := &replayConn{inner: aC}
	sA, err := HandshakeInitiator(rc, ia, nil, nil)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if err := <-errB; err != nil {
		t.Fatalf("responder: %v", err)
	}
	defer sA.Close()
	defer sB.Close()

	readOne := make(chan error, 1)
	go func() {
		_, _, err := sB.Read()
		readOne <- err
	}()
	if err := sA.Write(4, []byte("first")); err != nil {
		t.Fatalf("write1: %v", err)
	}
	if err := <-readOne; err != nil {
		t.Fatalf("read1: %v", err)
	}
	// replay the exact previous frame bytes verbatim
	errC := make(chan error, 1)
	go func() {
		_, _, err := sB.Read()
		errC <- err
	}()
	_, werr := rc.inner.Write(append([]byte(nil), rc.last...))
	err = <-errC
	// detection may show up either as Read-side ErrCorrupt, or as an
	// immediate socket close on the writing side. Both reject the replay.
	if err == nil && werr == nil {
		t.Fatal("replayed frame accepted")
	}
	if err != nil && !errors.Is(err, ErrCorrupt) && werr == nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandshakeIdentityPin(t *testing.T) {
	ia, ib := mustIdentity(t), mustIdentity(t)
	ic := mustIdentity(t) // wrong pin
	aC, bC := pipeWithDeadlines(t)
	go func() {
		_, _ = HandshakeResponder(bC, ib, nil)
	}()
	_, err := HandshakeInitiator(aC, ia, ic.SignPub(), nil)
	if err == nil {
		t.Fatal("wrong pinned identity accepted")
	}
}
