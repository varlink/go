package varlink_test

// test with no internal access

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/varlink/go/varlink"
)

type VarlinkInterface struct{}

func (s *VarlinkInterface) VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error {
	return call.ReplyMethodNotImplemented(ctx, methodname)
}
func (s *VarlinkInterface) VarlinkGetName() string {
	return `org.example.test`
}

func (s *VarlinkInterface) VarlinkGetDescription() string {
	return "#"
}

type VarlinkInterface2 struct{}

func (s *VarlinkInterface2) VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error {
	return call.ReplyMethodNotImplemented(ctx, methodname)
}
func (s *VarlinkInterface2) VarlinkGetName() string {
	return `org.example.test2`
}

func (s *VarlinkInterface2) VarlinkGetDescription() string {
	return "#"
}

func TestInvalidAddress(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service, err := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}

	if err = service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("RegisterInterface(): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err = service.Listen(ctx, "foo", 0); err == nil {
		t.Fatalf("service.Listen() should error")
	}

	if err = service.Listen(ctx, "", 0); err == nil {
		t.Fatalf("service.Listen() should error")
	}

	if err = service.Listen(ctx, "unix", 0); err == nil {
		t.Fatalf("service.Listen() should error")
	}
}

func TestListenFDSNotInt(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service, err := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}
	os.Setenv("LISTEN_FDS", "foo")
	os.Setenv("LISTEN_PID", fmt.Sprint(os.Getpid()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen(ctx, "unix:varlinkexternal_TestListenFDSNotInt", 0)
	}()

	time.Sleep(time.Second / 5)
	service.Shutdown()

	err = <-servererror

	if err != nil {
		t.Fatalf("service.Run(): %v", err)
	}
}
