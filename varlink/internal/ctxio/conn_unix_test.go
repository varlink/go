//go:build unix

package ctxio_test

import (
	"bytes"
	"context"
	"math/rand"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/varlink/go/varlink/internal/ctxio"
)

// socketPair returns a connected pair of stream net.Conns backed by an anonymous
// AF_UNIX socketpair(2). No filesystem path or listener is involved.
func socketPair(t *testing.T) (a, b net.Conn) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("Socketpair: %v", err)
	}
	mkConn := func(fd int, name string) net.Conn {
		f := os.NewFile(uintptr(fd), name)
		conn, err := net.FileConn(f)
		f.Close() // FileConn dups the fd; close our copy.
		if err != nil {
			t.Fatalf("FileConn %s: %v", name, err)
		}
		return conn
	}
	return mkConn(fds[0], "sp-a"), mkConn(fds[1], "sp-b")
}

// TestReadBytesRandomFragmentation stress-tests the framer against arbitrary
// fragmentation: a stream of delimited messages is written in random-sized
// chunks with random delays over an anonymous socketpair, and the reader must
// recover every message intact and in order. This exercises delimiters landing
// at chunk boundaries, multiple messages coalescing into one read, and messages
// straddling many reads.
func TestReadBytesRandomFragmentation(t *testing.T) {
	const delim = 0

	writerRaw, readerRaw := socketPair(t)
	defer writerRaw.Close()
	defer readerRaw.Close()

	rng := rand.New(rand.NewSource(1)) // fixed seed for reproducibility

	// Build random messages whose payloads never contain the delimiter, then
	// the wire stream as payload+delim repeated.
	const numMessages = 200
	messages := make([][]byte, numMessages)
	var stream []byte
	for i := range messages {
		msg := make([]byte, rng.Intn(20*1024)) // 0..20 KiB, spanning the 8 KiB chunk
		for j := range msg {
			msg[j] = byte(1 + rng.Intn(255)) // 1..255, never the delimiter
		}
		messages[i] = msg
		stream = append(stream, msg...)
		stream = append(stream, delim)
	}

	var writeErr error
	go func() {
		for off := 0; off < len(stream); {
			n := 1 + rng.Intn(4096) // random fragment size
			if off+n > len(stream) {
				n = len(stream) - off
			}
			if _, werr := writerRaw.Write(stream[off : off+n]); werr != nil {
				writeErr = werr
				return
			}
			off += n
			if rng.Intn(4) == 0 {
				time.Sleep(time.Duration(rng.Intn(200)) * time.Microsecond)
			}
		}
	}()

	ctxC := ctxio.NewConn(readerRaw)

	ctx := context.Background()
	for i, want := range messages {
		got, rerr := ctxC.ReadBytes(ctx, delim)
		if rerr != nil {
			t.Fatalf("ReadBytes message %d: %v", i, rerr)
		}
		// ReadBytes returns the payload including the trailing delimiter.
		if n := len(got); n == 0 || got[n-1] != delim {
			t.Fatalf("message %d: missing trailing delimiter (len=%d)", i, n)
		}
		if !bytes.Equal(got[:len(got)-1], want) {
			t.Fatalf("message %d: got %d bytes, want %d (content mismatch)", i, len(got)-1, len(want))
		}
	}

	if writeErr != nil {
		t.Fatalf("writer: %v", writeErr)
	}
}
