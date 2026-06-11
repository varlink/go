package varlink

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/varlink/go/v2/internal/codec"
)

// ConnClient is a framed varlink client over an existing transport connection.
type ConnClient struct {
	cfg       ClientConfig
	transport *framedTransport

	opMu    sync.Mutex
	stateMu sync.Mutex
	closed  bool
}

var _ Client = (*ConnClient)(nil)
var _ Batch = (*clientBatch)(nil)

// NewClient returns a v2 client over conn.
func NewClient(conn io.ReadWriteCloser, cfg ClientConfig) *ConnClient {
	if cfg.MaxFrameSize == 0 {
		cfg.MaxFrameSize = MaxFrameSizeDefault
	}
	return &ConnClient{
		cfg:       cfg,
		transport: newFramedTransport(conn, cfg.MaxFrameSize),
	}
}

func (c *ConnClient) Invoke(ctx context.Context, method string, in any, out any) error {
	c.opMu.Lock()
	defer c.opMu.Unlock()

	if err := c.ensureUsable(); err != nil {
		return err
	}
	if err := c.sendRequest(ctx, method, in, false, false, false); err != nil {
		return err
	}

	msg, err := c.readReply(ctx, method)
	if err != nil {
		return err
	}
	if msg.Continues {
		return c.fail(&ReplyError{Method: method, Err: ErrUnexpectedReply})
	}
	if msg.Parameters != nil {
		if err := codec.DecodeParameters(msg.Parameters, out); err != nil {
			return c.fail(&ReplyError{Method: method, Err: err})
		}
	}
	return nil
}

func (c *ConnClient) Oneway(ctx context.Context, method string, in any) error {
	c.opMu.Lock()
	defer c.opMu.Unlock()

	if err := c.ensureUsable(); err != nil {
		return err
	}
	return c.sendRequest(ctx, method, in, false, true, false)
}

func (c *ConnClient) Stream(ctx context.Context, method string, in any) (Stream, error) {
	c.opMu.Lock()
	if err := c.ensureUsable(); err != nil {
		c.opMu.Unlock()
		return nil, err
	}
	if err := c.sendRequest(ctx, method, in, true, false, false); err != nil {
		c.opMu.Unlock()
		return nil, err
	}
	return &clientStream{
		client: c,
		method: method,
	}, nil
}

func (c *ConnClient) Upgrade(ctx context.Context, method string, in any, out any) (UpgradedConn, error) {
	c.opMu.Lock()
	defer c.opMu.Unlock()

	if err := c.ensureUsable(); err != nil {
		return nil, err
	}
	if err := c.sendRequest(ctx, method, in, false, false, true); err != nil {
		return nil, err
	}

	msg, err := c.readReply(ctx, method)
	if err != nil {
		return nil, err
	}
	if msg.Continues {
		return nil, c.fail(&ReplyError{Method: method, Err: ErrUnexpectedReply})
	}
	if msg.Parameters != nil {
		if err := codec.DecodeParameters(msg.Parameters, out); err != nil {
			return nil, c.fail(&ReplyError{Method: method, Err: err})
		}
	}

	conn := c.detach()
	if conn == nil {
		return nil, io.ErrClosedPipe
	}
	return conn, nil
}

func (c *ConnClient) Batch() Batch {
	return &clientBatch{client: c}
}

func (c *ConnClient) Close() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.transport == nil || c.transport.conn == nil {
		return nil
	}
	err := c.transport.conn.Close()
	c.transport = nil
	return err
}

func (c *ConnClient) ensureUsable() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.closed || c.transport == nil || c.transport.conn == nil {
		return io.ErrClosedPipe
	}
	return nil
}

func (c *ConnClient) detach() io.ReadWriteCloser {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.closed || c.transport == nil {
		return nil
	}
	conn := c.transport.detach()
	c.closed = true
	c.transport = nil
	return conn
}

func (c *ConnClient) sendRequest(ctx context.Context, method string, in any, more, oneway, upgrade bool) error {
	if method == "" {
		return ErrInvalidMethod
	}
	params, err := codec.MarshalParameters(in)
	if err != nil {
		return err
	}
	if err := c.transport.writeMessage(ctx, c.cfg.WriteTimeout, Message{
		Method:     method,
		Parameters: params,
		More:       more,
		Oneway:     oneway,
		Upgrade:    upgrade,
	}); err != nil {
		return c.fail(err)
	}
	return nil
}

