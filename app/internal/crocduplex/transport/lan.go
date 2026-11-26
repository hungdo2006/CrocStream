package transport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultLANDiscoveryPort is the UDP port used for local-network discovery.
	DefaultLANDiscoveryPort   = 9019
	defaultLANDiscoverTimeout = 400 * time.Millisecond
	lanProbeInterval          = 140 * time.Millisecond
	lanDialTimeout            = 1500 * time.Millisecond
	lanAdmissionTimeout       = 2 * time.Second
	lanMaxRejections          = 16
	lanReplyMinGap            = 25 * time.Millisecond

	lanMagic       = "CSL1"
	lanOKMagic     = "CSOK"
	lanQueryKind   = byte(1)
	lanReplyKind   = byte(2)
	lanQuerySize   = 4 + 1 + 3 + 16
	lanReplySize   = 4 + 1 + 3 + 16 + 2
	lanAdmitSize   = 4 + 16
	lanMaxUDPPkt   = 64
	lanTagByteSize = 16
)

var errLANAdmit = errors.New("lan admission failed")

// deriveLANTag derives a 16-byte domain-separated lookup tag from the room hash.
// Neither the invitation, PAKE passphrase, nor relay password is ever sent over LAN discovery.
func deriveLANTag(room string) [lanTagByteSize]byte {
	sum := sha256.Sum256([]byte("crocstream/lan-probe/v1\x00" + room))
	var tag [lanTagByteSize]byte
	copy(tag[:], sum[:lanTagByteSize])
	return tag
}

func encodeLANQuery(nonce [3]byte, tag [lanTagByteSize]byte) [lanQuerySize]byte {
	var pkt [lanQuerySize]byte
	copy(pkt[:4], lanMagic)
	pkt[4] = lanQueryKind
	copy(pkt[5:8], nonce[:])
	copy(pkt[8:24], tag[:])
	return pkt
}

func decodeLANQuery(pkt []byte, expected [lanTagByteSize]byte) ([3]byte, bool) {
	var nonce [3]byte
	if len(pkt) != lanQuerySize || string(pkt[:4]) != lanMagic || pkt[4] != lanQueryKind {
		return nonce, false
	}
	if subtle.ConstantTimeCompare(pkt[8:24], expected[:]) != 1 {
		return nonce, false
	}
	copy(nonce[:], pkt[5:8])
	return nonce, true
}

func encodeLANReply(nonce [3]byte, tag [lanTagByteSize]byte, tcpPort uint16) [lanReplySize]byte {
	var pkt [lanReplySize]byte
	copy(pkt[:4], lanMagic)
	pkt[4] = lanReplyKind
	copy(pkt[5:8], nonce[:])
	copy(pkt[8:24], tag[:])
	binary.BigEndian.PutUint16(pkt[24:26], tcpPort)
	return pkt
}

func decodeLANReply(pkt []byte, nonce [3]byte, expected [lanTagByteSize]byte) (uint16, bool) {
	if len(pkt) != lanReplySize || string(pkt[:4]) != lanMagic || pkt[4] != lanReplyKind {
		return 0, false
	}
	if subtle.ConstantTimeCompare(pkt[5:8], nonce[:]) != 1 ||
		subtle.ConstantTimeCompare(pkt[8:24], expected[:]) != 1 {
		return 0, false
	}
	port := binary.BigEndian.Uint16(pkt[24:26])
	if port == 0 {
		return 0, false
	}
	return port, true
}

type lanAcceptor struct {
	tcpLn   net.Listener
	udpConn net.PacketConn
	tag     [lanTagByteSize]byte
	tcpPort uint16
	udpPort int

	closeOnce sync.Once
	closed    chan struct{}
	wg        sync.WaitGroup
}

