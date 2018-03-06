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
			Name: `org.example.more`,
			Description: `# Example service
interface org.example.more

# Enum, returning either start, progress or end
# progress: [0-100]
type State (
     start: bool,
     progress: int,
     end: bool
)

# Returns the same string
method Ping(ping : string) -> (pong: string)

# Dummy progress method
# n: number of progress steps
method TestMore(n : int) -> (state: State)

# Stop serving
method StopServing() -> ()

# Something failed
error ActionFailed (reason: string)`,
			Methods: varlink.MethodMap{
				"Ping":        nil,
				"TestMore":    nil,
				"StopServing": nil,
			},
		}
	}

	service := varlink.NewService(
		"Varlink Test",
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
		fmt.Println(err)
		t.Fatal("Couldn't register service")
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
