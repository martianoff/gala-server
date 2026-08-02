package httpcore

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// TCPServer accepts connections and runs a handler per connection, with
// graceful shutdown: on SIGINT/SIGTERM it stops accepting, lets in-flight
// connections finish, and force-closes whatever is still running when the
// timeout expires.
//
// This mirrors what http.Server.Shutdown does for HTTP. The Go standard library
// has no equivalent for a bare net.Listener, so the accounting is done here:
// a WaitGroup tracks live handlers and a registry of open connections provides
// the force-close path.
type TCPServer struct {
	ln net.Listener

	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closing bool

	wg sync.WaitGroup
}

// NewTCPServer binds addr and returns a server ready to Serve.
func NewTCPServer(addr string) (*TCPServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &TCPServer{ln: ln, conns: make(map[net.Conn]struct{})}, nil
}

// Addr is the bound address, useful when the OS chose the port (":0").
func (s *TCPServer) Addr() string { return s.ln.Addr().String() }

// Listener exposes the underlying listener for callers that want to drive the
// accept loop themselves.
func (s *TCPServer) Listener() net.Listener { return s.ln }

// track registers a live connection so shutdown can force-close it. It reports
// false if the server is already closing, in which case the caller must not
// serve the connection.
func (s *TCPServer) track(c net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.conns[c] = struct{}{}
	return true
}

func (s *TCPServer) untrack(c net.Conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

// Serve accepts until the listener is closed, running handler per connection.
//
// It returns nil on a clean shutdown so callers can distinguish "stopped on
// purpose" from a real accept failure.
func (s *TCPServer) Serve(handler func(net.Conn)) error {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing {
				return nil
			}
			return err
		}
		if !s.track(c) {
			c.Close()
			continue
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer s.untrack(c)
			handler(c)
		}(c)
	}
}

// Shutdown stops accepting, waits for in-flight connections up to timeout, then
// force-closes any that remain. Returns context.DeadlineExceeded if it had to
// force-close, so a caller can tell a clean drain from a truncated one.
func (s *TCPServer) Shutdown(timeout time.Duration) error {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return nil
	}
	s.closing = true
	s.mu.Unlock()

	// Stop accepting first; in-flight handlers keep running.
	s.ln.Close()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// Timed out: force-close whatever is still open. Each handler's next
		// read or write fails, so they unwind rather than being killed.
		s.mu.Lock()
		for c := range s.conns {
			c.Close()
		}
		s.mu.Unlock()
		<-done
		return ctx.Err()
	}
}

// ServeGraceful runs Serve and shuts down on SIGINT/SIGTERM, mirroring
// ListenAndServeGraceful for HTTP.
func (s *TCPServer) ServeGraceful(handler func(net.Conn), shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(handler)
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		fmt.Println("\nShutdown signal received, draining connections...")
		return s.Shutdown(shutdownTimeout)
	}
}
