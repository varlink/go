package codec

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

func BenchmarkFramerReadFrame(b *testing.B) {
	cases := []struct {
		name       string
		payloadLen int
		bufferSize int
	}{
		{name: "small/buf64", payloadLen: 64, bufferSize: 64},
		{name: "small/buf4k", payloadLen: 64, bufferSize: 4096},
		{name: "medium/buf64", payloadLen: 4 * 1024, bufferSize: 64},
		{name: "medium/buf4k", payloadLen: 4 * 1024, bufferSize: 4096},
		{name: "large/buf64", payloadLen: 64 * 1024, bufferSize: 64},
		{name: "large/buf4k", payloadLen: 64 * 1024, bufferSize: 4096},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := benchmarkPayload(tc.payloadLen)
			frame := append(append([]byte(nil), payload...), Delimiter)
			source := bytes.NewReader(nil)
			reader := bufio.NewReaderSize(source, tc.bufferSize)
			framer := New(tc.payloadLen)

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(frame)
				reader.Reset(source)

				got, err := framer.ReadFrame(reader)
				if err != nil {
					b.Fatalf("ReadFrame() error = %v", err)
				}
				if len(got) != len(payload) {
					b.Fatalf("ReadFrame() len = %d, want %d", len(got), len(payload))
				}
			}
		})
	}
}

func BenchmarkFramerReadFrameInto(b *testing.B) {
	cases := []struct {
		name       string
		payloadLen int
		bufferSize int
	}{
		{name: "small/buf64", payloadLen: 64, bufferSize: 64},
		{name: "small/buf4k", payloadLen: 64, bufferSize: 4096},
		{name: "medium/buf64", payloadLen: 4 * 1024, bufferSize: 64},
		{name: "medium/buf4k", payloadLen: 4 * 1024, bufferSize: 4096},
		{name: "large/buf64", payloadLen: 64 * 1024, bufferSize: 64},
		{name: "large/buf4k", payloadLen: 64 * 1024, bufferSize: 4096},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := benchmarkPayload(tc.payloadLen)
			frame := append(append([]byte(nil), payload...), Delimiter)
			source := bytes.NewReader(nil)
			reader := bufio.NewReaderSize(source, tc.bufferSize)
			framer := New(tc.payloadLen)
			dst := make([]byte, 0, tc.payloadLen)

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(frame)
				reader.Reset(source)

				got, err := framer.ReadFrameInto(reader, dst)
				if err != nil {
					b.Fatalf("ReadFrameInto() error = %v", err)
				}
				if len(got) != len(payload) {
					b.Fatalf("ReadFrameInto() len = %d, want %d", len(got), len(payload))
				}
			}
		})
	}
}

func BenchmarkReaderReadFrame(b *testing.B) {
	cases := []struct {
		name       string
		payloadLen int
		bufferSize int
	}{
		{name: "small/buf64", payloadLen: 64, bufferSize: 64},
		{name: "small/buf4k", payloadLen: 64, bufferSize: 4096},
		{name: "medium/buf64", payloadLen: 4 * 1024, bufferSize: 64},
		{name: "medium/buf4k", payloadLen: 4 * 1024, bufferSize: 4096},
		{name: "large/buf64", payloadLen: 64 * 1024, bufferSize: 64},
		{name: "large/buf4k", payloadLen: 64 * 1024, bufferSize: 4096},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := benchmarkPayload(tc.payloadLen)
			frame := append(append([]byte(nil), payload...), Delimiter)
			source := bytes.NewReader(nil)
			reader := NewReader(source, tc.bufferSize, tc.payloadLen)

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(frame)
				reader.Reset(source)

				got, err := reader.ReadFrame()
				if err != nil {
					b.Fatalf("Reader.ReadFrame() error = %v", err)
				}
				if len(got) != len(payload) {
					b.Fatalf("Reader.ReadFrame() len = %d, want %d", len(got), len(payload))
				}
			}
		})
	}
}

