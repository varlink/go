package varlink_test

// test with no internal access

import (
	"fmt"
	"github.com/varlink/go/varlink"
	"os"
	"testing"
	"time"
)

func testFunc(c varlink.Call) error {
	return nil
}

func TestRegisterService(t *testing.T) {
	newTestInterface := func() varlink.Interface {
		return varlink.Interface{
			Name:        `org.example.more`,
			Description: `#`,
			Methods: varlink.MethodMap{
				"Ping":        nil,
				"TestMore":    nil,
				"StopServing": nil,
			},
		}
	}

	service := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	d := newTestInterface()

	m := varlink.MethodMap{
		"TestMore":    testFunc,
		"StopServing": testFunc,
		"Ping":        testFunc,
	}

	if err := service.RegisterInterface(&d, m); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}

	if err := service.RegisterInterface(&d, m); err == nil {
		t.Fatal("Could register service twice")
	}

	go service.Run(fmt.Sprintf("unix:@varlinkexternal_test%d", os.Getpid()))

	time.Sleep(time.Second)

	n := newTestInterface()

	if err := service.RegisterInterface(&n, m); err == nil {
		t.Fatal("Could register service while running")
	}
	service.Stop()
}

func TestRegisterWrongMethod(t *testing.T) {
	newTestInterface := func() varlink.Interface {
		return varlink.Interface{
			Name:        `org.example.more`,
			Description: `#`,
			Methods: varlink.MethodMap{
				"Ping":        nil,
				"TestMore":    nil,
				"StopServing": nil,
			},
		}
	}

	service := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	d := newTestInterface()

	m := varlink.MethodMap{
		"Foo": testFunc,
	}

	if err := service.RegisterInterface(&d, m); err == nil {
		t.Fatal("Could add method not part of interface")
	}
}