func (c *ConnClient) readReply(ctx context.Context, method string) (Message, error) {
	msg, err := c.transport.readMessage(ctx, c.cfg.ReadTimeout)
	if err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return Message{}, c.fail(err)
	}
	if msg.Method != "" || msg.More || msg.Oneway || msg.Upgrade {
		return Message{}, c.fail(&ReplyError{Method: method, Err: ErrProtocolViolation})
	}
	if remote := remoteErrorFromMessage(msg); remote != nil {
		return Message{}, remote
	}
	return msg, nil
}

func (c *ConnClient) fail(err error) error {
	_ = c.Close()
	return err
}

type clientStream struct {
	client *ConnClient
	method string
	done   bool
	closed bool
}

func (s *clientStream) Recv(ctx context.Context, out any) error {
	if s.closed {
		return ErrStreamClosed
	}
	if s.done {
		return io.EOF
	}

	msg, err := s.client.readReply(ctx, s.method)
	if err != nil {
		s.done = true
		s.release()
		return err
	}
	if msg.Parameters != nil {
		if err := codec.DecodeParameters(msg.Parameters, out); err != nil {
			client := s.client
			s.done = true
			s.release()
			err := &ReplyError{Method: s.method, Err: err}
			if client != nil {
				return client.fail(err)
			}
			return err
		}
	}
	if !msg.Continues {
		s.done = true
		s.release()
	}
	return nil
}

func (s *clientStream) Close() error {
	if s.done {
		return nil
	}
	s.done = true
	s.closed = true
	client := s.client
	s.release()
	if client == nil {
		return nil
	}
	return client.Close()
}

func (s *clientStream) release() {
	if s.client != nil {
		s.client.opMu.Unlock()
		s.client = nil
	}
}

type batchOp struct {
	method string
	in     any
	out    any
	oneway bool
}

type batchRequest struct {
	method string
	out    any
	oneway bool
	msg    Message
}

type clientBatch struct {
	client *ConnClient
	ops    []batchOp
	sent   bool
}

func (b *clientBatch) Invoke(method string, in any, out any) {
	b.ops = append(b.ops, batchOp{
		method: method,
		in:     in,
		out:    out,
	})
}

func (b *clientBatch) Oneway(method string, in any) {
	b.ops = append(b.ops, batchOp{
		method: method,
		in:     in,
		oneway: true,
	})
}

func (b *clientBatch) Send(ctx context.Context) error {
	if b.sent {
		return ErrBatchSent
	}
	b.sent = true
	if b.client == nil {
		return io.ErrClosedPipe
	}
	if len(b.ops) == 0 {
		return nil
	}

	b.client.opMu.Lock()
	defer b.client.opMu.Unlock()

	if err := b.client.ensureUsable(); err != nil {
		return err
	}
	requests, err := b.prepareRequests()
	if err != nil {
		return err
	}

	writeErr := make(chan error, 1)
	go func() {
		for _, request := range requests {
			if err := b.client.transport.writeMessage(ctx, b.client.cfg.WriteTimeout, request.msg); err != nil {
				writeErr <- b.client.fail(err)
				return
			}
		}
		writeErr <- nil
	}()
	writeDone := false
	for _, request := range requests {
		if request.oneway {
			continue
		}
		if !writeDone {
			select {
			case err := <-writeErr:
				if err != nil {
					return err
				}
				writeDone = true
			default:
			}
		}
		msg, err := b.client.readReply(ctx, request.method)
		if err != nil {
			return b.client.fail(err)
		}
		if msg.Continues {
			return b.client.fail(&ReplyError{Method: request.method, Err: ErrUnexpectedReply})
		}
		if msg.Parameters != nil {
			if err := codec.DecodeParameters(msg.Parameters, request.out); err != nil {
				return b.client.fail(&ReplyError{Method: request.method, Err: err})
			}
		}
	}
	if !writeDone {
		if err := <-writeErr; err != nil {
			return err
		}
	}
	return nil
}

func (b *clientBatch) prepareRequests() ([]batchRequest, error) {
	requests := make([]batchRequest, len(b.ops))
	framer := codec.New(b.client.cfg.MaxFrameSize)
	for i, op := range b.ops {
		if op.method == "" {
			return nil, ErrInvalidMethod
		}
		params, err := codec.MarshalParameters(op.in)
		if err != nil {
			return nil, err
		}
		msg := Message{
			Method:     op.method,
			Parameters: params,
			Oneway:     op.oneway,
		}
		frame, err := codec.MarshalMessage(codec.Message(msg))
		if err != nil {
			return nil, err
		}
		if err := framer.WriteFrame(io.Discard, frame); err != nil {
			return nil, err
		}
		requests[i] = batchRequest{
			method: op.method,
			out:    op.out,
			oneway: op.oneway,
			msg:    msg,
		}
	}
	return requests, nil
}