func BenchmarkFramerReadFrameCompareReadBytes(b *testing.B) {
	cases := []struct {
		name       string
		payloadLen int
		bufferSize int
	}{
		{name: "medium/buf64", payloadLen: 4 * 1024, bufferSize: 64},
		{name: "medium/buf4k", payloadLen: 4 * 1024, bufferSize: 4096},
		{name: "large/buf64", payloadLen: 64 * 1024, bufferSize: 64},
		{name: "large/buf4k", payloadLen: 64 * 1024, bufferSize: 4096},
	}

	for _, tc := range cases {
		payload := benchmarkPayload(tc.payloadLen)
		frame := append(append([]byte(nil), payload...), Delimiter)

		b.Run(tc.name+"/framer", func(b *testing.B) {
			source := bytes.NewReader(nil)
			reader := bufio.NewReaderSize(source, tc.bufferSize)
			framer := New(tc.payloadLen)

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(frame)
				reader.Reset(source)

				got, err := framer.ReadFrame(reader)
				if err != nil {
					b.Fatalf("ReadFrame() error = %v", err)
				}
				if len(got) != len(payload) {
					b.Fatalf("ReadFrame() len = %d, want %d", len(got), len(payload))
				}
			}
		})

		b.Run(tc.name+"/into", func(b *testing.B) {
			source := bytes.NewReader(nil)
			reader := bufio.NewReaderSize(source, tc.bufferSize)
			framer := New(tc.payloadLen)
			dst := make([]byte, 0, tc.payloadLen)

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(frame)
				reader.Reset(source)

				got, err := framer.ReadFrameInto(reader, dst)
				if err != nil {
					b.Fatalf("ReadFrameInto() error = %v", err)
				}
				if len(got) != len(payload) {
					b.Fatalf("ReadFrameInto() len = %d, want %d", len(got), len(payload))
				}
			}
		})

		b.Run(tc.name+"/readbytes", func(b *testing.B) {
			source := bytes.NewReader(nil)
			reader := bufio.NewReaderSize(source, tc.bufferSize)

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(frame)
				reader.Reset(source)

				got, err := reader.ReadBytes(Delimiter)
				if err != nil {
					b.Fatalf("ReadBytes() error = %v", err)
				}
				if len(got) != len(frame) {
					b.Fatalf("ReadBytes() len = %d, want %d", len(got), len(frame))
				}
			}
		})

		b.Run(tc.name+"/reader", func(b *testing.B) {
			source := bytes.NewReader(nil)
			reader := NewReader(source, tc.bufferSize, tc.payloadLen)

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(frame)
				reader.Reset(source)

				got, err := reader.ReadFrame()
				if err != nil {
					b.Fatalf("Reader.ReadFrame() error = %v", err)
				}
				if len(got) != len(payload) {
					b.Fatalf("Reader.ReadFrame() len = %d, want %d", len(got), len(payload))
				}
			}
		})
	}
}

func BenchmarkFramerWriteFrame(b *testing.B) {
	cases := []struct {
		name       string
		payloadLen int
	}{
		{name: "small", payloadLen: 64},
		{name: "medium", payloadLen: 4 * 1024},
		{name: "large", payloadLen: 64 * 1024},
	}

	for _, tc := range cases {
		b.Run(tc.name+"/discard", func(b *testing.B) {
			payload := benchmarkPayload(tc.payloadLen)
			framer := New(tc.payloadLen)

			b.ReportAllocs()
			b.SetBytes(int64(len(payload) + 1))
			b.ResetTimer()

			for b.Loop() {
				if err := framer.WriteFrame(io.Discard, payload); err != nil {
					b.Fatalf("WriteFrame() error = %v", err)
				}
			}
		})

		b.Run(tc.name+"/sink", func(b *testing.B) {
			payload := benchmarkPayload(tc.payloadLen)
			framer := New(tc.payloadLen)
			writer := sinkWriter{buf: make([]byte, len(payload)+1)}

			b.ReportAllocs()
			b.SetBytes(int64(len(payload) + 1))
			b.ResetTimer()

			for b.Loop() {
				writer.Reset()
				if err := framer.WriteFrame(&writer, payload); err != nil {
					b.Fatalf("WriteFrame() error = %v", err)
				}
				if writer.n != len(payload)+1 {
					b.Fatalf("WriteFrame() wrote %d bytes, want %d", writer.n, len(payload)+1)
				}
			}
		})
	}
}

func benchmarkPayload(n int) []byte {
	payload := make([]byte, n)
	for i := range payload {
		payload[i] = byte('a' + i%26)
	}
	return payload
}

type sinkWriter struct {
	buf []byte
	n   int
}

func (w *sinkWriter) Write(p []byte) (int, error) {
	n := copy(w.buf[w.n:], p)
	w.n += n
	return n, nil
}

func (w *sinkWriter) Reset() {
	w.n = 0
}
