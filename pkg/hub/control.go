package hub

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"devhive-tunnel/pkg/crypto"
)

// hubBlob is bound into every handshake with this hub.
var hubBlob = []byte("devhive-hub/signaling/v1")

// Serve runs the control TCP listener until a fatal error.
func (s *Server) Serve() error {
	ln, err := net.Listen("tcp", s.opts.Listen)
	if err != nil {
		return err
	}
	log.Printf("[hub] control listening on %s", s.opts.Listen)

	go func() {
		tk := time.NewTicker(time.Minute)
		for range tk.C {
			s.sweepJoins()
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.serveConn(conn)
	}
}

func (s *Server) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	sess, err := crypto.HandshakeResponder(conn, s.id, hubBlob)
	if err != nil {
		log.Printf("[hub] handshake from %s failed: %v", conn.RemoteAddr(), err)
		return
	}
	fp := crypto.FingerprintOfPub(sess.RemoteEd())
	lk := &link{sess: sess, fp: fp}
	defer s.dropLink(sess, lk)

	for {
		_ = sess.RawConn().SetReadDeadline(time.Now().Add(30 * time.Minute))
		tp, payload, err := sess.Read()
		if err != nil {
			return
		}
		switch tp {
		case crypto.TypePing:
			continue
		case crypto.TypeBye:
			return
		}
		if len(payload) > 1<<20 {
			_ = s.reply(sess, byte(WSysErr), sysErr{Code: "too_big"})
			return
		}
		if err := s.dispatch(sess, lk, tp, payload); err != nil {
			log.Printf("[hub] %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

type sysErr struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func (s *Server) reply(sess *crypto.Session, t byte, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_ = sess.RawConn().SetWriteDeadline(time.Now().Add(15 * time.Second))
	return sess.Write(t, b)
}

func (s *Server) dispatch(sess *crypto.Session, lk *link, tp byte, payload []byte) error {
	switch WireType(tp) {
	case WCreate:
		var req CreateReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return s.reply(sess, byte(WSysErr), sysErr{Code: "bad_json"})
		}
		return s.handleCreate(sess, lk, req)
	case WJoin:
		var req JoinReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return s.reply(sess, byte(WSysErr), sysErr{Code: "bad_json"})
		}
		return s.handleJoin(sess, lk, req)
	case WList:
		var req ListReq
		_ = json.Unmarshal(payload, &req)
		return s.handleList(sess, req)
	case WRenew:
		if lk.netID == "" {
			return s.reply(sess, byte(WSysErr), sysErr{Code: "not_joined"})
		}
		return s.handleRenew(sess, lk)
	case WInvite:
		var req InviteReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return s.reply(sess, byte(WSysErr), sysErr{Code: "bad_json"})
		}
		if lk.netID == "" {
			return s.reply(sess, byte(WSysErr), sysErr{Code: "not_joined"})
		}
		return s.handleInvite(sess, lk, req)
	case WRoute:
		var env RouteEnvelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return s.reply(sess, byte(WSysErr), sysErr{Code: "bad_json"})
		}
		if lk.netID == "" {
			return s.reply(sess, byte(WSysErr), sysErr{Code: "not_joined"})
		}
		return s.handleRoute(sess, lk, env)
	default:
		return s.reply(sess, byte(WSysErr), sysErr{Code: "unknown_type"})
	}
}
