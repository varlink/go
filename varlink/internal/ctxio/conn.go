package ctxio

import (
	"bufio"
	"context"
	"net"
	"time"
)

// Conn wraps net.Conn with context aware functionality.
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// NewConn creates a new context aware Conn.
func NewConn(c net.Conn) *Conn {
	return &Conn{
		conn:   c,
		reader: bufio.NewReader(c),
	}
}

// aLongTimeAgo is a time in the past that indicates a connection should
// immediately time out.
var aLongTimeAgo = time.Unix(1, 0)

func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// Close releases the Conns resources.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Write writes to the underlying connection.
// It is not safe for concurrent use with itself.
func (c *Conn) Write(ctx context.Context, buf []byte) (int, error) {
	done := make(chan struct{})
	ioInterrupted := context.AfterFunc(ctx, func() {
		c.conn.SetWriteDeadline(aLongTimeAgo)
		close(done)
	})
	n, err := c.conn.Write(buf)
	if !ioInterrupted() {
		<-done
		c.conn.SetWriteDeadline(time.Time{})
		return n, ctx.Err()
	}
	return n, err
}

// Read reads from the underlying connection.
// It is not safe for concurrent use with itself or ReadBytes.
func (c *Conn) Read(ctx context.Context, buf []byte) (int, error) {
	done := make(chan struct{})
	ioInterrupted := context.AfterFunc(ctx, func() {
		c.conn.SetReadDeadline(aLongTimeAgo)
		close(done)
	})
	n, err := c.conn.Read(buf)
	if !ioInterrupted() {
		<-done
		c.conn.SetReadDeadline(time.Time{})
		return n, ctx.Err()
	}
	return n, err
}

// ReadBytes reads from the connection until the bytes are found.
// It is not safe for concurrent use with itself or Read.
func (c *Conn) ReadBytes(ctx context.Context, delim byte) ([]byte, error) {
	done := make(chan struct{})
	ioInterrupted := context.AfterFunc(ctx, func() {
		c.conn.SetReadDeadline(aLongTimeAgo)
		close(done)
	})
	out, err := c.reader.ReadBytes(delim)
	if !ioInterrupted() {
		<-done
		c.conn.SetReadDeadline(time.Time{})
		return out, ctx.Err()
	}
	return out, err
}
