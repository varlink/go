//go:build unix

package varlink_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/varlink/go/varlink"
)

// fdTestInterface implements a tiny varlink interface that exercises fd passing.
//
// Supported methods:
//
//	org.example.fdtest.EchoFDs: reads RequestFDs, verifies content against expected
//	  (passed in params.Expected slice), replies with a fresh "pong" fd or, if
//	  params.ReplyContent is non-empty, one fd containing that content.
//
//	org.example.fdtest.MultiIn: reads N request fds, checks their content equals
//	  params.Expected[i], replies with zero fds.
type fdTestInterface struct {
	mu sync.Mutex
	// lastErr captures the first error seen in VarlinkDispatch for test assertions.
	lastErr error
}

func (i *fdTestInterface) setErr(err error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.lastErr == nil {
		i.lastErr = err
	}
}

func (i *fdTestInterface) getErr() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.lastErr
}

func (i *fdTestInterface) VarlinkGetName() string        { return "org.example.fdtest" }
func (i *fdTestInterface) VarlinkGetDescription() string { return "#" }

func (i *fdTestInterface) VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error {
	switch methodname {
	case "EchoFDs":
		// Params: {"expected":"<content>","replyContent":"<content>"}
		var params struct {
			Expected     string `json:"expected"`
			ReplyContent string `json:"replyContent"`
		}
		if err := call.GetParameters(&params); err != nil {
			return call.ReplyError(ctx, "org.example.fdtest.InvalidParam", nil)
		}

		fds := call.RequestFDs()
		defer func() {
			for _, f := range fds {
				f.Close()
			}
		}()

		if len(fds) != 1 {
			i.setErr(fmt.Errorf("EchoFDs: expected 1 fd, got %d", len(fds)))
			return call.ReplyError(ctx, "org.example.fdtest.FDCountMismatch", nil)
		}

		// Verify the content of the received fd.
		if _, err := fds[0].Seek(0, io.SeekStart); err != nil {
			i.setErr(err)
			return call.ReplyError(ctx, "org.example.fdtest.InternalError", nil)
		}
		data, err := io.ReadAll(fds[0])
		if err != nil {
			i.setErr(err)
			return call.ReplyError(ctx, "org.example.fdtest.InternalError", nil)
		}
		if string(data) != params.Expected {
			i.setErr(fmt.Errorf("EchoFDs: fd content %q, want %q", string(data), params.Expected))
		}

		// Reply with a fresh fd containing replyContent.
		replyContent := params.ReplyContent
		if replyContent == "" {
			replyContent = "pong"
		}
		replyFD := makeTempFD(ctx, replyContent)
		if replyFD == nil {
			i.setErr(fmt.Errorf("EchoFDs: failed to create reply fd"))
			return call.ReplyError(ctx, "org.example.fdtest.InternalError", nil)
		}
		defer replyFD.Close()

		return call.ReplyWithFDs(ctx, struct{}{}, []*os.File{replyFD})

	case "MultiIn":
		// Params: {"expected":["a","b","c"]}
		var params struct {
			Expected []string `json:"expected"`
		}
		if err := call.GetParameters(&params); err != nil {
			return call.ReplyError(ctx, "org.example.fdtest.InvalidParam", nil)
		}

		fds := call.RequestFDs()
		defer func() {
			for _, f := range fds {
				f.Close()
			}
		}()

		if len(fds) != len(params.Expected) {
			i.setErr(fmt.Errorf("MultiIn: expected %d fds, got %d", len(params.Expected), len(fds)))
			return call.ReplyError(ctx, "org.example.fdtest.FDCountMismatch", nil)
		}

		for idx, want := range params.Expected {
			if _, err := fds[idx].Seek(0, io.SeekStart); err != nil {
				i.setErr(err)
				return call.ReplyError(ctx, "org.example.fdtest.InternalError", nil)
			}
			got, err := io.ReadAll(fds[idx])
			if err != nil {
				i.setErr(err)
				return call.ReplyError(ctx, "org.example.fdtest.InternalError", nil)
			}
			if string(got) != want {
				i.setErr(fmt.Errorf("MultiIn fd[%d]: got %q, want %q", idx, string(got), want))
			}
		}

		return call.Reply(ctx, struct{}{})

	default:
		return call.ReplyMethodNotImplemented(ctx, methodname)
	}
}

// makeTempFD creates a temp file with content, seeks to start, and returns it.
// Returns nil on error (side-effect: logs nothing — caller handles).
func makeTempFD(_ context.Context, content string) *os.File {
	f, err := os.CreateTemp("", "varlink-fd-reply-*")
	if err != nil {
		return nil
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil
	}
	return f
}

// varlinkIface is the interface varlink dispatchers must satisfy.
type varlinkIface interface {
	VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error
	VarlinkGetName() string
	VarlinkGetDescription() string
}

