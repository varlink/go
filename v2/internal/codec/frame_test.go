package codec

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestFramerReadFrame(t *testing.T) {
	framer := New(16)
	reader := bufio.NewReader(bytes.NewReader([]byte("hello\x00world\x00")))

	frame, err := framer.ReadFrame(reader)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if got, want := string(frame), "hello"; got != want {
		t.Fatalf("ReadFrame() = %q, want %q", got, want)
	}

	frame, err = framer.ReadFrame(reader)
	if err != nil {
		t.Fatalf("ReadFrame() second error = %v", err)
	}
	if got, want := string(frame), "world"; got != want {
		t.Fatalf("ReadFrame() second = %q, want %q", got, want)
	}
}

func TestFramerReadFrameUnexpectedEOF(t *testing.T) {
	framer := New(16)
	reader := bufio.NewReader(bytes.NewReader([]byte("partial")))

	_, err := framer.ReadFrame(reader)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadFrame() error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func TestFramerReadFrameEOFOnEmptyReader(t *testing.T) {
	framer := New(16)
	reader := bufio.NewReader(bytes.NewReader(nil))

	_, err := framer.ReadFrame(reader)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("ReadFrame() error = %v, want %v", err, io.EOF)
	}
}

func TestFramerReadFrameTooLarge(t *testing.T) {
	framer := New(4)
	reader := bufio.NewReaderSize(bytes.NewReader([]byte("hello\x00")), 2)

	_, err := framer.ReadFrame(reader)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("ReadFrame() error = %v, want %v", err, ErrFrameTooLarge)
	}
}

func TestFramerReadFrameAtLimit(t *testing.T) {
	framer := New(5)
	reader := bufio.NewReaderSize(bytes.NewReader([]byte("hello\x00")), 2)

	frame, err := framer.ReadFrame(reader)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if got, want := string(frame), "hello"; got != want {
		t.Fatalf("ReadFrame() = %q, want %q", got, want)
	}
}

func TestFramerReadFrameIntoReusesBuffer(t *testing.T) {
	framer := New(16)
	reader := bufio.NewReaderSize(bytes.NewReader([]byte("hello\x00")), 2)
	dst := make([]byte, 0, 5)

	frame, err := framer.ReadFrameInto(reader, dst)
	if err != nil {
		t.Fatalf("ReadFrameInto() error = %v", err)
	}
	if got, want := string(frame), "hello"; got != want {
		t.Fatalf("ReadFrameInto() = %q, want %q", got, want)
	}
	if len(frame) == 0 {
		t.Fatal("ReadFrameInto() returned empty frame")
	}
	if &frame[:cap(frame)][0] != &dst[:cap(dst)][0] {
		t.Fatal("ReadFrameInto() did not reuse destination buffer")
	}
}

func TestFramerReadFrameIntoTooLarge(t *testing.T) {
	framer := New(4)
	reader := bufio.NewReaderSize(bytes.NewReader([]byte("hello\x00")), 2)
	dst := make([]byte, 0, 4)

	_, err := framer.ReadFrameInto(reader, dst)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("ReadFrameInto() error = %v, want %v", err, ErrFrameTooLarge)
	}
}

func TestReaderReadFrameReusesScratch(t *testing.T) {
	source := bytes.NewReader([]byte("hello\x00world\x00"))
	reader := NewReader(source, 2, 16)

	first, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() first error = %v", err)
	}
	if got, want := string(first), "hello"; got != want {
		t.Fatalf("ReadFrame() first = %q, want %q", got, want)
	}
	if len(first) == 0 {
		t.Fatal("ReadFrame() returned empty first frame")
	}
	firstPtr := &first[:cap(first)][0]

	second, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() second error = %v", err)
	}
	if got, want := string(second), "world"; got != want {
		t.Fatalf("ReadFrame() second = %q, want %q", got, want)
	}
	if &second[:cap(second)][0] != firstPtr {
		t.Fatal("ReadFrame() did not reuse scratch buffer")
	}
}

func TestReaderReset(t *testing.T) {
	reader := NewReader(bytes.NewReader([]byte("hello\x00")), 4, 16)

	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() before Reset error = %v", err)
	}
	if got, want := string(frame), "hello"; got != want {
		t.Fatalf("ReadFrame() before Reset = %q, want %q", got, want)
	}

	reader.Reset(bytes.NewReader([]byte("world\x00")))
	frame, err = reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame() after Reset error = %v", err)
	}
	if got, want := string(frame), "world"; got != want {
		t.Fatalf("ReadFrame() after Reset = %q, want %q", got, want)
	}
}

func TestFramerWriteFrame(t *testing.T) {
	framer := New(16)
	var out bytes.Buffer

	if err := framer.WriteFrame(&out, []byte("hello")); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	if got, want := out.String(), "hello\x00"; got != want {
		t.Fatalf("WriteFrame() wrote %q, want %q", got, want)
	}
}

func TestFramerWriteFrameTooLarge(t *testing.T) {
	framer := New(4)
	var out bytes.Buffer

	err := framer.WriteFrame(&out, []byte("hello"))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("WriteFrame() error = %v, want %v", err, ErrFrameTooLarge)
	}
}
