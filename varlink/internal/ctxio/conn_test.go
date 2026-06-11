package ctxio_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/varlink/go/varlink/internal/ctxio"
)

func TestConn(t *testing.T) {
	l, err := net.Listen("tcp", ":")
	if err != nil {
		t.Fatalf("Unexpected error creating a listener: %v", err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := l.Accept()
		if err != nil {
			return
		}

		rd := bufio.NewReader(c)
		req, err := rd.ReadBytes('\n')
		if err != nil {
			t.Errorf("Failed to execute readFunc: %v", err)
			return
		}

		_, err = c.Write(append([]byte("Request received: "), req...))
		if err != nil {
			t.Errorf("Failed to execute writeFunc: %v", err)
			return
		}
	}()

	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}

	ctxC := ctxio.NewConn(c)

	_, err = ctxC.Write(context.Background(), []byte("hello world\n"))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	ret, err := ctxC.ReadBytes(context.Background(), '\n')
	if err != nil {
		t.Fatalf("Failed to read reply: %v", err)
	}

	want := []byte("Request received: hello world\n")
	if !bytes.Equal(ret, want) {
		t.Fatalf("Unexpected response: wanted %q, got %q", string(want), string(ret))
	}

	err = ctxC.Close()
	if err != nil {
		t.Fatalf("Failed to close ctx connection: %v", err)
	}

	err = l.Close()
	if err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}

	wg.Wait()
}

func TestBlockingWrite(t *testing.T) {
	cl, _ := net.Pipe()

	ctxC := ctxio.NewConn(cl)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Millisecond)
		cancel()
	}()
	_, err := ctxC.Write(ctx, []byte("hello world\n"))
	if err == nil {
		t.Fatal("Unexpectedly did not error")
	}
	if err != context.Canceled {
		t.Fatalf("Got unexpected error: %T, %s", err, err)
	}
}

func TestBlockingRead(t *testing.T) {
	cl, _ := net.Pipe()

	ctxC := ctxio.NewConn(cl)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Millisecond)
		cancel()
	}()
	_, err := ctxC.ReadBytes(ctx, '\n')
	if err == nil {
		t.Fatal("Unexpectedly did not error")
	}
	if err != context.Canceled {
		t.Fatalf("Got unexpected error: %T, %s", err, err)
	}
}

// TestReadBytesSplitDelimiter verifies a message split across multiple socket
// reads is reassembled, since the manual reader fills its buffer one chunk at a
// time rather than relying on bufio read-ahead.
func TestReadBytesSplitDelimiter(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()

	ctxC := ctxio.NewConn(cli)
	defer ctxC.Close()

	go func() {
		srv.Write([]byte("hello "))
		time.Sleep(5 * time.Millisecond)
		srv.Write([]byte("world\n"))
	}()

	got, err := ctxC.ReadBytes(context.Background(), '\n')
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if want := "hello world\n"; string(got) != want {
		t.Fatalf("ReadBytes = %q, want %q", got, want)
	}
}

// TestReadBytesLeftoverCarryover verifies bytes read past the delimiter are
// retained for the next ReadBytes call.
func TestReadBytesLeftoverCarryover(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()

	ctxC := ctxio.NewConn(cli)
	defer ctxC.Close()

	// Send two delimited messages plus the start of a third in a single write.
	go srv.Write([]byte("first\nsecond\nthi"))

	ctx := context.Background()
	for _, want := range []string{"first\n", "second\n"} {
		got, err := ctxC.ReadBytes(ctx, '\n')
		if err != nil {
			t.Fatalf("ReadBytes: %v", err)
		}
		if string(got) != want {
			t.Fatalf("ReadBytes = %q, want %q", got, want)
		}
	}
}

// TestReadBytesEOFWithPartial verifies that an EOF before the delimiter returns
// the bytes read so far together with io.EOF, matching bufio.Reader.ReadBytes.
func TestReadBytesEOFWithPartial(t *testing.T) {
	srv, cli := net.Pipe()

	ctxC := ctxio.NewConn(cli)
	defer ctxC.Close()

	go func() {
		srv.Write([]byte("partial"))
		srv.Close()
	}()

	got, err := ctxC.ReadBytes(context.Background(), '\n')
	if err != io.EOF {
		t.Fatalf("ReadBytes error = %v, want io.EOF", err)
	}
	if want := "partial"; string(got) != want {
		t.Fatalf("ReadBytes = %q, want %q", got, want)
	}
}

// TestReadBytesLargeMessage verifies a message larger than a single read chunk
// is reassembled, exercising the buffer-growth and scan-cursor paths across
// multiple direct reads.
func TestReadBytesLargeMessage(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()

	ctxC := ctxio.NewConn(cli)
	defer ctxC.Close()

	// 64 KiB of non-delimiter bytes followed by the delimiter, well past the
	// 8 KiB read chunk so the buffer must grow several times.
	payload := bytes.Repeat([]byte("x"), 64*1024)
	want := append(append([]byte{}, payload...), '\n')

	go func() {
		// Write in pieces so the reader performs many partial reads.
		for off := 0; off < len(want); off += 1500 {
			end := off + 1500
			if end > len(want) {
				end = len(want)
			}
			srv.Write(want[off:end])
		}
	}()

	got, err := ctxC.ReadBytes(context.Background(), '\n')
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ReadBytes len = %d, want %d (content mismatch)", len(got), len(want))
	}
}

// TestReadBytesManySmallMessages reads many small messages whose total size far
// exceeds any single read chunk, with a non-empty leftover carried across most
// pops. This exercises popMessage's front-reclamation slide on every pop; a
// broken slide/copy would corrupt or drop messages partway through the stream.
func TestReadBytesManySmallMessages(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()

	ctxC := ctxio.NewConn(cli)
	defer ctxC.Close()

	const numMessages = 5000
	want := make([][]byte, numMessages)
	var stream []byte
	for i := range want {
		msg := []byte(fmt.Sprintf("message-%d\n", i))
		want[i] = msg
		stream = append(stream, msg...)
	}

	go func() {
		// Feed the whole stream in modest pieces so each read pulls in several
		// complete messages plus a partial one (a persistent leftover).
		for off := 0; off < len(stream); off += 100 {
			end := off + 100
			if end > len(stream) {
				end = len(stream)
			}
			srv.Write(stream[off:end])
		}
	}()

	ctx := context.Background()
	for i, w := range want {
		got, err := ctxC.ReadBytes(ctx, '\n')
		if err != nil {
			t.Fatalf("ReadBytes message %d: %v", i, err)
		}
		if !bytes.Equal(got, w) {
			t.Fatalf("message %d: got %q, want %q", i, got, w)
		}
	}
}
