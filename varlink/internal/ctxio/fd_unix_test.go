//go:build unix

package ctxio_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/varlink/go/varlink/internal/ctxio"
)

// fdWithContent creates a temporary file containing data, seeking back to the start.
func fdWithContent(t *testing.T, data string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fd-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(data); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	return f
}

// assertFDContent reads all bytes from f starting at offset 0, compares to want, then closes f.
func assertFDContent(t *testing.T, f *os.File, want string) {
	t.Helper()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	f.Close()
	if string(got) != want {
		t.Errorf("fd content: got %q, want %q", string(got), want)
	}
}

// makeUnixPair creates a connected pair of *ctxio.Conn backed by unix sockets.
func makeUnixPair(t *testing.T) (client, server *ctxio.Conn) {
	t.Helper()
	dir := t.TempDir()
	addr := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	var (
		serverConn net.Conn
		wg         sync.WaitGroup
		acceptErr  error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		serverConn, acceptErr = ln.Accept()
	}()

	clientRaw, err := net.Dial("unix", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	wg.Wait()

	if acceptErr != nil {
		t.Fatalf("Accept: %v", acceptErr)
	}

	return ctxio.NewConn(clientRaw), ctxio.NewConn(serverConn)
}

// makeUnixConnPair returns a connected pair of *net.UnixConn via socketpair(2).
// This is used in tests that need to inject bytes at the syscall level.
func makeUnixConnPair(t *testing.T) (writer, reader *net.UnixConn) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("Socketpair: %v", err)
	}
	wf := os.NewFile(uintptr(fds[0]), "socketpair-w")
	rf := os.NewFile(uintptr(fds[1]), "socketpair-r")

	wconn, err := net.FileConn(wf)
	wf.Close()
	if err != nil {
		t.Fatalf("FileConn writer: %v", err)
	}
	rconn, err := net.FileConn(rf)
	rf.Close()
	if err != nil {
		t.Fatalf("FileConn reader: %v", err)
	}
	return wconn.(*net.UnixConn), rconn.(*net.UnixConn)
}

func TestFDRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		msg    string
		fdData []string // content of each fd to send
	}{
		{
			name:   "single fd",
			msg:    `{"method":"test"}`,
			fdData: []string{"hello"},
		},
		{
			name:   "three fds in order",
			msg:    `{"method":"test"}`,
			fdData: []string{"aaa", "bbb", "ccc"},
		},
		{
			name:   "zero fds",
			msg:    `{"method":"test"}`,
			fdData: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, server := makeUnixPair(t)
			defer client.Close()
			defer server.Close()

			ctx := context.Background()
			msgBytes := append([]byte(tc.msg), 0)

			// Build fd slice.
			var sendFDs []*os.File
			for _, data := range tc.fdData {
				sendFDs = append(sendFDs, fdWithContent(t, data))
			}
			defer func() {
				for _, f := range sendFDs {
					f.Close()
				}
			}()

			// Write from client.
			_, err := client.WriteWithFDs(ctx, msgBytes, sendFDs)
			if err != nil {
				t.Fatalf("WriteWithFDs: %v", err)
			}

			// Read on server.
			gotMsg, gotFDs, err := server.ReadBytesWithFDs(ctx, 0)
			if err != nil {
				t.Fatalf("ReadBytesWithFDs: %v", err)
			}
			defer func() {
				for _, f := range gotFDs {
					f.Close()
				}
			}()

			// Verify message bytes.
			if string(gotMsg) != string(msgBytes) {
				t.Errorf("message: got %q, want %q", gotMsg, msgBytes)
			}

			// Verify fd count.
			if len(gotFDs) != len(tc.fdData) {
				t.Fatalf("fd count: got %d, want %d", len(gotFDs), len(tc.fdData))
			}

			// Verify fd contents.
			for i, want := range tc.fdData {
				assertFDContent(t, gotFDs[i], want)
			}
		})
	}
}

