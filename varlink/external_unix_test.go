// +build !windows

package varlink_test

// test with no internal access

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/varlink/go/varlink"
)

func TestRegisterService(t *testing.T) {
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

	if err := service.RegisterInterface(newTestInterface); err == nil {
		t.Fatal("Could register service twice")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() { service.Shutdown() }()

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen(ctx, "unix:varlinkexternal_TestRegisterService", 0)
	}()

	time.Sleep(time.Second / 5)

	n := new(VarlinkInterface2)

	if err := service.RegisterInterface(n); err == nil {
		time.Sleep(time.Second / 5)
		service.Shutdown()

		if err := <-servererror; err != nil {
			t.Fatalf("service.Listen(): %v", err)
		}

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
		t.Fatalf("RegisterInterface(): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen(ctx, "unix:varlinkexternal_TestUnix", 0)
	}()

	time.Sleep(time.Second / 5)
	service.Shutdown()

	if err := <-servererror; err != nil {
		t.Fatalf("service.Listen(): %v", err)
	}
}

func TestAnonUnix(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

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
		t.Fatalf("RegisterInterface(): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen(ctx, "unix:@varlinkexternal_TestAnonUnix", 0)
	}()

	time.Sleep(time.Second / 5)
	service.Shutdown()

	if err := <-servererror; err != nil {
		t.Fatalf("service.Listen(): %v", err)
	}
}

