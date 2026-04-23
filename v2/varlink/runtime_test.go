package varlink

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestConnClientInvoke(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		b.SetInfo(ServiceInfo{
			Vendor:  "Test Vendor",
			Product: "Test Product",
			Version: "1.0",
			URL:     "https://example.test/service",
		})
		if err := b.SetInterfaceDescription("org.example.test", "interface org.example.test\n\nmethod Ping() -> ()"); err != nil {
			t.Fatalf("SetInterfaceDescription() error = %v", err)
		}
		if err := b.RegisterUnary("org.example.test", "Ping", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				Message string `json:"message"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			return call.Reply(ctx, struct {
				Message string `json:"message"`
			}{Message: "pong:" + in.Message})
		}); err != nil {
			t.Fatalf("RegisterUnary() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	var out struct {
		Message string `json:"message"`
	}
	err := client.Invoke(context.Background(), "org.example.test.Ping", struct {
		Message string `json:"message"`
	}{Message: "hello"}, &out)
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if got, want := out.Message, "pong:hello"; got != want {
		t.Fatalf("Invoke() message = %q, want %q", got, want)
	}
}

func TestConnClientServiceInterface(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		b.SetInfo(ServiceInfo{
			Vendor:  "Vendor",
			Product: "Product",
			Version: "2",
			URL:     "https://example.test/v2",
		})
		if err := b.RegisterWithDescription("org.example.test", "interface org.example.test\n\nmethod Ping() -> ()", HandlerSet{
			Unary: map[string]UnaryHandler{
				"Ping": func(ctx context.Context, call UnaryCall) error { return call.Reply(ctx, nil) },
			},
		}); err != nil {
			t.Fatalf("RegisterWithDescription() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	var info struct {
		Vendor     string   `json:"vendor"`
		Product    string   `json:"product"`
		Version    string   `json:"version"`
		URL        string   `json:"url"`
		Interfaces []string `json:"interfaces"`
	}
	if err := client.Invoke(context.Background(), "org.varlink.service.GetInfo", nil, &info); err != nil {
		t.Fatalf("GetInfo error = %v", err)
	}
	if info.Vendor != "Vendor" || info.Product != "Product" || info.Version != "2" || info.URL != "https://example.test/v2" {
		t.Fatalf("GetInfo returned wrong metadata: %+v", info)
	}
	if got, want := len(info.Interfaces), 2; got != want {
		t.Fatalf("GetInfo interfaces len = %d, want %d", got, want)
	}
	if info.Interfaces[0] != serviceInterfaceName || info.Interfaces[1] != "org.example.test" {
		t.Fatalf("GetInfo interfaces = %v", info.Interfaces)
	}

	var desc struct {
		Description string `json:"description"`
	}
	if err := client.Invoke(context.Background(), "org.varlink.service.GetInterfaceDescription", map[string]string{"interface": "org.example.test"}, &desc); err != nil {
		t.Fatalf("GetInterfaceDescription error = %v", err)
	}
	if desc.Description != "interface org.example.test\n\nmethod Ping() -> ()" {
		t.Fatalf("GetInterfaceDescription returned %q", desc.Description)
	}

	err := client.Invoke(context.Background(), "org.varlink.service.GetInterfaceDescription", map[string]string{"interface": "org.example.missing"}, &desc)
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("GetInterfaceDescription missing error = %T %v", err, err)
	}
	if remote.Name != serviceInvalidParameter {
		t.Fatalf("GetInterfaceDescription missing error name = %q", remote.Name)
	}
}

func TestConnClientOneway(t *testing.T) {
	var calls atomic.Int32
	doneCall := make(chan struct{}, 1)
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterUnary("org.example.test", "Notify", func(ctx context.Context, call UnaryCall) error {
			if !call.IsOneway() {
				t.Fatal("expected oneway request")
			}
			calls.Add(1)
			doneCall <- struct{}{}
			return nil
		}); err != nil {
			t.Fatalf("RegisterUnary() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	if err := client.Oneway(context.Background(), "org.example.test.Notify", map[string]string{"message": "hello"}); err != nil {
		t.Fatalf("Oneway() error = %v", err)
	}
	<-doneCall
	if got := calls.Load(); got != 1 {
		t.Fatalf("oneway handler calls = %d, want 1", got)
	}
}

func TestConnClientStream(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterStream("org.example.test", "Watch", func(ctx context.Context, call StreamCall) error {
			if err := call.Send(ctx, struct {
				Value string `json:"value"`
			}{Value: "one"}); err != nil {
				return err
			}
			if err := call.Send(ctx, struct {
				Value string `json:"value"`
			}{Value: "two"}); err != nil {
				return err
			}
			return call.Close(ctx, struct {
				Value string `json:"value"`
			}{Value: "three"})
		}); err != nil {
			t.Fatalf("RegisterStream() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	stream, err := client.Stream(context.Background(), "org.example.test.Watch", nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	want := []string{"one", "two", "three"}
	for _, expected := range want {
		var out struct {
			Value string `json:"value"`
		}
		if err := stream.Recv(context.Background(), &out); err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		if out.Value != expected {
			t.Fatalf("Recv() value = %q, want %q", out.Value, expected)
		}
	}
	if err := stream.Recv(context.Background(), nil); !errors.Is(err, io.EOF) {
		t.Fatalf("Recv() final error = %v, want %v", err, io.EOF)
	}
}

func TestConnClientStreamCloseBeforeFinalReplyDoesNotPanic(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterStream("org.example.test", "Watch", func(ctx context.Context, call StreamCall) error {
			return call.Send(ctx, struct {
				Value string `json:"value"`
			}{Value: "one"})
		}); err != nil {
			t.Fatalf("RegisterStream() error = %v", err)
		}
	})

	stream, err := client.Stream(context.Background(), "org.example.test.Watch", nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Stream.Close() panic = %v", recovered)
		}
	}()
	if err := stream.Close(); err != nil {
		t.Fatalf("Stream.Close() error = %v", err)
	}
	<-done
}

func TestConnClientUpgrade(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterUpgrade("org.example.test", "Bridge", func(ctx context.Context, call UpgradeCall) error {
			conn, err := call.Accept(ctx, struct {
				Ready bool `json:"ready"`
			}{Ready: true})
			if err != nil {
				return err
			}
			buf := make([]byte, 4)
			if _, err := io.ReadFull(conn, buf); err != nil {
				return err
			}
			_, err = conn.Write([]byte("pong"))
			return err
		}); err != nil {
			t.Fatalf("RegisterUpgrade() error = %v", err)
		}
	})
	defer waitServerDone(t, done)

	var out struct {
		Ready bool `json:"ready"`
	}
	upgraded, err := client.Upgrade(context.Background(), "org.example.test.Bridge", nil, &out)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !out.Ready {
		t.Fatal("Upgrade() reply did not set Ready")
	}
	defer upgraded.Close()

	if _, err := upgraded.Write([]byte("ping")); err != nil {
		t.Fatalf("upgraded Write() error = %v", err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(upgraded, reply); err != nil {
		t.Fatalf("upgraded Read() error = %v", err)
	}
	if got, want := string(reply), "pong"; got != want {
		t.Fatalf("upgraded reply = %q, want %q", got, want)
	}
}

func TestConnClientUpgradePreservesBufferedBytes(t *testing.T) {
	conn := &scriptedConn{
		reader: bytes.NewBufferString(`{"parameters":{"ready":true}}` + "\x00" + "early"),
	}
	client := NewClient(conn, ClientConfig{})

	var out struct {
		Ready bool `json:"ready"`
	}
	upgraded, err := client.Upgrade(context.Background(), "org.example.test.Bridge", nil, &out)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !out.Ready {
		t.Fatal("Upgrade() reply did not set Ready")
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(upgraded, buf); err != nil {
		t.Fatalf("upgraded Read() error = %v", err)
	}
	if got, want := string(buf), "early"; got != want {
		t.Fatalf("upgraded buffered bytes = %q, want %q", got, want)
	}
}

func TestConnClientStreamReplyErrorIsTerminal(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterStream("org.example.test", "Watch", func(ctx context.Context, call StreamCall) error {
			return call.ReplyError(ctx, "org.example.test.Failed", struct {
				Reason string `json:"reason"`
			}{Reason: "boom"})
		}); err != nil {
			t.Fatalf("RegisterStream() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	stream, err := client.Stream(context.Background(), "org.example.test.Watch", nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	err = stream.Recv(context.Background(), nil)
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Recv() error = %T %v, want *RemoteError", err, err)
	}
	if remote.Name != "org.example.test.Failed" {
		t.Fatalf("remote name = %q", remote.Name)
	}
}

func TestConnClientUpgradeReplyErrorIsTerminal(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterUpgrade("org.example.test", "Bridge", func(ctx context.Context, call UpgradeCall) error {
			return call.ReplyError(ctx, "org.example.test.Failed", struct {
				Reason string `json:"reason"`
			}{Reason: "boom"})
		}); err != nil {
			t.Fatalf("RegisterUpgrade() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	_, err := client.Upgrade(context.Background(), "org.example.test.Bridge", nil, nil)
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Upgrade() error = %T %v, want *RemoteError", err, err)
	}
	if remote.Name != "org.example.test.Failed" {
		t.Fatalf("remote name = %q", remote.Name)
	}
}

func TestConnClientBatchInvoke(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterUnary("org.example.test", "Ping", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				Message string `json:"message"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			return call.Reply(ctx, struct {
				Message string `json:"message"`
			}{Message: "pong:" + in.Message})
		}); err != nil {
			t.Fatalf("RegisterUnary() error = %v", err)
		}
		if err := b.RegisterUnary("org.example.test", "Notify", func(ctx context.Context, call UnaryCall) error {
			if !call.IsOneway() {
				t.Fatal("expected oneway request")
			}
			return nil
		}); err != nil {
			t.Fatalf("RegisterUnary() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	var first struct {
		Message string `json:"message"`
	}
	var second struct {
		Message string `json:"message"`
	}
	batch := client.Batch()
	batch.Invoke("org.example.test.Ping", map[string]string{"message": "one"}, &first)
	batch.Oneway("org.example.test.Notify", map[string]string{"message": "side"})
	batch.Invoke("org.example.test.Ping", map[string]string{"message": "two"}, &second)
	if err := batch.Send(context.Background()); err != nil {
		t.Fatalf("Batch.Send() error = %v", err)
	}
	if first.Message != "pong:one" {
		t.Fatalf("first reply = %q", first.Message)
	}
	if second.Message != "pong:two" {
		t.Fatalf("second reply = %q", second.Message)
	}
	if err := batch.Send(context.Background()); !errors.Is(err, ErrBatchSent) {
		t.Fatalf("Batch.Send() second error = %v, want %v", err, ErrBatchSent)
	}
}

