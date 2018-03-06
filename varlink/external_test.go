package varlink_test

// test with no internal access

import (
	. "github.com/varlink/go/varlink"
	"testing"
	"time"
)

func TestRegisterService(t *testing.T) {
	newTestInterface := func() Interface {
		return Interface{
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
			Methods: map[string]struct{}{
				"TestMore":    {},
				"StopServing": {},
				"Ping":        {},
			},
		}
	}

	service := NewService(
		"Varlink Test",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)
	d := newTestInterface()

	if err := service.RegisterInterface(&d); err != nil {
		t.Fatal("Could register service")
	}

	if err := service.RegisterInterface(&d); err == nil {
		t.Fatal("Could register service twice")
	}

	go service.Run("unix:@testtesttest")

	time.Sleep(time.Second)

	n := newTestInterface()

	if err := service.RegisterInterface(&n); err == nil {
		t.Fatal("Could register service while running")
	}
	service.Stop()
}
