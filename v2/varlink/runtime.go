package varlink

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/varlink/go/v2/internal/codec"
)

const (
	serviceInterfaceNotFound    = "org.varlink.service.InterfaceNotFound"
	serviceMethodNotFound       = "org.varlink.service.MethodNotFound"
	serviceMethodNotImplemented = "org.varlink.service.MethodNotImplemented"
	serviceInvalidParameter     = "org.varlink.service.InvalidParameter"
)

var (
	// ErrProtocolViolation is returned when a peer sends a structurally invalid varlink message.
	ErrProtocolViolation = errors.New("varlink: protocol violation")
	// ErrUnexpectedReply is returned when the peer sends a reply shape that does not match the request.
	ErrUnexpectedReply = errors.New("varlink: unexpected reply")
)

// RemoteError is a remote varlink error reply.
type RemoteError struct {
	Name       string
	Parameters codec.RawValue
}

func (e *RemoteError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Name
}

// Decode decodes the remote error parameters into out.
func (e *RemoteError) Decode(out any) error {
	if e == nil || out == nil || len(e.Parameters) == 0 {
		return nil
	}
	return codec.DecodeParameters(&e.Parameters, out)
}

type deadlineReader interface {
	SetReadDeadline(time.Time) error
}

type deadlineWriter interface {
	SetWriteDeadline(time.Time) error
}

type framedTransport struct {
	conn io.ReadWriteCloser
	wire *codec.Wire
}

func newFramedTransport(conn io.ReadWriteCloser, maxFrameSize int) *framedTransport {
	return &framedTransport{
		conn: conn,
		wire: codec.NewWire(conn, conn, 4096, maxFrameSize),
	}
}

func (t *framedTransport) readMessage(ctx context.Context, timeout time.Duration) (Message, error) {
	if err := setReadDeadline(t.conn, ctx, timeout); err != nil {
		return Message{}, err
	}
	defer clearReadDeadline(t.conn)

	msg, err := t.wire.ReadMessage()
	if err != nil {
		if codec.IsDecodeError(err) {
			return Message{}, &ReplyError{Err: err}
		}
		return Message{}, err
	}
	return Message(msg), nil
}

func (t *framedTransport) writeMessage(ctx context.Context, timeout time.Duration, msg Message) error {
	if err := setWriteDeadline(t.conn, ctx, timeout); err != nil {
		return err
	}
	defer clearWriteDeadline(t.conn)

	return t.wire.WriteMessage(codec.Message(msg))
}

func (t *framedTransport) detach() io.ReadWriteCloser {
	if t.wire.Buffered() == 0 {
		return t.conn
	}
	return &bufferedConn{
		conn: t.conn,
		wire: t.wire,
	}
}

type bufferedConn struct {
	conn io.ReadWriteCloser
	wire *codec.Wire
	mu   sync.Mutex
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.wire != nil {
		if c.wire.Buffered() > 0 {
			return c.wire.ReadBuffered(p)
		}
		c.wire = nil
	}
	return c.conn.Read(p)
}

func (c *bufferedConn) Write(p []byte) (int, error) {
	return c.conn.Write(p)
}

func (c *bufferedConn) Close() error {
	return c.conn.Close()
}

func remoteErrorFromMessage(msg Message) *RemoteError {
	if msg.Error == "" {
		return nil
	}
	var params codec.RawValue
	if msg.Parameters != nil {
		params = append(params[:0], (*msg.Parameters)...)
	}
	return &RemoteError{
		Name:       msg.Error,
		Parameters: params,
	}
}

func setReadDeadline(conn io.ReadWriteCloser, ctx context.Context, timeout time.Duration) error {
	r, ok := conn.(deadlineReader)
	if !ok {
		return nil
	}
	deadline, ok := resolveDeadline(ctx, timeout)
	if !ok {
		return r.SetReadDeadline(time.Time{})
	}
	return r.SetReadDeadline(deadline)
}

func clearReadDeadline(conn io.ReadWriteCloser) {
	if r, ok := conn.(deadlineReader); ok {
		_ = r.SetReadDeadline(time.Time{})
	}
}

func setWriteDeadline(conn io.ReadWriteCloser, ctx context.Context, timeout time.Duration) error {
	w, ok := conn.(deadlineWriter)
	if !ok {
		return nil
	}
	deadline, ok := resolveDeadline(ctx, timeout)
	if !ok {
		return w.SetWriteDeadline(time.Time{})
	}
	return w.SetWriteDeadline(deadline)
}

func clearWriteDeadline(conn io.ReadWriteCloser) {
	if w, ok := conn.(deadlineWriter); ok {
		_ = w.SetWriteDeadline(time.Time{})
	}
}

func resolveDeadline(ctx context.Context, timeout time.Duration) (time.Time, bool) {
	var deadline time.Time
	var ok bool
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
		ok = true
	}
	if ctxDeadline, has := ctx.Deadline(); has {
		if !ok || ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
			ok = true
		}
	}
	return deadline, ok
}
