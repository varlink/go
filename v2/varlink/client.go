package varlink

import (
	"context"
	"io"
	"time"
)

// ClientConfig contains transport policy for a client instance.
type ClientConfig struct {
	MaxFrameSize int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Client is the high-level v2 client API.
type Client interface {
	Invoke(ctx context.Context, method string, in any, out any) error
	Oneway(ctx context.Context, method string, in any) error
	Stream(ctx context.Context, method string, in any) (Stream, error)
	Upgrade(ctx context.Context, method string, in any, out any) (UpgradedConn, error)
	Batch() Batch
	io.Closer
}

// Batch pipelines multiple unary and oneway requests over one connection.
type Batch interface {
	Invoke(method string, in any, out any)
	Oneway(method string, in any)
	Send(ctx context.Context) error
}

// Stream receives zero or more replies from a streaming method.
type Stream interface {
	Recv(ctx context.Context, out any) error
	Close() error
}

// UpgradedConn is returned after a successful protocol upgrade.
type UpgradedConn interface {
	io.ReadWriteCloser
}