func startLANAcceptor(room string, opts Options) *lanAcceptor {
	listenAddr := opts.lanListenAddr
	if listenAddr == "" {
		listenAddr = ":0"
	}
	tcpLn, err := net.Listen("tcp4", listenAddr)
	if err != nil {
		return nil
	}
	tcpAddr, ok := tcpLn.Addr().(*net.TCPAddr)
	if !ok || tcpAddr.Port < 1 || tcpAddr.Port > 65535 {
		_ = tcpLn.Close()
		return nil
	}
	udpAddrStr := opts.lanUDPListenAddr()
	udpConn, err := net.ListenPacket("udp4", udpAddrStr)
	if err != nil {
		_ = tcpLn.Close()
		return nil
	}
	udpLocal, ok := udpConn.LocalAddr().(*net.UDPAddr)
	if !ok || udpLocal.Port < 1 || udpLocal.Port > 65535 {
		_ = udpConn.Close()
		_ = tcpLn.Close()
		return nil
	}
	a := &lanAcceptor{
		tcpLn:   tcpLn,
		udpConn: udpConn,
		tag:     deriveLANTag(room),
		tcpPort: uint16(tcpAddr.Port),
		udpPort: udpLocal.Port,
		closed:  make(chan struct{}),
	}
	a.wg.Add(1)
	go a.serveDiscovery()
	return a
}

func (a *lanAcceptor) serveDiscovery() {
	defer a.wg.Done()
	buf := make([]byte, lanMaxUDPPkt)
	var lastReply time.Time
	for {
		n, remote, err := a.udpConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-a.closed:
				return
			default:
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					continue
				}
				return
			}
		}
		nonce, ok := decodeLANQuery(buf[:n], a.tag)
		if !ok {
			continue
		}
		now := time.Now()
		if !lastReply.IsZero() && now.Sub(lastReply) < lanReplyMinGap {
			continue
		}
		lastReply = now
		reply := encodeLANReply(nonce, a.tag, a.tcpPort)
		_ = a.udpConn.SetWriteDeadline(now.Add(200 * time.Millisecond))
		_, _ = a.udpConn.WriteTo(reply[:], remote)
	}
}

func (a *lanAcceptor) close() {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		close(a.closed)
		_ = a.udpConn.Close()
		_ = a.tcpLn.Close()
	})
	a.wg.Wait()
}

// acceptAdmitted waits for a local-network TCP peer that presents the matching
// room tag. Unrelated scanners are rejected without aborting the listener.
func (a *lanAcceptor) acceptAdmitted(ctx context.Context) (net.Conn, error) {
	if a == nil {
		return nil, errors.New("lan acceptor unavailable")
	}
	stop := context.AfterFunc(ctx, func() { _ = a.tcpLn.Close() })
	defer stop()
	rejections := 0
	for {
		conn, err := a.tcpLn.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		if err := admitLANServer(conn, a.tag); err != nil {
			_ = conn.Close()
			rejections++
			if rejections >= lanMaxRejections {
				return nil, errors.New("lan admission limit exceeded")
			}
			continue
		}
		return conn, nil
	}
}

func admitLANServer(c net.Conn, expected [lanTagByteSize]byte) error {
	deadline := time.Now().Add(lanAdmissionTimeout)
	if err := c.SetDeadline(deadline); err != nil {
		return err
	}
	var req [lanAdmitSize]byte
	if _, err := io.ReadFull(c, req[:]); err != nil {
		return errLANAdmit
	}
	if string(req[:4]) != lanMagic || subtle.ConstantTimeCompare(req[4:], expected[:]) != 1 {
		return errLANAdmit
	}
	if err := writeAll(c, []byte(lanOKMagic)); err != nil {
		return errLANAdmit
	}
	return c.SetDeadline(time.Time{})
}

func admitLANClient(c net.Conn, tag [lanTagByteSize]byte) error {
	deadline := time.Now().Add(lanAdmissionTimeout)
	if err := c.SetDeadline(deadline); err != nil {
		return err
	}
	var req [lanAdmitSize]byte
	copy(req[:4], lanMagic)
	copy(req[4:], tag[:])
	if err := writeAll(c, req[:]); err != nil {
		return errLANAdmit
	}
	var ack [4]byte
	if _, err := io.ReadFull(c, ack[:]); err != nil {
		return errLANAdmit
	}
	if string(ack[:]) != lanOKMagic {
		return errLANAdmit
	}
	return c.SetDeadline(time.Time{})
}

