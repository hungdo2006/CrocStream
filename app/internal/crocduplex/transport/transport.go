// Package transport implements the experimental G0 contract, not croc CLI media.
package transport

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/schollz/croc/v11/src/codephrase"
	"github.com/schollz/croc/v11/src/crypt"
	"github.com/schollz/croc/v11/src/tcp"
	"golang.org/x/crypto/hkdf"

	"crocstream.local/app/internal/crocduplex/upstreampake"
)

const (
	maxData      = 65536
	maxRecord    = 65582
	maxHandshake = 8192
	maxRecords   = 1 << 20
	dataKind     = 1
	closeKind    = 2

	RouteRelay = "relay"
	RouteLAN   = "lan"
)

var ErrProtocol = errors.New("invalid G0 transport message")

type Options struct {
	RelayAddress  string
	RelayPassword string
	// EnableLAN enables croc-style automatic LAN discovery with relay fallback.
	EnableLAN bool
	// LANDiscoveryPort overrides DefaultLANDiscoveryPort (9019) when > 0.
	LANDiscoveryPort int
	// LANDiscoverTimeout overrides the LAN discovery probe window when > 0.
	LANDiscoverTimeout time.Duration
	// Zero uses the defaults. Overrides are for bounded tests/operators.
	HandshakeTimeout time.Duration
	WaitTimeout      time.Duration
	IOTimeout        time.Duration

	// Test-only overrides to keep LAN unit tests deterministic and isolated.
	lanListenAddr   string
	lanUDPAddr      string
	lanProbeTargets []*net.UDPAddr
}

func (o Options) lanDiscoveryPort() int {
	if o.LANDiscoveryPort > 0 {
		return o.LANDiscoveryPort
	}
	return DefaultLANDiscoveryPort
}

func (o Options) lanUDPListenAddr() string {
	if o.lanUDPAddr != "" {
		return o.lanUDPAddr
	}
	return fmt.Sprintf(":%d", o.lanDiscoveryPort())
}

func (o Options) validate() (Options, error) {
	if o.RelayAddress == "" || o.RelayPassword == "" || len(o.RelayPassword) > 1024 {
		return o, errors.New("relay address and bounded password required")
	}
	if o.HandshakeTimeout == 0 {
		o.HandshakeTimeout = 30 * time.Second
	}
	if o.WaitTimeout == 0 {
		o.WaitTimeout = 5 * time.Minute
	}
	if o.IOTimeout == 0 {
		o.IOTimeout = 30 * time.Second
	}
	if o.HandshakeTimeout <= 0 || o.HandshakeTimeout > 30*time.Second ||
		o.WaitTimeout <= 0 || o.WaitTimeout > 5*time.Minute ||
		o.IOTimeout <= 0 || o.IOTimeout > 5*time.Minute {
		return o, errors.New("timeouts outside G0 bounds")
	}
	return o, nil
}

type owner struct {
	conn   net.Conn
	cancel context.CancelFunc
	stop   func() bool
	once   sync.Once
	closed atomic.Bool
}

func (o *owner) abort() {
	o.once.Do(func() {
		o.closed.Store(true)
		o.stop()
		o.cancel()
		_ = o.conn.Close()
	})
}

func dial(ctx context.Context, o Options, room string) (*owner, context.Context, error) {
	if ctx == nil {
		return nil, nil, errors.New("context required")
	}
	life, cancel := context.WithCancel(ctx)
	admission, done := context.WithTimeout(life, o.HandshakeTimeout)
	c, _, _, _, err := tcp.ConnectToTCPServerControlContext(admission, o.RelayAddress, o.RelayPassword, room, o.HandshakeTimeout)
	done()
	if err != nil {
		cancel()
		// Upstream errors can contain server-supplied text. Keep diagnostics bounded.
		return nil, nil, errors.New("relay admission failed or canceled")
	}
	raw := c.Connection()
	if err := raw.SetDeadline(time.Time{}); err != nil {
		c.Close()
		cancel()
		return nil, nil, err
	}
	own := &owner{conn: raw, cancel: cancel}
	// Callback only closes raw I/O; do not call abort until stop is assigned.
	own.stop = context.AfterFunc(life, func() { own.closed.Store(true); _ = raw.Close() })
	if err := life.Err(); err != nil {
		own.abort()
		return nil, nil, err
	}
	return own, life, nil
}

