package varlink

import (
	"context"
	"net"
	"testing"
)

func TestNewConnection(t *testing.T) {
	// Start a simple TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Test NewConnection
	conn, err := NewConnection(context.Background(), "tcp:"+addr)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestNewConnectionWithDialer(t *testing.T) {
	// Start a simple TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Track if custom dialer was called
	called := false
	customDialer := &testDialer{
		Dialer: &net.Dialer{},
		onDial: func() { called = true },
	}

	// Test NewConnectionWithDialer
	conn, err := NewConnectionWithDialer(context.Background(), "tcp:"+addr, customDialer)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if !called {
		t.Fatal("custom dialer was not used")
	}
}

// testDialer wraps net.Dialer to track usage
type testDialer struct {
	*net.Dialer
	onDial func()
}

func (d *testDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.onDial()

	return d.Dialer.DialContext(ctx, network, address)
}