func lanBroadcastTargets(port int) []*net.UDPAddr {
	seen := make(map[string]struct{})
	var targets []*net.UDPAddr
	add := func(ip net.IP) {
		ip4 := ip.To4()
		if ip4 == nil {
			return
		}
		key := ip4.String()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		targets = append(targets, &net.UDPAddr{IP: append(net.IP(nil), ip4...), Port: port})
	}
	add(net.IPv4(127, 0, 0, 1))
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagBroadcast == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip4 := ipNet.IP.To4()
				if ip4 == nil || len(ipNet.Mask) != net.IPv4len {
					continue
				}
				bcast := make(net.IP, net.IPv4len)
				for i := 0; i < net.IPv4len; i++ {
					bcast[i] = ip4[i] | ^ipNet.Mask[i]
				}
				add(bcast)
			}
		}
	}
	add(net.IPv4bcast)
	return targets
}

func discoverLAN(ctx context.Context, tag [lanTagByteSize]byte, opts Options) (string, bool) {
	timeout := opts.LANDiscoverTimeout
	if timeout <= 0 {
		timeout = defaultLANDiscoverTimeout
	}
	port := opts.lanDiscoveryPort()
	targets := opts.lanProbeTargets
	if len(targets) == 0 {
		targets = lanBroadcastTargets(port)
	}
	if len(targets) == 0 {
		return "", false
	}

	pc, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return "", false
	}
	defer pc.Close()

	var nonce [3]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", false
	}
	query := encodeLANQuery(nonce, tag)

	discCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stop := context.AfterFunc(discCtx, func() { _ = pc.Close() })
	defer stop()

	sendBurst := func() {
		for _, dst := range targets {
			_, _ = pc.WriteTo(query[:], dst)
		}
	}

	// Send one initial probe burst and one bounded retry mid-window.
	sendBurst()
	retryDone := make(chan struct{})
	go func() {
		defer close(retryDone)
		timer := time.NewTimer(lanProbeInterval)
		defer timer.Stop()
		select {
		case <-discCtx.Done():
		case <-timer.C:
			sendBurst()
		}
	}()
	defer func() {
		cancel()
		<-retryDone
	}()

	buf := make([]byte, lanMaxUDPPkt)
	deadline, _ := discCtx.Deadline()
	_ = pc.SetReadDeadline(deadline)
	for {
		n, remote, err := pc.ReadFrom(buf)
		if err != nil {
			return "", false
		}
		tcpPort, ok := decodeLANReply(buf[:n], nonce, tag)
		if !ok {
			continue
		}
		udpRemote, ok := remote.(*net.UDPAddr)
		if !ok || udpRemote.IP.To4() == nil {
			continue
		}
		return net.JoinHostPort(udpRemote.IP.To4().String(), strconv.Itoa(int(tcpPort))), true
	}
}

func dialLAN(ctx context.Context, addr string, tag [lanTagByteSize]byte) (*owner, context.Context, error) {
	life, cancel := context.WithCancel(ctx)
	dialCtx, done := context.WithTimeout(life, lanDialTimeout)
	var d net.Dialer
	raw, err := d.DialContext(dialCtx, "tcp4", addr)
	done()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if err := admitLANClient(raw, tag); err != nil {
		_ = raw.Close()
		cancel()
		return nil, nil, err
	}
	own := &owner{conn: raw, cancel: cancel}
	own.stop = context.AfterFunc(life, func() { own.closed.Store(true); _ = raw.Close() })
	if err := life.Err(); err != nil {
		own.abort()
		return nil, nil, err
	}
	return own, life, nil
}
