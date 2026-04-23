package codec

import (
	"bytes"
	"testing"
)

func BenchmarkWireReadMessage(b *testing.B) {
	cases := []struct {
		name       string
		payloadLen int
		bufferSize int
	}{
		{name: "small/buf64", payloadLen: 64, bufferSize: 64},
		{name: "medium/buf4k", payloadLen: 4 * 1024, bufferSize: 4096},
		{name: "large/buf4k", payloadLen: 64 * 1024, bufferSize: 4096},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			msg := benchmarkMessage(tc.payloadLen)
			var encoded bytes.Buffer
			maxFrameSize := tc.payloadLen*4 + 1024
			wireOut := NewWire(bytes.NewReader(nil), &encoded, tc.bufferSize, maxFrameSize)
			if err := wireOut.WriteMessage(msg); err != nil {
				b.Fatalf("WriteMessage() setup error = %v", err)
			}

			source := bytes.NewReader(nil)
			wire := NewWire(source, ioDiscard{}, tc.bufferSize, maxFrameSize)

			b.ReportAllocs()
			b.SetBytes(int64(encoded.Len()))
			b.ResetTimer()

			for b.Loop() {
				source.Reset(encoded.Bytes())
				wire.ResetReader(source)
				got, err := wire.ReadMessage()
				if err != nil {
					b.Fatalf("ReadMessage() error = %v", err)
				}
				if got.Method != msg.Method {
					b.Fatalf("ReadMessage() method = %q, want %q", got.Method, msg.Method)
				}
			}
		})
	}
}

func BenchmarkWireWriteMessage(b *testing.B) {
	cases := []struct {
		name       string
		payloadLen int
	}{
		{name: "small", payloadLen: 64},
		{name: "medium", payloadLen: 4 * 1024},
		{name: "large", payloadLen: 64 * 1024},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			msg := benchmarkMessage(tc.payloadLen)
			wire := NewWire(bytes.NewReader(nil), ioDiscard{}, 4096, tc.payloadLen*4+1024)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if err := wire.WriteMessage(msg); err != nil {
					b.Fatalf("WriteMessage() error = %v", err)
				}
			}
		})
	}
}

func benchmarkMessage(payloadLen int) Message {
	params, err := MarshalParameters(struct {
		Value string `json:"value"`
	}{Value: string(benchmarkPayload(payloadLen))})
	if err != nil {
		panic(err)
	}
	return Message{
		Method:     "org.example.bench.Ping",
		Parameters: params,
	}
}
