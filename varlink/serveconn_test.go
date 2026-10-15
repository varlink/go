//go:build unix

package varlink_test

import (
	"context"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/varlink/go/varlink"
)

// connDialer hands out a connection that already exists.
type connDialer struct {
	conn net.Conn
}

func (d connDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return d.conn, nil
}

// socketPair returns the two ends of a connected Unix socket pair.
func socketPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("Socketpair: %v", err)
	}
	conns := make([]*net.UnixConn, 2)
	for i, fd := range fds {
		f := os.NewFile(uintptr(fd), "socketpair")
		c, err := net.FileConn(f)
		f.Close()
		if err != nil {
			t.Fatalf("FileConn: %v", err)
		}
		conns[i] = c.(*net.UnixConn)
	}
	return conns[0], conns[1]
}

// newServedConn serves one end of a socket pair with ServeConn, and returns
// a client Connection on the other end, and a channel that is closed when
// ServeConn returns.
func newServedConn(t *testing.T, ctx context.Context) (*varlink.Connection, <-chan struct{}) {
	t.Helper()
	svc, err := varlink.NewService("Varlink", "Test", "1", "https://github.com/varlink/go")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := svc.RegisterInterface(&fdTestInterface{}); err != nil {
		t.Fatalf("RegisterInterface: %v", err)
	}

	serverConn, clientConn := socketPair(t)
	done := make(chan struct{})
	go func() {
		svc.ServeConn(ctx, serverConn)
		close(done)
	}()

	conn, err := varlink.NewConnectionWithDialer(context.Background(), "unix:@unused", connDialer{conn: clientConn})
	if err != nil {
		t.Fatalf("NewConnectionWithDialer: %v", err)
	}
	return conn, done
}

// waitDone fails the test if done is not closed in time.
func waitDone(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("ServeConn did not return after %s", what)
	}
}

// TestServeConnFDRoundTrip checks that a connection the service did not
// accept is served, file descriptors included, and that ServeConn returns
// once the peer is gone.
func TestServeConnFDRoundTrip(t *testing.T) {
	conn, done := newServedConn(t, context.Background())

	type params struct {
		Expected     string `json:"expected"`
		ReplyContent string `json:"replyContent"`
	}
	var out struct{}
	rxFDs, err := conn.CallWithFDs(
		context.Background(),
		"org.example.fdtest.EchoFDs",
		params{Expected: "ping", ReplyContent: "pong"},
		&out,
		[]*os.File{makeSendFD(t, "ping")},
	)
	if err != nil {
		t.Fatalf("CallWithFDs: %v", err)
	}
	defer func() {
		for _, f := range rxFDs {
			f.Close()
		}
	}()
	if len(rxFDs) != 1 {
		t.Fatalf("expected 1 reply fd, got %d", len(rxFDs))
	}
	if got := readFDContent(t, rxFDs[0]); got != "pong" {
		t.Errorf("reply fd content: got %q, want %q", got, "pong")
	}

	conn.Close()
	waitDone(t, done, "the peer closed the connection")
}

// TestServeConnContextCancel checks that cancelling the context stops
// serving a connection whose peer is idle.
func TestServeConnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	conn, done := newServedConn(t, ctx)
	defer conn.Close()

	cancel()
	waitDone(t, done, "the context was cancelled")
}