// fdTestHelper creates a Service + listener on a unix socket, starts it in the
// background, and returns a client Connection along with a cancel func.
func fdTestHelper(t *testing.T, iface varlinkIface) (*varlink.Connection, func()) {
	t.Helper()
	dir := t.TempDir()
	addr := "unix:" + filepath.Join(dir, "test.sock")

	svc, err := varlink.NewService("Test", "FD Test", "1", "http://example.com")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := svc.RegisterInterface(iface); err != nil {
		t.Fatalf("RegisterInterface: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	svcErr := make(chan error, 1)
	go func() {
		svcErr <- svc.Listen(ctx, addr, 0)
	}()
	// Give the listener a moment to start.
	time.Sleep(20 * time.Millisecond)

	conn, err := varlink.NewConnection(context.Background(), addr)
	if err != nil {
		cancel()
		t.Fatalf("NewConnection: %v", err)
	}

	cleanup := func() {
		conn.Close()
		cancel()
		svc.Shutdown()
		<-svcErr
	}
	return conn, cleanup
}

// readFDContent reads the entire content of a file from its current position.
func readFDContent(t *testing.T, f *os.File) string {
	t.Helper()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(data)
}

// makeSendFD creates a temp file containing content for use as a send fd.
// The file is closed in t.Cleanup.
func makeSendFD(t *testing.T, content string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "send-fd-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// TestClientServiceFDRoundTrip sends a single "ping" fd to the service; the handler
// verifies its content and replies with a "pong" fd. The client verifies the reply fd.
func TestClientServiceFDRoundTrip(t *testing.T) {
	iface := &fdTestInterface{}
	conn, cleanup := fdTestHelper(t, iface)
	defer cleanup()

	sendFD := makeSendFD(t, "ping")

	type params struct {
		Expected     string `json:"expected"`
		ReplyContent string `json:"replyContent"`
	}
	type result struct{}

	var out result
	rxFDs, err := conn.CallWithFDs(
		context.Background(),
		"org.example.fdtest.EchoFDs",
		params{Expected: "ping", ReplyContent: "pong"},
		&out,
		[]*os.File{sendFD},
	)
	if err != nil {
		t.Fatalf("CallWithFDs: %v", err)
	}
	defer func() {
		for _, f := range rxFDs {
			f.Close()
		}
	}()

	if herr := iface.getErr(); herr != nil {
		t.Fatalf("handler error: %v", herr)
	}

	if len(rxFDs) != 1 {
		t.Fatalf("expected 1 reply fd, got %d", len(rxFDs))
	}

	if got := readFDContent(t, rxFDs[0]); got != "pong" {
		t.Errorf("reply fd content: got %q, want %q", got, "pong")
	}
}

// TestServiceMultipleFDsInOrder sends 3 fds; the handler checks order/content and replies 0 fds.
func TestServiceMultipleFDsInOrder(t *testing.T) {
	iface := &fdTestInterface{}
	conn, cleanup := fdTestHelper(t, iface)
	defer cleanup()

	contents := []string{"alpha", "beta", "gamma"}
	var sendFDs []*os.File
	for _, c := range contents {
		sendFDs = append(sendFDs, makeSendFD(t, c))
	}

	type params struct {
		Expected []string `json:"expected"`
	}
	type result struct{}

	var out result
	rxFDs, err := conn.CallWithFDs(
		context.Background(),
		"org.example.fdtest.MultiIn",
		params{Expected: contents},
		&out,
		sendFDs,
	)
	if err != nil {
		t.Fatalf("CallWithFDs: %v", err)
	}
	for _, f := range rxFDs {
		f.Close()
	}

	if herr := iface.getErr(); herr != nil {
		t.Fatalf("handler error: %v", herr)
	}

	if len(rxFDs) != 0 {
		t.Errorf("expected 0 reply fds, got %d", len(rxFDs))
	}
}

// TestZeroFDRegression verifies that ordinary Call/Reply (no fd API) over the unix
// service still works correctly when fd support is present.
func TestZeroFDRegression(t *testing.T) {
	iface := &fdTestInterface{}
	conn, cleanup := fdTestHelper(t, iface)
	defer cleanup()

	// Use plain Call (no fds) to invoke MultiIn with zero expected fds.
	type params struct {
		Expected []string `json:"expected"`
	}
	type result struct{}

	var out result
	err := conn.Call(
		context.Background(),
		"org.example.fdtest.MultiIn",
		params{Expected: []string{}},
		&out,
	)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if herr := iface.getErr(); herr != nil {
		t.Fatalf("handler error: %v", herr)
	}
}

// TestTCPRejectsFDs tests that a connection over TCP correctly rejects fd passing.
func TestTCPRejectsFDs(t *testing.T) {
	iface := &fdTestInterface{}

	svc, err := varlink.NewService("Test", "FD Test", "1", "http://example.com")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := svc.RegisterInterface(iface); err != nil {
		t.Fatalf("RegisterInterface: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer svc.Shutdown()

	svcErr := make(chan error, 1)
	go func() {
		svcErr <- svc.Listen(ctx, "tcp:127.0.0.1:0", 0)
	}()
	time.Sleep(30 * time.Millisecond)

	ln, err := svc.GetListener()
	if err != nil || ln == nil {
		t.Fatalf("GetListener: %v / %v", err, ln)
	}
	addr := "tcp:" + ln.Addr().String()

	conn, err := varlink.NewConnection(context.Background(), addr)
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}
	defer conn.Close()

	// SendWithFDs with 1 fd must return ErrFDsUnsupported.
	f := makeSendFD(t, "test")
	_, err = conn.SendWithFDs(
		context.Background(),
		"org.example.fdtest.MultiIn",
		nil,
		0,
		[]*os.File{f},
	)
	if !errors.Is(err, varlink.ErrFDsUnsupported) {
		t.Errorf("SendWithFDs over TCP with fd: expected ErrFDsUnsupported, got %v", err)
	}

	// Plain Call still works.
	type params struct {
		Expected []string `json:"expected"`
	}
	type result struct{}
	var out result
	if err := conn.Call(context.Background(), "org.example.fdtest.MultiIn", params{Expected: []string{}}, &out); err != nil {
		t.Errorf("plain Call over TCP: %v", err)
	}
}

// retainFDsInterface implements a tiny varlink interface whose handler claims
// the request fds via RequestFDs() but, unlike fdTestInterface, does not close
// them: it stashes them for the test to inspect after the call has returned.
// This exercises the case of a handler that wants to keep an fd alive beyond
// the lifetime of the call (e.g. to service it asynchronously).
type retainFDsInterface struct {
	mu       sync.Mutex
	retained []*os.File
}

func (i *retainFDsInterface) VarlinkGetName() string        { return "org.example.retainfds" }
func (i *retainFDsInterface) VarlinkGetDescription() string { return "#" }

func (i *retainFDsInterface) VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error {
	switch methodname {
	case "Retain":
		fds := call.RequestFDs()
		i.mu.Lock()
		i.retained = fds
		i.mu.Unlock()
		return call.Reply(ctx, struct{}{})
	default:
		return call.ReplyMethodNotImplemented(ctx, methodname)
	}
}

func (i *retainFDsInterface) getRetained() []*os.File {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.retained
}

// TestHandlerRetainedFDsSurviveDispatch verifies that fds claimed by a handler
// via RequestFDs() remain open and usable after VarlinkDispatch returns. The
// dispatcher interface passes Call by value, so a naive implementation that
// tracks "claimed" state only on that per-call copy would let the service's
// post-dispatch cleanup close fds the handler intended to keep.
func TestHandlerRetainedFDsSurviveDispatch(t *testing.T) {
	iface := &retainFDsInterface{}
	conn, cleanup := fdTestHelper(t, iface)
	defer cleanup()

	sendFD := makeSendFD(t, "keep-me")

	type result struct{}
	var out result
	if _, err := conn.CallWithFDs(context.Background(), "org.example.retainfds.Retain", struct{}{}, &out, []*os.File{sendFD}); err != nil {
		t.Fatalf("CallWithFDs: %v", err)
	}

	retained := iface.getRetained()
	if len(retained) != 1 {
		t.Fatalf("expected 1 retained fd, got %d", len(retained))
	}
	defer retained[0].Close()

	if got := readFDContent(t, retained[0]); got != "keep-me" {
		t.Errorf("retained fd content: got %q, want %q (fd may have been closed out from under the handler)", got, "keep-me")
	}
}

// TestLargeFDCount passes 64 fds to the handler, which verifies order and content.
func TestLargeFDCount(t *testing.T) {
	iface := &fdTestInterface{}
	conn, cleanup := fdTestHelper(t, iface)
	defer cleanup()

	const n = 64
	var contents []string
	var sendFDs []*os.File
	for i := 0; i < n; i++ {
		c := fmt.Sprintf("fd-content-%03d", i)
		contents = append(contents, c)
		sendFDs = append(sendFDs, makeSendFD(t, c))
	}

	type params struct {
		Expected []string `json:"expected"`
	}
	type result struct{}

	var out result
	rxFDs, err := conn.CallWithFDs(
		context.Background(),
		"org.example.fdtest.MultiIn",
		params{Expected: contents},
		&out,
		sendFDs,
	)
	if err != nil {
		t.Fatalf("CallWithFDs: %v", err)
	}
	for _, f := range rxFDs {
		f.Close()
	}

	if herr := iface.getErr(); herr != nil {
		t.Fatalf("handler error: %v", herr)
	}
}