func TestConnClientBatchRemoteErrorClosesClient(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterUnary("org.example.test", "Ping", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				Message string `json:"message"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			if in.Message == "fail" {
				return call.ReplyError(ctx, "org.example.test.Failed", struct {
					Reason string `json:"reason"`
				}{Reason: "boom"})
			}
			return call.Reply(ctx, struct {
				Message string `json:"message"`
			}{Message: "pong:" + in.Message})
		}); err != nil {
			t.Fatalf("RegisterUnary() error = %v", err)
		}
	})
	defer waitServerDone(t, done)

	var first struct {
		Message string `json:"message"`
	}
	var second struct {
		Message string `json:"message"`
	}
	batch := client.Batch()
	batch.Invoke("org.example.test.Ping", map[string]string{"message": "one"}, &first)
	batch.Invoke("org.example.test.Ping", map[string]string{"message": "fail"}, &second)
	err := batch.Send(context.Background())
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Batch.Send() error = %T %v, want *RemoteError", err, err)
	}
	if remote.Name != "org.example.test.Failed" {
		t.Fatalf("remote name = %q", remote.Name)
	}
	if first.Message != "pong:one" {
		t.Fatalf("first reply = %q", first.Message)
	}
	if err := client.Invoke(context.Background(), "org.example.test.Ping", map[string]string{"message": "after"}, &second); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Invoke() after batch error = %v, want %v", err, io.ErrClosedPipe)
	}
}

func TestConnClientBatchInvalidRequestReturnsPromptly(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {})
	defer closeClientServer(t, client, done)

	batch := client.Batch()
	batch.Invoke("", nil, nil)
	if err := batch.Send(context.Background()); !errors.Is(err, ErrInvalidMethod) {
		t.Fatalf("Batch.Send() error = %v, want %v", err, ErrInvalidMethod)
	}
}

func TestConnClientBatchMarshalErrorReturnsPromptly(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {})
	defer closeClientServer(t, client, done)

	batch := client.Batch()
	batch.Invoke("org.example.test.Ping", map[string]any{"bad": make(chan int)}, nil)
	if err := batch.Send(context.Background()); err == nil {
		t.Fatal("Batch.Send() unexpectedly succeeded")
	}
}

func TestConnClientRemoteError(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterUnary("org.example.test", "Fail", func(ctx context.Context, call UnaryCall) error {
			return call.ReplyError(ctx, "org.example.test.Failed", struct {
				Reason string `json:"reason"`
			}{Reason: "boom"})
		}); err != nil {
			t.Fatalf("RegisterUnary() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	err := client.Invoke(context.Background(), "org.example.test.Fail", nil, nil)
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Invoke() error = %T %v, want *RemoteError", err, err)
	}
	if remote.Name != "org.example.test.Failed" {
		t.Fatalf("RemoteError.Name = %q", remote.Name)
	}
	var params struct {
		Reason string `json:"reason"`
	}
	if err := remote.Decode(&params); err != nil {
		t.Fatalf("RemoteError.Decode() error = %v", err)
	}
	if params.Reason != "boom" {
		t.Fatalf("RemoteError.Decode() reason = %q", params.Reason)
	}
}

func TestConnClientServiceErrors(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {})
	defer closeClientServer(t, client, done)

	err := client.Invoke(context.Background(), "org.example.missing.Ping", nil, nil)
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Invoke() missing interface error = %T %v", err, err)
	}
	if remote.Name != serviceInterfaceNotFound {
		t.Fatalf("missing interface error name = %q", remote.Name)
	}

	err = client.Invoke(context.Background(), "missing", nil, nil)
	if !errors.As(err, &remote) {
		t.Fatalf("Invoke() invalid method error = %T %v", err, err)
	}
	if remote.Name != serviceInvalidParameter {
		t.Fatalf("invalid method error name = %q", remote.Name)
	}
}

func TestConnClientInvokeOnStreamMethodReturnsError(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterStream("org.example.test", "Watch", func(ctx context.Context, call StreamCall) error {
			return call.Close(ctx, nil)
		}); err != nil {
			t.Fatalf("RegisterStream() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	err := client.Invoke(context.Background(), "org.example.test.Watch", nil, nil)
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Invoke() error = %T %v", err, err)
	}
	if remote.Name != serviceInvalidParameter {
		t.Fatalf("Invoke() error name = %q", remote.Name)
	}
}

func TestConnClientAnyRoundTripPreservesJSONShape(t *testing.T) {
	client, done := newTestClientServer(t, func(b *ServerBuilder) {
		if err := b.RegisterUnary("org.example.types", "Echo", func(ctx context.Context, call UnaryCall) error {
			var in struct {
				Value any `json:"value"`
			}
			if err := call.Decode(&in); err != nil {
				return err
			}
			return call.Reply(ctx, in)
		}); err != nil {
			t.Fatalf("RegisterUnary() error = %v", err)
		}
	})
	defer closeClientServer(t, client, done)

	in := struct {
		Value any `json:"value"`
	}{
		Value: map[string]any{
			"bool":   true,
			"int":    float64(7),
			"float":  3.5,
			"string": "hello",
			"array":  []any{"x", float64(1)},
			"object": map[string]any{"k": "v"},
		},
	}
	var out struct {
		Value any `json:"value"`
	}
	if err := client.Invoke(context.Background(), "org.example.types.Echo", in, &out); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("any round trip mismatch:\n got: %#v\nwant: %#v", out, in)
	}
}

func newTestClientServer(t *testing.T, register func(*ServerBuilder)) (*ConnClient, <-chan error) {
	t.Helper()

	builder := NewServerBuilder(ServerConfig{})
	register(builder)
	server := builder.Build()

	serverConn, clientConn := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(context.Background(), serverConn)
	}()

	return NewClient(clientConn, ClientConfig{}), done
}

func closeClientServer(t *testing.T, client *ConnClient, done <-chan error) {
	t.Helper()
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close() error = %v", err)
	}
	waitServerDone(t, done)
}

func waitServerDone(t *testing.T, done <-chan error) {
	t.Helper()
	if err := <-done; err != nil {
		t.Fatalf("server.Serve() error = %v", err)
	}
}

type scriptedConn struct {
	reader *bytes.Buffer
	writer bytes.Buffer
	closed bool
}

func (c *scriptedConn) Read(p []byte) (int, error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	return c.reader.Read(p)
}

func (c *scriptedConn) Write(p []byte) (int, error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	return c.writer.Write(p)
}

func (c *scriptedConn) Close() error {
	c.closed = true
	return nil
}