func TestFDFramingCarryover(t *testing.T) {
	client, server := makeUnixPair(t)
	defer client.Close()
	defer server.Close()

	ctx := context.Background()

	msg1 := append([]byte(`{"method":"first"}`), 0)
	msg2 := append([]byte(`{"method":"second"}`), 0)

	fd1 := fdWithContent(t, "msg1-fd")
	fd2 := fdWithContent(t, "msg2-fd")
	defer fd1.Close()
	defer fd2.Close()

	// Write both messages back-to-back before reading.
	if _, err := client.WriteWithFDs(ctx, msg1, []*os.File{fd1}); err != nil {
		t.Fatalf("WriteWithFDs msg1: %v", err)
	}
	if _, err := client.WriteWithFDs(ctx, msg2, []*os.File{fd2}); err != nil {
		t.Fatalf("WriteWithFDs msg2: %v", err)
	}

	// Read first message.
	gotMsg1, gotFDs1, err := server.ReadBytesWithFDs(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytesWithFDs msg1: %v", err)
	}
	if string(gotMsg1) != string(msg1) {
		t.Errorf("msg1 bytes: got %q, want %q", gotMsg1, msg1)
	}
	if len(gotFDs1) != 1 {
		t.Fatalf("msg1 fd count: got %d, want 1", len(gotFDs1))
	}
	assertFDContent(t, gotFDs1[0], "msg1-fd")

	// Read second message.
	gotMsg2, gotFDs2, err := server.ReadBytesWithFDs(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytesWithFDs msg2: %v", err)
	}
	if string(gotMsg2) != string(msg2) {
		t.Errorf("msg2 bytes: got %q, want %q", gotMsg2, msg2)
	}
	if len(gotFDs2) != 1 {
		t.Fatalf("msg2 fd count: got %d, want 1", len(gotFDs2))
	}
	assertFDContent(t, gotFDs2[0], "msg2-fd")
}

// TestFDFramingCoalesced is the regression test for fd misattribution under
// receive-side coalescing.
//
// An fd-LESS message followed by an fd-BEARING message — each sent as its own
// sendmsg, exactly as WriteWithFDs emits them — can be delivered to the receiver
// in a SINGLE recvmsg: [msg1\x00 + msg2\x00] with msg2's fds attached. The fds
// must be attributed to msg2 (which carried them), not msg1 (which happens to
// contain the recvmsg's first byte).
//
// This is the exact case where keying the fd batch to the recvmsg's FIRST byte
// is wrong: the kernel ends its receive loop right after attaching an skb's fds,
// so the fd-bearing skb's bytes always end at the recvmsg's LAST byte, which is
// the only anchor that reliably lands in the owning message.
//
// Strategy: raw socketpair, two separate WriteMsgUnix calls (msg1 without oob,
// msg2 with oob), relying on the kernel coalescing them into one ReadMsgUnix.
func TestFDFramingCoalesced(t *testing.T) {
	writerRaw, readerRaw := makeUnixConnPair(t)
	defer writerRaw.Close()
	defer readerRaw.Close()

	// Wrap only the reader side in a ctxio.Conn.
	server := ctxio.NewConn(readerRaw)
	defer server.Close()

	// The fd travels with msg2.
	fdB := fdWithContent(t, "coalesced-fd")
	defer fdB.Close()

	msg1 := append([]byte(`{"method":"first"}`), 0)
	msg2 := append([]byte(`{"method":"second"}`), 0)
	oob := syscall.UnixRights(int(fdB.Fd()))

	// msg1 with no fds, then msg2 with fds — two sendmsgs the kernel may coalesce
	// into one recvmsg. Send msg1 first so the reader has not yet drained it.
	if _, _, err := writerRaw.WriteMsgUnix(msg1, nil, nil); err != nil {
		t.Fatalf("WriteMsgUnix msg1: %v", err)
	}
	if _, _, err := writerRaw.WriteMsgUnix(msg2, oob, nil); err != nil {
		t.Fatalf("WriteMsgUnix msg2: %v", err)
	}

	ctx := context.Background()

	// Read first message — must get NO fds (it carried none).
	gotMsg1, gotFDs1, err := server.ReadBytesWithFDs(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytesWithFDs msg1: %v", err)
	}
	defer func() {
		for _, f := range gotFDs1 {
			f.Close()
		}
	}()
	if string(gotMsg1) != string(msg1) {
		t.Errorf("msg1 bytes: got %q, want %q", gotMsg1, msg1)
	}
	if len(gotFDs1) != 0 {
		t.Fatalf("msg1 fd count: got %d, want 0 (fd misattributed to first message!)", len(gotFDs1))
	}

	// Read second message — must get the fd it was sent with.
	gotMsg2, gotFDs2, err := server.ReadBytesWithFDs(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytesWithFDs msg2: %v", err)
	}
	defer func() {
		for _, f := range gotFDs2 {
			f.Close()
		}
	}()
	if string(gotMsg2) != string(msg2) {
		t.Errorf("msg2 bytes: got %q, want %q", gotMsg2, msg2)
	}
	if len(gotFDs2) != 1 {
		t.Fatalf("msg2 fd count: got %d, want 1", len(gotFDs2))
	}
	assertFDContent(t, gotFDs2[0], "coalesced-fd")
}