type Listener struct {
	own                    *owner
	lan                    *lanAcceptor
	ctx                    context.Context
	cancel                 context.CancelFunc
	relayCtx               context.Context
	opts                   Options
	mu                     sync.Mutex
	invite, password, room string
	accepted               bool
}

func Host(ctx context.Context, opts Options) (*Listener, error) {
	if ctx == nil {
		return nil, errors.New("context required")
	}
	opts, err := opts.validate()
	if err != nil {
		return nil, err
	}
	code, err := codephrase.GenerateSSH()
	if err != nil {
		return nil, err
	}
	invite := "cs1-" + code
	password, room, err := parseInvite(invite)
	if err != nil {
		return nil, err
	}
	hostCtx, hostCancel := context.WithCancel(ctx)
	own, relayLife, err := dial(hostCtx, opts, room)
	if err != nil {
		hostCancel()
		return nil, err
	}
	var lan *lanAcceptor
	if opts.EnableLAN {
		lan = startLANAcceptor(room, opts)
	}
	// Relay admission is complete. Only handshake/record timeouts are needed
	// afterward; do not retain the relay password in the one-shot listener.
	opts.RelayPassword = ""
	return &Listener{
		own:      own,
		lan:      lan,
		ctx:      hostCtx,
		cancel:   hostCancel,
		relayCtx: relayLife,
		opts:     opts,
		invite:   invite,
		password: password,
		room:     room,
	}, nil
}

func (l *Listener) Invitation() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.own.closed.Load() || l.ctx.Err() != nil {
		return ""
	}
	return l.invite
}

func (l *Listener) Accept(ctx context.Context) (*Session, error) {
	return l.AcceptWithProgress(ctx, nil)
}

// AcceptWithProgress preserves Accept semantics and reports only after a
// relay-admitted or LAN-admitted peer has supplied a structurally valid CrocStream PAKE INIT.
func (l *Listener) AcceptWithProgress(ctx context.Context, authenticating func()) (*Session, error) {
	if ctx == nil {
		return nil, errors.New("context required")
	}
	l.mu.Lock()
	if l.accepted {
		l.mu.Unlock()
		return nil, errors.New("Accept is one-shot")
	}
	l.accepted = true
	password, room := l.password, l.room
	l.password = ""
	l.mu.Unlock()

	if l.lan == nil {
		stop := context.AfterFunc(ctx, func() { l.own.cancel() })
		if ctx.Err() != nil {
			stop()
			l.own.abort()
			return nil, ctx.Err()
		}
		s, err := handshake(l.relayCtx, l.own, l.opts, password, room, 1, authenticating)
		if err != nil {
			stop()
			l.Close()
			return nil, err
		}
		s.route = RouteRelay
		s.stopAccept = stop
		return s, nil
	}

	return l.acceptRace(ctx, password, room, authenticating)
}

