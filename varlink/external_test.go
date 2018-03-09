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
	service := varlink.NewService(
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

	go service.Run(fmt.Sprintf("unix:@varlinkexternal_test%d", os.Getpid()))

	time.Sleep(time.Second)

	n := new(VarlinkInterface2)

	if err := service.RegisterInterface(n); err == nil {
		t.Fatal("Could register service while running")
	}
	service.Stop()
}
