package codec

import (
	"errors"
	"io"
)

// Message is the JSON wire representation of a varlink request or reply.
type Message struct {
	Method     string    `json:"method,omitempty"`
	Parameters *RawValue `json:"parameters,omitempty"`
	Error      string    `json:"error,omitempty"`
	More       bool      `json:"more,omitempty"`
	Continues  bool      `json:"continues,omitempty"`
	Oneway     bool      `json:"oneway,omitempty"`
	Upgrade    bool      `json:"upgrade,omitempty"`
}

// Wire reads and writes JSON varlink messages over framed I/O.
type Wire struct {
	reader *Reader
	framer Framer
	writer io.Writer
}

type jsonDecodeError struct {
	err error
}

func (e *jsonDecodeError) Error() string { return e.err.Error() }
func (e *jsonDecodeError) Unwrap() error { return e.err }

// NewWire returns a Wire using its own buffered reader and framer.
func NewWire(r io.Reader, w io.Writer, bufferSize, maxFrameSize int) *Wire {
	framer := New(maxFrameSize)
	return &Wire{
		reader: NewReader(r, bufferSize, maxFrameSize),
		framer: framer,
		writer: w,
	}
}

// NewWireFromReader returns a Wire that reuses an existing frame Reader.
func NewWireFromReader(r *Reader, w io.Writer, maxFrameSize int) *Wire {
	framer := New(maxFrameSize)
	return &Wire{
		reader: r,
		framer: framer,
		writer: w,
	}
}

// ResetReader switches the Wire to a new underlying reader while preserving internal scratch buffers.
func (w *Wire) ResetReader(r io.Reader) {
	w.reader.Reset(r)
}

// ReadMessage reads and decodes one JSON wire message.
func (w *Wire) ReadMessage() (Message, error) {
	frame, err := w.reader.ReadFrame()
	if err != nil {
		return Message{}, err
	}

	var msg Message
	if err := unmarshalJSON(frame, &msg); err != nil {
		return Message{}, &jsonDecodeError{err: err}
	}
	return msg, nil
}

// WriteMessage encodes and writes one JSON wire message.
func (w *Wire) WriteMessage(msg Message) error {
	frame, err := marshalJSON(msg)
	if err != nil {
		return err
	}
	return w.framer.WriteFrame(w.writer, frame)
}

// Buffered returns the number of bytes already read from the underlying reader
// but not consumed by the wire decoder.
func (w *Wire) Buffered() int {
	return w.reader.BufferedReader().Buffered()
}

// ReadBuffered copies already-buffered bytes into dst without reading from the
// underlying reader.
func (w *Wire) ReadBuffered(dst []byte) (int, error) {
	return w.reader.BufferedReader().Read(dst)
}

// MarshalMessage encodes one JSON wire message without writing it.
func MarshalMessage(msg Message) ([]byte, error) {
	return marshalJSON(msg)
}

// MarshalParameters encodes arbitrary parameters into a raw JSON value.
func MarshalParameters(v any) (*RawValue, error) {
	return marshalParameters(v)
}

// DecodeParameters decodes a raw JSON value into out.
func DecodeParameters(raw *RawValue, out any) error {
	return decodeParameters(raw, out)
}

// IsDecodeError reports whether err came from JSON message decoding.
func IsDecodeError(err error) bool {
	var target *jsonDecodeError
	return errors.As(err, &target)
}
