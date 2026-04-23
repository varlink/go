package varlink

import (
	"errors"

	"github.com/varlink/go/v2/internal/codec"
)

var (
	// ErrFrameTooLarge is returned when a peer sends a message larger than the configured limit.
	ErrFrameTooLarge = codec.ErrFrameTooLarge
	// ErrStreamClosed is returned when receiving from a closed stream.
	ErrStreamClosed = errors.New("varlink: stream closed")
	// ErrAlreadyStarted is returned when a server is started more than once.
	ErrAlreadyStarted = errors.New("varlink: server already started")
	// ErrDuplicateInterface is returned when the same interface is registered twice.
	ErrDuplicateInterface = errors.New("varlink: interface already registered")
	// ErrDuplicateMethod is returned when the same method is registered more than once.
	ErrDuplicateMethod = errors.New("varlink: method already registered")
	// ErrInvalidInterface is returned when an interface name is malformed.
	ErrInvalidInterface = errors.New("varlink: invalid interface name")
	// ErrInvalidMethod is returned when a fully qualified method name is malformed.
	ErrInvalidMethod = errors.New("varlink: invalid method name")
	// ErrNilHandler is returned when a nil handler is registered.
	ErrNilHandler = errors.New("varlink: nil handler")
	// ErrInterfaceNotFound is returned when a method references an unknown interface.
	ErrInterfaceNotFound = errors.New("varlink: interface not found")
	// ErrMethodNotFound is returned when a method lookup misses within a known interface.
	ErrMethodNotFound = errors.New("varlink: method not found")
	// ErrBatchSent is returned when a batch is reused after Send.
	ErrBatchSent = errors.New("varlink: batch already sent")
)

// ReplyError represents a protocol or decode failure observed while receiving a reply.
type ReplyError struct {
	Method string
	Err    error
}

func (e *ReplyError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Method == "" {
		return "varlink reply error: " + e.Err.Error()
	}
	return "varlink reply error for " + e.Method + ": " + e.Err.Error()
}

func (e *ReplyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
