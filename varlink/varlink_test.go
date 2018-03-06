package varlink

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService(
		"Varlink Test",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	err := service.handleMessage(w, []byte{0})
	if err == nil {
		t.Fatal("HandleMessage returned non-error")
	}
	msg := []byte(`{"method":"org.varlink.service.GetInfo"}`)
	err = service.handleMessage(w, msg)
	if err != nil {
		fmt.Println(err)
		t.Fatal("HandleMessage returned error")
	}
	returned := b.String()
	const expected = `{"parameters":{"vendor":"Varlink Test","product":"Varlink Test","version":"1","url":"https://github.com/varlink/go/varlink","interfaces":["org.varlink.service"]}}` + "\000"
	if strings.Compare(returned, expected) != 0 {
		fmt.Println("Expected: \"" + expected + "\"")
		fmt.Printf("Expected len: %d\n", len(expected))
		fmt.Println("Got:      \"" + returned + "\"")
		fmt.Printf("Got len: %d\n", len(returned))
		t.Fatal("HandleMessage return value differs")
	}
}

func TestRegisterService(t *testing.T) {
	newTestInface := func() Interface {
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
	d := orgvarlinkserviceNew()

	if err := service.RegisterInterface(&d); err == nil {
		t.Fatal("Could register service twice")
	}
	service.running = true
	n := newTestInface()

	if err := service.RegisterInterface(&n); err == nil {
		t.Fatal("Could register service while running")
	}
}