// TestFDFramingCoalescedSplit exercises case (b): a message split across two
// recvmsgs, where fds arrive with the first recvmsg.  The assembled message
// must receive the fds.
func TestFDFramingCoalescedSplit(t *testing.T) {
	writerRaw, readerRaw := makeUnixConnPair(t)
	defer writerRaw.Close()
	defer readerRaw.Close()

	server := ctxio.NewConn(readerRaw)
	defer server.Close()

	fdA := fdWithContent(t, "split-fd")
	defer fdA.Close()

	// Send the first half of the message WITH fds attached.
	half1 := []byte(`{"method":"spl`)
	oob := syscall.UnixRights(int(fdA.Fd()))
	n, _, err := writerRaw.WriteMsgUnix(half1, oob, nil)
	if err != nil {
		t.Fatalf("WriteMsgUnix half1: %v", err)
	}
	if n != len(half1) {
		t.Fatalf("WriteMsgUnix half1 short: %d/%d", n, len(half1))
	}

	// Send the second half WITHOUT fds.
	half2 := append([]byte(`it"}`), 0)
	n, _, err = writerRaw.WriteMsgUnix(half2, nil, nil)
	if err != nil {
		t.Fatalf("WriteMsgUnix half2: %v", err)
	}
	if n != len(half2) {
		t.Fatalf("WriteMsgUnix half2 short: %d/%d", n, len(half2))
	}

	ctx := context.Background()
	gotMsg, gotFDs, err := server.ReadBytesWithFDs(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytesWithFDs: %v", err)
	}
	defer func() {
		for _, f := range gotFDs {
			f.Close()
		}
	}()

	want := append([]byte(`{"method":"split"}`), 0)
	if string(gotMsg) != string(want) {
		t.Errorf("msg bytes: got %q, want %q", gotMsg, want)
	}
	if len(gotFDs) != 1 {
		t.Fatalf("split msg fd count: got %d, want 1", len(gotFDs))
	}
	assertFDContent(t, gotFDs[0], "split-fd")
}

