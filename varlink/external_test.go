package varlink_test

// test with no internal access

import (
	"fmt"
	"github.com/varlink/go/varlink"
	"os"
	"testing"
	"time"
)

type VarlinkInterface struct{}

func (s *VarlinkInterface) VarlinkDispatch(call varlink.Call, methodname string) error {
	return call.ReplyMethodNotImplemented(methodname)
}
func (s *VarlinkInterface) VarlinkGetName() string {
	return `org.example.test`
}

func (s *VarlinkInterface) VarlinkGetDescription() string {
	return "#"
}

type VarlinkInterface2 struct{}

func (s *VarlinkInterface2) VarlinkDispatch(call varlink.Call, methodname string) error {
	return call.ReplyMethodNotImplemented(methodname)
}
func (s *VarlinkInterface2) VarlinkGetName() string {
	return `org.example.test2`
}

func (s *VarlinkInterface2) VarlinkGetDescription() string {
	return "#"
}

func TestRegisterService(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service, _ := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}

	if err := service.RegisterInterface(newTestInterface); err == nil {
		t.Fatal("Could register service twice")
	}

	defer func() { service.Shutdown() }()

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen(fmt.Sprintf("unix:@varlinkexternal_TestRegisterService%d", os.Getpid()))
	}()

	time.Sleep(time.Second / 5)

	n := new(VarlinkInterface2)

	if err := service.RegisterInterface(n); err == nil {
		t.Fatal("Could register service while running")
	}
	time.Sleep(time.Second / 5)
	service.Shutdown()

	if err := <-servererror; err != nil {
		t.Fatalf("service.Listen(): %v", err)
	}
}

func TestUnix(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service, _ := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen(fmt.Sprintf("unix:varlinkexternal_TestUnix%d", os.Getpid()))
	}()

	time.Sleep(time.Second / 5)
	service.Shutdown()

	if err := <-servererror; err != nil {
		t.Fatalf("service.Listen(): %v", err)
	}
}

/*
func TestListen(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}
	os.Setenv("LISTEN_FDS", "foo")

	go func() { time.Sleep(time.Second); service.Stop() }()
	defer func() { time.Sleep(time.Second); service.Stop() }()
	err := service.Listen(fmt.Sprintf("unix:@varlinkexternal_TestListen%d", os.Getpid()))
	if err == nil {
		t.Fatalf("service.Listen() despite LISTEN_FDS set to `foo`")
	}
	t.Fatalf("service.Listen(): %v", err)

}
*/
