package varlink

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/varlink/go/v2/internal/codec"
)

// Serve serves requests on conn until EOF, context cancellation, or upgrade.
func (s *Server) Serve(ctx context.Context, conn io.ReadWriteCloser) error {
	transport := newFramedTransport(conn, s.cfg.MaxFrameSize)
	owned := true
	defer func() {
		if owned {
			_ = conn.Close()
		}
	}()

	for {
		msg, err := transport.readMessage(ctx, s.cfg.ReadTimeout)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				return nil
			}
			return s.reportError(ctx, err)
		}

		upgraded, err := s.handleMessage(ctx, transport, msg)
		if err != nil {
			return s.reportError(ctx, err)
		}
		if upgraded {
			owned = false
			return nil
		}
	}
}

func (s *Server) handleMessage(ctx context.Context, transport *framedTransport, msg Message) (bool, error) {
	if msg.Error != "" || msg.Continues {
		return false, ErrProtocolViolation
	}
	if msg.Method == "" {
		return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "method")
	}
	if msg.Oneway && (msg.More || msg.Upgrade) {
		return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "oneway")
	}
	if msg.More && msg.Upgrade {
		return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "more")
	}
	if iface, method, err := splitMethod(msg.Method); err == nil && iface == serviceInterfaceName {
		return false, s.handleServiceMethod(ctx, transport, msg, method)
	}

	handler, err := s.lookup(msg.Method)
	if err != nil {
		switch err {
		case ErrInvalidMethod:
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "method")
		case ErrInterfaceNotFound:
			iface, _, splitErr := splitMethod(msg.Method)
			if splitErr != nil {
				return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "method")
			}
			return false, s.writeServiceError(ctx, transport, msg, serviceInterfaceNotFound, "interface", iface)
		case ErrMethodNotFound:
			_, method, splitErr := splitMethod(msg.Method)
			if splitErr != nil {
				return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "method")
			}
			return false, s.writeServiceError(ctx, transport, msg, serviceMethodNotImplemented, "method", method)
		default:
			return false, err
		}
	}

	switch handler.kind {
	case handlerKindUnary:
		if msg.More {
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "more")
		}
		if msg.Upgrade {
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "upgrade")
		}
		call := &unaryRequest{
			baseRequest: baseRequest{
				method:     msg.Method,
				parameters: msg.Parameters,
				transport:  transport,
				cfg:        s.cfg,
				oneway:     msg.Oneway,
			},
		}
		err := s.callUnaryHandler(ctx, handler.unary, call)
		if err != nil {
			return false, err
		}
		if !call.oneway && !call.replied {
			return false, call.Reply(ctx, nil)
		}
		return false, nil
	case handlerKindStream:
		if !msg.More {
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "more")
		}
		if msg.Upgrade {
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "upgrade")
		}
		call := &streamRequest{
			baseRequest: baseRequest{
				method:     msg.Method,
				parameters: msg.Parameters,
				transport:  transport,
				cfg:        s.cfg,
			},
		}
		err := s.callStreamHandler(ctx, handler.stream, call)
		if err != nil {
			return false, err
		}
		if !call.replied && !call.closed {
			return false, call.Close(ctx, nil)
		}
		return false, nil
	case handlerKindUpgrade:
		if msg.More {
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "more")
		}
		if msg.Oneway {
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "oneway")
		}
		if !msg.Upgrade {
			return false, s.writeServiceError(ctx, transport, msg, serviceInvalidParameter, "parameter", "upgrade")
		}
		call := &upgradeRequest{
			baseRequest: baseRequest{
				method:     msg.Method,
				parameters: msg.Parameters,
				transport:  transport,
				cfg:        s.cfg,
			},
		}
		err := s.callUpgradeHandler(ctx, handler.upgrade, call)
		if err != nil {
			return false, err
		}
		if !call.replied && !call.upgraded {
			_, err := call.Accept(ctx, nil)
			return err == nil, err
		}
		return call.upgraded, nil
	default:
		return false, ErrProtocolViolation
	}
}

func (s *Server) callUnaryHandler(ctx context.Context, handler UnaryHandler, call *unaryRequest) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = s.recoverCallPanic(recovered)
		}
	}()
	return handler(ctx, call)
}

func (s *Server) callStreamHandler(ctx context.Context, handler StreamHandler, call *streamRequest) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = s.recoverCallPanic(recovered)
		}
	}()
	return handler(ctx, call)
}

func (s *Server) callUpgradeHandler(ctx context.Context, handler UpgradeHandler, call *upgradeRequest) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = s.recoverCallPanic(recovered)
		}
	}()
	return handler(ctx, call)
}

func (s *Server) recoverCallPanic(recovered any) error {
	err := fmt.Errorf("varlink: handler panic: %v", recovered)
	return err
}

