// Package remote is what watching away from home needs: HTTPS (your own certificate or a free one
// from Let's Encrypt), a port opened on the router with UPnP, and the home's outside address.
package remote

import (
	"crypto/tls"
	"net"
	"sync"
	"time"
)

// Listener serves plain HTTP and HTTPS on the same port, as Plex does: a connection that starts
// with a TLS handshake is answered with HTTPS when config gives a TLS config (nil while HTTPS is
// off). net/http notices the *tls.Conn and fills in Request.TLS.
func Listener(ln net.Listener, config func() *tls.Config) net.Listener {
	l := &splitListener{Listener: ln, config: config, conns: make(chan net.Conn), done: make(chan struct{})}
	go l.run()
	return l
}

type splitListener struct {
	net.Listener
	config func() *tls.Config
	conns  chan net.Conn
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	err    error
}

func (l *splitListener) run() {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			l.mu.Lock()
			l.err = err
			l.mu.Unlock()
			l.Close()
			return
		}
		// Looking at the first byte waits for the client, so it's done away from Accept.
		go l.classify(c)
	}
}

func (l *splitListener) classify(c net.Conn) {
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	first := make([]byte, 1)
	if _, err := c.Read(first); err != nil {
		c.Close()
		return
	}
	c.SetReadDeadline(time.Time{})
	var out net.Conn = &peekedConn{Conn: c, first: first}
	// 0x16 starts a TLS handshake record; no HTTP method starts with it.
	if first[0] == 0x16 {
		cfg := l.config()
		if cfg == nil {
			c.Close() // HTTPS is off
			return
		}
		out = tls.Server(out, cfg)
	}
	select {
	case l.conns <- out:
	case <-l.done:
		out.Close()
	}
}

func (l *splitListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.err != nil {
			return nil, l.err
		}
		return nil, net.ErrClosed
	}
}

func (l *splitListener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.done)
		err = l.Listener.Close()
	})
	return err
}

// peekedConn gives back the byte already read before the rest of the connection.
type peekedConn struct {
	net.Conn
	first []byte
}

func (c *peekedConn) Read(p []byte) (int, error) {
	if len(c.first) > 0 && len(p) > 0 {
		n := copy(p, c.first)
		c.first = c.first[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}
