package codec

import (
	"bytes"
	"testing"
)

func TestWireWriteReadMessage(t *testing.T) {
	wireOut := NewWire(bytes.NewReader(nil), &bytes.Buffer{}, 64, 1024)
	buf := wireOut.writer.(*bytes.Buffer)

	params, err := MarshalParameters(struct {
		Value string `json:"value"`
	}{Value: "hello"})
	if err != nil {
		t.Fatalf("MarshalParameters() error = %v", err)
	}

	msg := Message{
		Method:     "org.example.test.Ping",
		Parameters: params,
		More:       true,
	}
	if err := wireOut.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	wireIn := NewWire(bytes.NewReader(buf.Bytes()), ioDiscard{}, 64, 1024)
	got, err := wireIn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if got.Method != msg.Method || !got.More {
		t.Fatalf("ReadMessage() = %+v, want %+v", got, msg)
	}
	var decoded struct {
		Value string `json:"value"`
	}
	if err := DecodeParameters(got.Parameters, &decoded); err != nil {
		t.Fatalf("DecodeParameters() error = %v", err)
	}
	if decoded.Value != "hello" {
		t.Fatalf("DecodeParameters() value = %q, want hello", decoded.Value)
	}
}

func TestWireResetReader(t *testing.T) {
	var out bytes.Buffer
	wire := NewWire(bytes.NewReader([]byte(`{"method":"one"}`+"\x00")), &out, 64, 1024)

	msg, err := wire.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() first error = %v", err)
	}
	if msg.Method != "one" {
		t.Fatalf("ReadMessage() first method = %q", msg.Method)
	}

	wire.ResetReader(bytes.NewReader([]byte(`{"method":"two"}` + "\x00")))
	msg, err = wire.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() second error = %v", err)
	}
	if msg.Method != "two" {
		t.Fatalf("ReadMessage() second method = %q", msg.Method)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