func (s *Server) handleServiceMethod(ctx context.Context, transport *framedTransport, request Message, method string) error {
	if request.Oneway || request.More || request.Upgrade {
		return s.writeServiceError(ctx, transport, request, serviceInvalidParameter, "parameter", "method")
	}

	switch method {
	case "GetInfo":
		info := s.info
		params, err := codec.MarshalParameters(struct {
			Vendor     string   `json:"vendor"`
			Product    string   `json:"product"`
			Version    string   `json:"version"`
			URL        string   `json:"url"`
			Interfaces []string `json:"interfaces"`
		}{
			Vendor:     info.Vendor,
			Product:    info.Product,
			Version:    info.Version,
			URL:        info.URL,
			Interfaces: s.interfaceNames(),
		})
		if err != nil {
			return err
		}
		return transport.writeMessage(ctx, s.cfg.WriteTimeout, Message{Parameters: params})
	case "GetInterfaceDescription":
		var in struct {
			Interface string `json:"interface"`
		}
		if err := codec.DecodeParameters(request.Parameters, &in); err != nil {
			return s.writeServiceError(ctx, transport, request, serviceInvalidParameter, "parameter", "parameters")
		}
		if in.Interface == "" {
			return s.writeServiceError(ctx, transport, request, serviceInvalidParameter, "parameter", "interface")
		}
		description, ok := s.interfaceDescription(in.Interface)
		if !ok {
			return s.writeServiceError(ctx, transport, request, serviceInvalidParameter, "parameter", "interface")
		}
		params, err := codec.MarshalParameters(struct {
			Description string `json:"description"`
		}{Description: description})
		if err != nil {
			return err
		}
		return transport.writeMessage(ctx, s.cfg.WriteTimeout, Message{Parameters: params})
	default:
		return s.writeServiceError(ctx, transport, request, serviceMethodNotFound, "method", method)
	}
}

func (s *Server) writeServiceError(ctx context.Context, transport *framedTransport, request Message, name, key, value string) error {
	if request.Oneway {
		return nil
	}
	params, err := codec.MarshalParameters(map[string]string{key: value})
	if err != nil {
		return err
	}
	return transport.writeMessage(ctx, s.cfg.WriteTimeout, Message{
		Error:      name,
		Parameters: params,
	})
}

func (s *Server) reportError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if s.cfg.Logger != nil {
		s.cfg.Logger.Printf("varlink server error: %v", err)
	}
	if s.cfg.ErrorHandler != nil {
		s.cfg.ErrorHandler(ctx, err)
	}
	return err
}

type baseRequest struct {
	method     string
	parameters *codec.RawValue
	transport  *framedTransport
	cfg        ServerConfig
	oneway     bool
	replied    bool
	upgraded   bool
}

func (r *baseRequest) Method() string {
	return r.method
}

func (r *baseRequest) Decode(out any) error {
	return codec.DecodeParameters(r.parameters, out)
}

func (r *baseRequest) ReplyError(ctx context.Context, name string, parameters any) error {
	if r.replied || r.upgraded {
		return ErrProtocolViolation
	}
	if r.oneway {
		r.replied = true
		return nil
	}
	params, err := codec.MarshalParameters(parameters)
	if err != nil {
		return err
	}
	r.replied = true
	return r.transport.writeMessage(ctx, r.cfg.WriteTimeout, Message{
		Error:      name,
		Parameters: params,
	})
}

type unaryRequest struct {
	baseRequest
}

func (r *unaryRequest) Reply(ctx context.Context, out any) error {
	if r.replied || r.upgraded {
		return ErrProtocolViolation
	}
	if r.oneway {
		r.replied = true
		return nil
	}
	params, err := codec.MarshalParameters(out)
	if err != nil {
		return err
	}
	r.replied = true
	return r.transport.writeMessage(ctx, r.cfg.WriteTimeout, Message{
		Parameters: params,
	})
}

func (r *unaryRequest) IsOneway() bool {
	return r.oneway
}

type streamRequest struct {
	baseRequest
	closed bool
}

func (r *streamRequest) Send(ctx context.Context, out any) error {
	if r.replied || r.closed || r.upgraded {
		return ErrProtocolViolation
	}
	params, err := codec.MarshalParameters(out)
	if err != nil {
		return err
	}
	return r.transport.writeMessage(ctx, r.cfg.WriteTimeout, Message{
		Parameters: params,
		Continues:  true,
	})
}

func (r *streamRequest) Close(ctx context.Context, out any) error {
	if r.replied || r.closed || r.upgraded {
		return ErrProtocolViolation
	}
	params, err := codec.MarshalParameters(out)
	if err != nil {
		return err
	}
	r.replied = true
	r.closed = true
	return r.transport.writeMessage(ctx, r.cfg.WriteTimeout, Message{
		Parameters: params,
	})
}

type upgradeRequest struct {
	baseRequest
}

func (r *upgradeRequest) Accept(ctx context.Context, out any) (io.ReadWriteCloser, error) {
	if r.replied || r.upgraded {
		return nil, ErrProtocolViolation
	}
	params, err := codec.MarshalParameters(out)
	if err != nil {
		return nil, err
	}
	r.replied = true
	r.upgraded = true
	if err := r.transport.writeMessage(ctx, r.cfg.WriteTimeout, Message{
		Parameters: params,
	}); err != nil {
		return nil, err
	}
	return r.transport.detach(), nil
}