func (l *Listener) acceptRace(ctx context.Context, password, room string, authenticating func()) (*Session, error) {
	raceCtx, cancelRace := context.WithCancel(l.ctx)
	stopParent := context.AfterFunc(ctx, cancelRace)
	if ctx.Err() != nil {
		stopParent()
		cancelRace()
		l.Close()
		return nil, ctx.Err()
	}

	var authOnce sync.Once
	notifyAuth := func() {
		if authenticating != nil {
			authOnce.Do(authenticating)
		}
	}

	type outcome struct {
		route string
		s     *Session
		err   error
	}
	results := make(chan outcome, 2)

	// Path 1: Relay handshake (cancels l.own only if relay loses or race cancels).
	relayCtx, cancelRelay := context.WithCancel(raceCtx)
	stopRelay := context.AfterFunc(relayCtx, func() { l.own.abort() })
	go func() {
		s, err := handshake(l.relayCtx, l.own, l.opts, password, room, 1, notifyAuth)
		if err == nil {
			s.route = RouteRelay
		}
		results <- outcome{route: RouteRelay, s: s, err: err}
	}()

	// Path 2: Local LAN direct TCP accept + identical PAKE/AEAD handshake.
	lanCtx, cancelLAN := context.WithCancel(raceCtx)
	go func() {
		raw, err := l.lan.acceptAdmitted(lanCtx)
		if err != nil {
			results <- outcome{route: RouteLAN, err: err}
			return
		}
		// Tie the LAN connection lifetime to l.ctx (independent of l.relayCtx)
		// so aborting the losing relay does not tear down the winning LAN session.
		lanLife, lanCancel := context.WithCancel(l.ctx)
		lanOwn := &owner{conn: raw, cancel: lanCancel}
		lanOwn.stop = context.AfterFunc(lanLife, func() { lanOwn.closed.Store(true); _ = raw.Close() })
		stopLANCancel := context.AfterFunc(lanCtx, func() { lanOwn.abort() })
		s, err := handshake(lanLife, lanOwn, l.opts, password, room, 1, notifyAuth)
		stopLANCancel()
		if err != nil {
			lanOwn.abort()
			results <- outcome{route: RouteLAN, err: err}
			return
		}
		s.route = RouteLAN
		results <- outcome{route: RouteLAN, s: s}
	}()

	first := <-results
	if first.err == nil {
		// Winner succeeded; immediately cancel and drain the losing path.
		stopParent()
		if first.route == RouteLAN {
			cancelRelay()
			second := <-results
			if second.s != nil {
				second.s.Close()
			}
			stopRelay()
			l.lan.close()
			cancelLAN()
			cancelRace()
			first.s.stopAccept = context.AfterFunc(ctx, func() { first.s.own.abort() })
			if ctx.Err() != nil {
				first.s.Close()
				return nil, ctx.Err()
			}
			return first.s, nil
		}
		// Relay won: detach relay from cancelRelay before closing raceCtx/LAN.
		stopRelay()
		cancelLAN()
		l.lan.close()
		second := <-results
		if second.s != nil {
			second.s.Close()
		}
		cancelRelay()
		cancelRace()
		first.s.stopAccept = context.AfterFunc(ctx, func() { first.s.own.abort() })
		if ctx.Err() != nil {
			first.s.Close()
			return nil, ctx.Err()
		}
		return first.s, nil
	}

	// First path failed. If LAN merely stopped due to local error (before peer
	// authentication) and raceCtx is still active, keep waiting on Relay.
	// If Relay failed or a peer failed PAKE confirmation, abort both paths.
	if first.route == RouteLAN && raceCtx.Err() == nil && !errors.Is(first.err, ErrProtocol) &&
		first.err.Error() != "peer confirmation failed" && first.err.Error() != "PAKE update failed" {
		second := <-results
		stopParent()
		stopRelay()
		cancelLAN()
		l.lan.close()
		cancelRelay()
		cancelRace()
		if second.err == nil {
			second.s.stopAccept = context.AfterFunc(ctx, func() { second.s.own.abort() })
			if ctx.Err() != nil {
				second.s.Close()
				return nil, ctx.Err()
			}
			return second.s, nil
		}
		l.Close()
		return nil, second.err
	}

	// Abort remaining path and wait for clean exit.
	cancelRelay()
	cancelLAN()
	l.lan.close()
	second := <-results
	if second.s != nil {
		second.s.Close()
	}
	stopParent()
	stopRelay()
	cancelRace()
	l.Close()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, first.err
}

func (l *Listener) Close() error {
	l.mu.Lock()
	l.invite, l.password, l.room = "", "", ""
	l.mu.Unlock()
	l.lan.close()
	l.own.abort()
	if l.cancel != nil {
		l.cancel()
	}
	return nil
}

func Join(ctx context.Context, invite string, opts Options) (*Session, error) {
	return JoinWithProgress(ctx, invite, opts, nil)
}