func TestFDContextCancel(t *testing.T) {
	client, server := makeUnixPair(t)
	defer client.Close()
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Don't write anything; the read should block until context is cancelled.

	done := make(chan error, 1)
	go func() {
		_, _, err := server.ReadBytesWithFDs(ctx, 0)
		done <- err
	}()

	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestNonUnixRejectsFDs(t *testing.T) {
	// net.Pipe() returns *net.pipe, not *net.UnixConn; CanPassFDs should be false.
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	conn := ctxio.NewConn(c1)
	_ = ctxio.NewConn(c2)

	if conn.CanPassFDs() {
		t.Fatal("CanPassFDs should be false on net.Pipe()")
	}

	f := fdWithContent(t, "test")
	defer f.Close()

	ctx := context.Background()

	// WriteWithFDs with 1 fd must return ErrFDsUnsupported.
	_, err := conn.WriteWithFDs(ctx, []byte("hello"), []*os.File{f})
	if !errors.Is(err, ctxio.ErrFDsUnsupported) {
		t.Errorf("expected ErrFDsUnsupported with 1 fd, got %v", err)
	}

	// WriteWithFDs with 0 fds must work like a normal write (c2 will read it).
	go func() {
		buf := make([]byte, 5)
		c2.Read(buf)
	}()
	_, err = conn.WriteWithFDs(ctx, []byte("hello"), nil)
	if err != nil {
		t.Errorf("WriteWithFDs with 0 fds should succeed: %v", err)
	}
}

// TestMixedReadPathsShareBuffer verifies that ReadBytes and ReadBytesWithFDs
// share the same leftover buffer on a single unix connection, so a plain
// ReadBytes that over-reads past its delimiter does not strand the following
// message's bytes from the fd path.
//
// A raw socketpair with a single WriteMsgUnix guarantees both messages coalesce
// into one buffer fill, so the shared-buffer property is exercised
// deterministically rather than relying on kernel scheduling.
//
// Note this only protects bytes: a plain read(2) cannot recover SCM_RIGHTS
// ancillary data, so callers must still use ReadBytesWithFDs consistently on an
// fd-capable connection (which both connection.go and service.go do). Here the
// fd is attached to a separate message read entirely through the fd path.
func TestMixedReadPathsShareBuffer(t *testing.T) {
	writerRaw, readerRaw := makeUnixConnPair(t)
	defer writerRaw.Close()
	defer readerRaw.Close()

	server := ctxio.NewConn(readerRaw)
	defer server.Close()

	ctx := context.Background()

	msg1 := append([]byte(`{"method":"first"}`), 0)
	msg2 := append([]byte(`{"method":"second"}`), 0)

	// One sendmsg delivers msg1+msg2 as a single contiguous buffer (no fds), so
	// the receiver gets both messages' bytes in one ReadMsgUnix/Read.
	combined := append(append([]byte{}, msg1...), msg2...)
	if n, _, err := writerRaw.WriteMsgUnix(combined, nil, nil); err != nil {
		t.Fatalf("WriteMsgUnix: %v", err)
	} else if n != len(combined) {
		t.Fatalf("WriteMsgUnix short write: %d/%d", n, len(combined))
	}

	// The plain path reads msg1 and necessarily buffers msg2's bytes.
	got1, err := server.ReadBytes(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytes msg1: %v", err)
	}
	if string(got1) != string(msg1) {
		t.Fatalf("msg1: got %q, want %q", got1, msg1)
	}

	// The fd path must find msg2's bytes via the shared buffer without another
	// read from the socket.
	got2, fds2, err := server.ReadBytesWithFDs(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytesWithFDs msg2: %v", err)
	}
	if string(got2) != string(msg2) {
		t.Fatalf("msg2: got %q, want %q", got2, msg2)
	}
	if len(fds2) != 0 {
		t.Fatalf("msg2 fd count: got %d, want 0", len(fds2))
	}

	// A subsequent fd-bearing message read entirely through the fd path still
	// delivers its descriptor.
	msg3 := append([]byte(`{"method":"third"}`), 0)
	fd3 := fdWithContent(t, "third-fd")
	defer fd3.Close()
	oob := syscall.UnixRights(int(fd3.Fd()))
	if _, _, err := writerRaw.WriteMsgUnix(msg3, oob, nil); err != nil {
		t.Fatalf("WriteMsgUnix msg3: %v", err)
	}
	got3, fds3, err := server.ReadBytesWithFDs(ctx, 0)
	if err != nil {
		t.Fatalf("ReadBytesWithFDs msg3: %v", err)
	}
	if string(got3) != string(msg3) {
		t.Fatalf("msg3: got %q, want %q", got3, msg3)
	}
	if len(fds3) != 1 {
		t.Fatalf("msg3 fd count: got %d, want 1", len(fds3))
	}
	assertFDContent(t, fds3[0], "third-fd")
}

// TestFDRandomFragmentation stress-tests fd passing against arbitrary kernel
// fragmentation. Many messages, each carrying a random set of fds (including
// none), are written back-to-back with random delays. Because the messages flow
// over a real unix socket the kernel is free to coalesce several into one
// recvmsg or split one across several. The reader must recover, for every
// message, both the exact payload AND exactly its own descriptors (right count
// and right contents) — i.e. the fd->message association must survive whatever
// the kernel does to the byte stream.
func TestFDRandomFragmentation(t *testing.T) {
	const (
		delim        = 0
		numMessages  = 150
		maxFDsPerMsg = 4
	)

	client, server := makeUnixPair(t)
	defer client.Close()
	defer server.Close()

	rng := rand.New(rand.NewSource(1)) // fixed seed for reproducibility

	// Pre-build every message and the unique content for each of its fds. Using
	// globally-unique fd contents lets the reader detect any cross-message fd
	// mix-up, not just a wrong count.
	type message struct {
		payload []byte   // without delimiter
		fdData  []string // expected fd contents, in order
	}
	messages := make([]message, numMessages)
	for i := range messages {
		payload := make([]byte, rng.Intn(12*1024)) // 0..12 KiB, spans the read chunk
		for j := range payload {
			payload[j] = byte(1 + rng.Intn(255)) // never the delimiter
		}
		nfds := rng.Intn(maxFDsPerMsg + 1)
		fdData := make([]string, nfds)
		for k := range fdData {
			fdData[k] = fmt.Sprintf("msg%d-fd%d-%d", i, k, rng.Int())
		}
		messages[i] = message{payload: payload, fdData: fdData}
	}

	ctx := context.Background()

	dir := t.TempDir()
	var writeErr error
	go func() {
		// This runs off the test goroutine, so it must not call t.Fatalf; route
		// any failure through writeErr instead.
		for _, m := range messages {
			fds := make([]*os.File, 0, len(m.fdData))
			for _, data := range m.fdData {
				f, ferr := os.CreateTemp(dir, "fd-*")
				if ferr == nil {
					_, ferr = f.WriteString(data)
				}
				if ferr == nil {
					_, ferr = f.Seek(0, io.SeekStart)
				}
				if ferr != nil {
					writeErr = ferr
					return
				}
				fds = append(fds, f)
			}
			buf := append(append([]byte{}, m.payload...), delim)
			_, werr := client.WriteWithFDs(ctx, buf, fds)
			// The library never takes ownership of the sent fds; close our copies.
			for _, f := range fds {
				f.Close()
			}
			if werr != nil {
				writeErr = werr
				return
			}
			if rng.Intn(3) == 0 {
				time.Sleep(time.Duration(rng.Intn(300)) * time.Microsecond)
			}
		}
	}()

	for i, want := range messages {
		gotMsg, gotFDs, err := server.ReadBytesWithFDs(ctx, delim)
		if err != nil {
			t.Fatalf("message %d: ReadBytesWithFDs: %v", i, err)
		}
		if n := len(gotMsg); n == 0 || gotMsg[n-1] != delim {
			t.Fatalf("message %d: missing trailing delimiter (len=%d)", i, n)
		}
		if got := gotMsg[:len(gotMsg)-1]; string(got) != string(want.payload) {
			t.Fatalf("message %d: payload mismatch (got %d bytes, want %d)", i, len(got), len(want.payload))
		}
		if len(gotFDs) != len(want.fdData) {
			t.Fatalf("message %d: fd count got %d, want %d", i, len(gotFDs), len(want.fdData))
		}
		for k, wantData := range want.fdData {
			assertFDContent(t, gotFDs[k], wantData) // closes the fd
		}
	}

	if writeErr != nil {
		t.Fatalf("writer: %v", writeErr)
	}
}

// TestReadBytesWithFDsClosesQueuedFDsOnEOF verifies that fds attached to a
// still-incomplete message (no delimiter seen yet) are closed as soon as
// ReadBytesWithFDs gives up on the connection (EOF here), rather than being
// stranded until some later Close() call that a direct caller of
// ReadBytesWithFDs might not make promptly, or at all.
//
// The pipe write-end fd is used as an observable proxy for "did the library
// close its dup": as long as any dup of the write end is open anywhere, reads
// from the read end block; once every dup is closed, reads see EOF.
func TestReadBytesWithFDsClosesQueuedFDsOnEOF(t *testing.T) {
	writerRaw, readerRaw := makeUnixConnPair(t)
	defer writerRaw.Close()

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	defer pipeR.Close()

	// Send a partial message (no delimiter) carrying pipeW as an SCM_RIGHTS fd,
	// then close both our local pipeW and the socket itself before ever sending
	// a delimiter. The server's dup of pipeW is now the only thing that can
	// still be keeping the pipe's write side open.
	oob := syscall.UnixRights(int(pipeW.Fd()))
	if _, _, err := writerRaw.WriteMsgUnix([]byte("partial-no-delim"), oob, nil); err != nil {
		t.Fatalf("WriteMsgUnix: %v", err)
	}
	pipeW.Close()
	writerRaw.Close()

	server := ctxio.NewConn(readerRaw)
	// Deliberately do not defer server.Close() here: the point of this test is
	// that fds must be closed by ReadBytesWithFDs itself on this error path,
	// not by a subsequent Close().

	ctx := context.Background()
	_, _, err = server.ReadBytesWithFDs(ctx, 0)
	if err == nil {
		t.Fatal("ReadBytesWithFDs: expected an error at EOF, got nil")
	}

	if err := pipeR.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := make([]byte, 1)
	n, rerr := pipeR.Read(buf)
	if n != 0 || rerr != io.EOF {
		t.Fatalf("pipe read after ReadBytesWithFDs error: got (%d, %v), want (0, io.EOF); "+
			"the server's dup of the pipe write-end fd was leaked instead of closed", n, rerr)
	}
}
