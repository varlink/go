package varlink

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/varlink/go/v2/internal/codec"
)

const (
	serviceInterfaceNotFound    = "org.varlink.service.InterfaceNotFound"
	serviceMethodNotFound       = "org.varlink.service.MethodNotFound"
	serviceMethodNotImplemented = "org.varlink.service.MethodNotImplemented"
	serviceInvalidParameter     = "org.varlink.service.InvalidParameter"
	serviceInternalError        = "org.varlink.service.InternalError"
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
		wire: codec.NewWire(conn, conn, codec.DefaultReaderBufferSize, maxFrameSize),
	}
}

func (t *framedTransport) readMessage(ctx context.Context, timeout time.Duration) (Message, error) {
	stopDeadline, err := setReadDeadline(t.conn, ctx, timeout)
	if err != nil {
		return Message{}, err
	}
	defer stopDeadline()

	msg, err := t.wire.ReadMessage()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Message{}, ctxErr
		}
		if codec.IsDecodeError(err) {
			return Message{}, &ReplyError{Err: err}
		}
		return Message{}, err
	}
	return Message(msg), nil
}

func (t *framedTransport) writeMessage(ctx context.Context, timeout time.Duration, msg Message) error {
	stopDeadline, err := setWriteDeadline(t.conn, ctx, timeout)
	if err != nil {
		return err
	}
	defer stopDeadline()

	if err := t.wire.WriteMessage(codec.Message(msg)); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	return nil
}

func (t *framedTransport) detach() io.ReadWriteCloser {
	if t.wire.Buffered() == 0 {
		return t.conn
	}
	conn := &bufferedConn{
		conn: t.conn,
		wire: t.wire,
	}
	if netConn, ok := t.conn.(net.Conn); ok {
		return &bufferedNetConn{
			bufferedConn: conn,
			netConn:      netConn,
		}
	}
	return conn
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

type bufferedNetConn struct {
	*bufferedConn
	netConn net.Conn
}

func (c *bufferedNetConn) LocalAddr() net.Addr {
	return c.netConn.LocalAddr()
}

func (c *bufferedNetConn) RemoteAddr() net.Addr {
	return c.netConn.RemoteAddr()
}

func (c *bufferedNetConn) SetDeadline(t time.Time) error {
	return c.netConn.SetDeadline(t)
}

func (c *bufferedNetConn) SetReadDeadline(t time.Time) error {
	return c.netConn.SetReadDeadline(t)
}

func (c *bufferedNetConn) SetWriteDeadline(t time.Time) error {
	return c.netConn.SetWriteDeadline(t)
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

func setReadDeadline(conn io.ReadWriteCloser, ctx context.Context, timeout time.Duration) (func(), error) {
	r, ok := conn.(deadlineReader)
	if !ok {
		return func() {}, nil
	}
	deadline, ok := resolveDeadline(ctx, timeout)
	if !ok {
		deadline = time.Time{}
	}
	return setDeadline(ctx, deadline, r.SetReadDeadline)
}

func setWriteDeadline(conn io.ReadWriteCloser, ctx context.Context, timeout time.Duration) (func(), error) {
	w, ok := conn.(deadlineWriter)
	if !ok {
		return func() {}, nil
	}
	deadline, ok := resolveDeadline(ctx, timeout)
	if !ok {
		deadline = time.Time{}
	}
	return setDeadline(ctx, deadline, w.SetWriteDeadline)
}

func setDeadline(ctx context.Context, deadline time.Time, set func(time.Time) error) (func(), error) {
	if err := set(deadline); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	stopContext := context.AfterFunc(ctx, func() {
		_ = set(time.Now())
		close(done)
	})
	if ctx.Err() != nil {
		_ = set(time.Now())
	}
	return func() {
		if !stopContext() {
			<-done
		}
		_ = set(time.Time{})
	}, nil
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
