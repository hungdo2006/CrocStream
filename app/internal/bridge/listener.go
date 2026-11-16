package bridge

import (
	"net"
	"sync"
)

// Bound connections before net/http starts handlers, including unauthenticated
// slow handshakes and hijacked WebSockets. Close always releases the slot.
type limitedListener struct {
	net.Listener
	slots       chan struct{}
	mu          sync.Mutex
	connections map[*limitedConn]struct{}
	closed      bool
}

func (l *limitedListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.slots <- struct{}{}:
			owned := &limitedConn{Conn: conn}
			owned.release = func() {
				l.mu.Lock()
				delete(l.connections, owned)
				l.mu.Unlock()
				<-l.slots
			}
			l.mu.Lock()
			if l.closed {
				l.mu.Unlock()
				_ = owned.Close()
				return nil, net.ErrClosed
			}
			if l.connections == nil {
				l.connections = make(map[*limitedConn]struct{})
			}
			l.connections[owned] = struct{}{}
			l.mu.Unlock()
			return owned, nil
		default:
			_ = conn.Close()
		}
	}
}

func (l *limitedListener) Close() error {
	l.mu.Lock()
	l.closed = true
	connections := make([]*limitedConn, 0, len(l.connections))
	for c := range l.connections {
		connections = append(connections, c)
	}
	l.mu.Unlock()
	err := l.Listener.Close()
	for _, c := range connections {
		_ = c.Close()
	}
	return err
}

type limitedConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *limitedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}
