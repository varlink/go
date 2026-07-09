package codec

import (
	"bufio"
	"errors"
	"io"
	"net"
)

const (
	// Delimiter separates varlink messages on the wire.
	Delimiter = byte(0)
	// DefaultMaxFrameSize is the default maximum size of a decoded message payload.
	DefaultMaxFrameSize = 1024 * 1024
	// DefaultReaderBufferSize is the default buffered reader size used by the wire codec.
	DefaultReaderBufferSize = 4096
	initialFrameBufferSize  = 4 * 1024
)

var (
	// ErrFrameTooLarge is returned when a frame exceeds the configured maximum.
	ErrFrameTooLarge = errors.New("varlink: frame too large")
	delimiterFrame   = [1]byte{Delimiter}
)

// Framer reads and writes null-delimited varlink frames.
type Framer struct {
	MaxFrameSize int
}

// Reader wraps a bufio.Reader and retains scratch space across frame reads.
// The returned frame is valid until the next ReadFrame call on the same Reader.
type Reader struct {
	framer  Framer
	reader  *bufio.Reader
	scratch []byte
}

// New returns a Framer with defaults applied.
func New(maxFrameSize int) Framer {
	return Framer{MaxFrameSize: resolveMaxFrameSize(maxFrameSize)}
}

// ReadFrame reads one null-delimited payload from r.
func (f Framer) ReadFrame(r *bufio.Reader) ([]byte, error) {
	return f.ReadFrameInto(r, nil)
}

// ReadFrameInto reads one null-delimited payload into dst and returns the payload slice.
// The returned slice aliases dst when dst has enough capacity for the payload.
func (f Framer) ReadFrameInto(r *bufio.Reader, dst []byte) ([]byte, error) {
	maxFrameSize := resolveMaxFrameSize(f.MaxFrameSize)
	frame := dst[:0]
	if cap(frame) == 0 {
		frame = make([]byte, 0, min(maxFrameSize, initialFrameBufferSize))
	}
	for {
		chunk, err := r.ReadSlice(Delimiter)

		switch err {
		case nil:
			payload := chunk[:len(chunk)-1]
			if len(frame)+len(payload) > maxFrameSize {
				return nil, ErrFrameTooLarge
			}
			return append(frame, payload...), nil
		case bufio.ErrBufferFull:
			if len(frame)+len(chunk) > maxFrameSize {
				return nil, ErrFrameTooLarge
			}
			frame = append(frame, chunk...)
			continue
		case io.EOF:
			if len(frame) == 0 && len(chunk) == 0 {
				return nil, io.EOF
			}
			if len(frame)+len(chunk) > maxFrameSize {
				return nil, ErrFrameTooLarge
			}
			return nil, io.ErrUnexpectedEOF
		default:
			return nil, err
		}
	}
}

// NewReader returns a Reader with its own bufio.Reader and reusable frame scratch buffer.
func NewReader(r io.Reader, bufferSize, maxFrameSize int) *Reader {
	if bufferSize <= 0 {
		bufferSize = DefaultReaderBufferSize
	}
	framer := New(maxFrameSize)
	return &Reader{
		framer:  framer,
		reader:  bufio.NewReaderSize(r, bufferSize),
		scratch: make([]byte, 0, min(framer.MaxFrameSize, initialFrameBufferSize)),
	}
}

// NewReaderFromBufio returns a Reader that reuses an existing bufio.Reader.
func NewReaderFromBufio(r *bufio.Reader, maxFrameSize int) *Reader {
	if r == nil {
		panic("codec: nil *bufio.Reader")
	}
	framer := New(maxFrameSize)
	return &Reader{
		framer:  framer,
		reader:  r,
		scratch: make([]byte, 0, min(framer.MaxFrameSize, initialFrameBufferSize)),
	}
}

// Reset switches the Reader to a new underlying reader while preserving the scratch buffer.
func (r *Reader) Reset(rd io.Reader) {
	r.reader.Reset(rd)
}

// BufferedReader returns the underlying bufio.Reader.
func (r *Reader) BufferedReader() *bufio.Reader {
	return r.reader
}

// ReadFrame reads one null-delimited payload using the Reader's reusable scratch buffer.
func (r *Reader) ReadFrame() ([]byte, error) {
	frame, err := r.framer.ReadFrameInto(r.reader, r.scratch)
	if err != nil {
		return nil, err
	}
	r.scratch = frame[:0]
	return frame, nil
}

// WriteFrame writes one null-delimited payload to w.
func (f Framer) WriteFrame(w io.Writer, payload []byte) error {
	maxFrameSize := resolveMaxFrameSize(f.MaxFrameSize)
	if len(payload) > maxFrameSize {
		return ErrFrameTooLarge
	}

	buffers := net.Buffers{payload, delimiterFrame[:]}
	_, err := buffers.WriteTo(w)
	return err
}

func resolveMaxFrameSize(maxFrameSize int) int {
	if maxFrameSize <= 0 {
		return DefaultMaxFrameSize
	}
	return maxFrameSize
}