// JoinWithProgress prefers an authenticated LAN peer when EnableLAN is true,
// falling back automatically to the configured relay.
func JoinWithProgress(ctx context.Context, invite string, opts Options, authenticating func()) (*Session, error) {
	opts, err := opts.validate()
	if err != nil {
		return nil, err
	}
	password, room, err := parseInvite(invite)
	if err != nil {
		return nil, err
	}
	if opts.EnableLAN {
		tag := deriveLANTag(room)
		if addr, ok := discoverLAN(ctx, tag, opts); ok {
			if lanOwn, lanLife, dialErr := dialLAN(ctx, addr, tag); dialErr == nil {
				lanOpts := opts
				lanOpts.RelayPassword = ""
				s, hsErr := handshake(lanLife, lanOwn, lanOpts, password, room, 0, authenticating)
				if hsErr == nil {
					s.route = RouteLAN
					return s, nil
				}
				lanOwn.abort()
				// LAN reached the exact room tag but PAKE failed (e.g. wrong code or cancel);
				// do not retry the same bad credentials on the relay.
				return nil, hsErr
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	own, life, err := dial(ctx, opts, room)
	if err != nil {
		return nil, err
	}
	opts.RelayPassword = ""
	s, err := handshake(life, own, opts, password, room, 0, authenticating)
	if err != nil {
		own.abort()
		return nil, err
	}
	s.route = RouteRelay
	return s, nil
}

func parseInvite(invite string) (string, string, error) {
	if len(invite) > 256 || !strings.HasPrefix(invite, "cs1-") {
		return "", "", errors.New("invalid invitation")
	}
	c, err := codephrase.ParseSSH(strings.TrimPrefix(invite, "cs1-"))
	if err != nil {
		return "", "", errors.New("invalid invitation")
	}
	sum := sha256.Sum256([]byte("crocstream/room/g0-v1\x00" + c.RoomName))
	return c.PAKEPassphrase, fmt.Sprintf("%x", sum), nil
}

// No Comm.Read after relay admission: it resets both connection deadlines.
func readFrame(c net.Conn, deadline time.Time, max int) ([]byte, error) {
	if err := c.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	var header [8]byte
	if _, err := io.ReadFull(c, header[:]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(header[4:])
	if string(header[:4]) != "croc" || size > uint32(max) {
		return nil, ErrProtocol
	}
	payload := make([]byte, int(size))
	_, err := io.ReadFull(c, payload)
	return payload, err
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) != 0 {
		n, err := w.Write(p)
		if n < 0 || n > len(p) {
			return io.ErrShortWrite
		}
		p = p[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func writeFrame(c net.Conn, deadline time.Time, p []byte) error {
	if err := c.SetWriteDeadline(deadline); err != nil {
		return err
	}
	var h [8]byte
	copy(h[:4], "croc")
	binary.LittleEndian.PutUint32(h[4:], uint32(len(p)))
	if err := writeAll(c, h[:]); err != nil {
		return err
	}
	return writeAll(c, p)
}

func sendHS(c net.Conn, d time.Time, kind byte, body []byte) error {
	if len(body)+5 > maxHandshake {
		return ErrProtocol
	}
	p := append([]byte("CSH1"), kind)
	p = append(p, body...)
	return writeFrame(c, d, p)
}

func recvHS(c net.Conn, d time.Time, kind byte, keepalive bool) ([]byte, error) {
	for {
		p, err := readFrame(c, d, maxHandshake)
		if err != nil {
			return nil, err
		}
		if keepalive && len(p) == 1 && p[0] == 1 {
			continue
		}
		if len(p) < 5 || string(p[:4]) != "CSH1" || p[4] != kind {
			return nil, ErrProtocol
		}
		return p[5:], nil
	}
}

func packPAKE(value, salt []byte) ([]byte, error) {
	if len(value) == 0 || len(value) > 4096 {
		return nil, ErrProtocol
	}
	b := make([]byte, 2, 2+len(value)+len(salt))
	binary.BigEndian.PutUint16(b, uint16(len(value)))
	b = append(b, value...)
	return append(b, salt...), nil
}

func unpackPAKE(b []byte, saltSize int) ([]byte, []byte, error) {
	if len(b) < 2 {
		return nil, nil, ErrProtocol
	}
	n := int(binary.BigEndian.Uint16(b))
	if n == 0 || n > 4096 || len(b) != 2+n+saltSize {
		return nil, nil, ErrProtocol
	}
	return b[2 : 2+n], b[2+n:], nil
}

func handshake(ctx context.Context, own *owner, opts Options, password, room string, role int, authenticating func()) (*Session, error) {
	c := own.conn
	d := time.Now().Add(opts.HandshakeTimeout)
	var a, b, salt []byte
	p, err := upstreampake.Init([]byte(password), role, room)
	if err != nil {
		return nil, errors.New("PAKE initialization failed")
	}
	if role == 0 {
		if authenticating != nil {
			authenticating()
		}
		a = append([]byte(nil), p.Bytes()...)
		body, err := packPAKE(a, nil)
		if err != nil {
			return nil, err
		}
		if err := sendHS(c, d, 1, body); err != nil {
			return nil, err
		}
		body, err = recvHS(c, d, 2, true)
		if err != nil {
			return nil, err
		}
		b, salt, err = unpackPAKE(body, 32)
		if err != nil {
			return nil, err
		}
		if err := p.Update(b); err != nil {
			return nil, errors.New("PAKE update failed")
		}
	} else {
		body, err := recvHS(c, time.Now().Add(opts.WaitTimeout), 1, true)
		if err != nil {
			return nil, err
		}
		d = time.Now().Add(opts.HandshakeTimeout)
		a, _, err = unpackPAKE(body, 0)
		if err != nil {
			return nil, err
		}
		if authenticating != nil {
			authenticating()
		}
		if err := p.Update(a); err != nil {
			return nil, errors.New("PAKE update failed")
		}
		b = append([]byte(nil), p.Bytes()...)
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, err
		}
		body, err = packPAKE(b, salt)
		if err != nil {
			return nil, err
		}
		if err := sendHS(c, d, 2, body); err != nil {
			return nil, err
		}
	}
	shared, err := p.SessionKey()
	if err != nil {
		return nil, errors.New("PAKE session key failed")
	}
	keys, err := upstreampake.Derive(shared, upstreampake.Context{Initiator: a, Responder: b, Salt: salt, Room: room})
	clear(shared)
	if err != nil {
		return nil, err
	}
	defer clear(keys.EncryptionKey)
	defer clear(keys.ConfirmationA)
	defer clear(keys.ConfirmationB)
	if role == 0 {
		if err := sendHS(c, d, 3, keys.ConfirmationA); err != nil {
			return nil, err
		}
		tag, err := recvHS(c, d, 4, false)
		if err != nil {
			return nil, err
		}
		if !upstreampake.Confirm(keys.ConfirmationB, tag) {
			return nil, errors.New("peer confirmation failed")
		}
	} else {
		tag, err := recvHS(c, d, 3, false)
		if err != nil {
			return nil, err
		}
		if !upstreampake.Confirm(keys.ConfirmationA, tag) {
			return nil, errors.New("peer confirmation failed")
		}
		if err := sendHS(c, d, 4, keys.ConfirmationB); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return newSession(own, keys.EncryptionKey, byte(role), opts.IOTimeout)
}

func directional(key []byte, direction byte) (cipher.AEAD, error) {
	label := "crocstream/record/g0-v1/viewer-to-broadcaster"
	if direction == 1 {
		label = "crocstream/record/g0-v1/broadcaster-to-viewer"
	}
	k := make([]byte, 32)
	defer clear(k)
	if _, err := io.ReadFull(hkdf.New(sha256.New, key, nil, []byte(label)), k); err != nil {
		return nil, err
	}
	return crypt.NewAESGCM(k)
}

type Session struct {
	own             *owner
	send, recv      cipher.AEAD
	direction       byte
	route           string
	readMu, writeMu sync.Mutex
	tx, rx          uint64
	pending         []byte
	eof             bool
	timeout         time.Duration
	stopAccept      func() bool
}

// Route returns the actually established transport route ("lan" or "relay").
func (s *Session) Route() string { return s.route }

func newSession(own *owner, key []byte, dir byte, timeout time.Duration) (*Session, error) {
	send, err := directional(key, dir)
	if err != nil {
		return nil, err
	}
	recv, err := directional(key, dir^1)
	if err != nil {
		return nil, err
	}
	return &Session{own: own, send: send, recv: recv, direction: dir, timeout: timeout}, nil
}

func seal(aead cipher.AEAD, dir, kind byte, seq uint64, p []byte) ([]byte, error) {
	if seq >= maxRecords || len(p) > maxData ||
		(kind == dataKind && (len(p) == 0 || seq >= maxRecords-1)) ||
		(kind == closeKind && len(p) != 0) ||
		(kind != dataKind && kind != closeKind) {
		return nil, ErrProtocol
	}
	plain := make([]byte, 18+len(p))
	defer clear(plain)
	copy(plain, "CSR1")
	plain[4], plain[5] = dir, kind
	binary.BigEndian.PutUint64(plain[6:14], seq)
	binary.BigEndian.PutUint32(plain[14:18], uint32(len(p)))
	copy(plain[18:], p)
	// Upstream format includes a fresh RANDOM nonce, ciphertext and tag.
	return crypt.EncryptAEADTo(nil, plain, aead)
}

func open(aead cipher.AEAD, dir byte, seq uint64, payload []byte) (byte, []byte, error) {
	if len(payload) < 46 || len(payload) > maxRecord || seq >= maxRecords {
		return 0, nil, ErrProtocol
	}
	plain, err := crypt.DecryptAEAD(payload, aead)
	if err != nil {
		return 0, nil, ErrProtocol
	}
	bad := func() (byte, []byte, error) { clear(plain); return 0, nil, ErrProtocol }
	if len(plain) < 18 || string(plain[:4]) != "CSR1" || plain[4] != dir ||
		binary.BigEndian.Uint64(plain[6:14]) != seq {
		return bad()
	}
	n := binary.BigEndian.Uint32(plain[14:18])
	if n > maxData || uint64(n)+18 != uint64(len(plain)) {
		return bad()
	}
	switch plain[5] {
	case dataKind:
		if n == 0 || seq >= maxRecords-1 {
			return bad()
		}
	case closeKind:
		if n != 0 {
			return bad()
		}
	default:
		return bad()
	}
	return plain[5], plain[18:], nil
}

func (s *Session) abort() {
	if s.stopAccept != nil {
		s.stopAccept()
	}
	s.own.abort()
}

func (s *Session) Write(p []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.own.closed.Load() {
		return 0, net.ErrClosed
	}
	total := 0
	for len(p) != 0 {
		n := min(len(p), maxData)
		payload, err := seal(s.send, s.direction, dataKind, s.tx, p[:n])
		if err == nil {
			err = writeFrame(s.own.conn, time.Now().Add(s.timeout), payload)
		}
		if err != nil {
			s.abort()
			return total, err
		}
		s.tx++
		total += n
		p = p[n:]
	}
	return total, nil
}

func (s *Session) Read(p []byte) (int, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	if s.eof {
		return 0, io.EOF
	}
	if s.own.closed.Load() {
		clear(s.pending)
		s.pending = nil
		return 0, net.ErrClosed
	}
	if len(s.pending) == 0 {
		payload, err := readFrame(s.own.conn, time.Now().Add(s.timeout), maxRecord)
		if err != nil {
			s.abort()
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		kind, data, err := open(s.recv, s.direction^1, s.rx, payload)
		if err != nil {
			s.abort()
			return 0, err
		}
		s.rx++
		if kind == closeKind {
			s.eof = true
			s.abort()
			return 0, io.EOF
		}
		s.pending = data
	}
	n := copy(p, s.pending)
	clear(s.pending[:n])
	s.pending = s.pending[n:]
	return n, nil
}

func (s *Session) Close() error {
	// Close must unblock a blocked writer, never wait for its mutex.
	if s.writeMu.TryLock() {
		if !s.own.closed.Load() {
			payload, err := seal(s.send, s.direction, closeKind, s.tx, nil)
			if err == nil {
				_ = writeFrame(s.own.conn, time.Now().Add(200*time.Millisecond), payload)
			}
		}
		s.own.abort()
		s.writeMu.Unlock()
	} else {
		s.own.abort()
	}
	if s.stopAccept != nil {
		s.stopAccept()
	}
	return nil
}

var _ io.ReadWriteCloser = (*Session)(nil)
