package varlink

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	ifaces := []Interface{
	// list own interface here
	}
	service := NewService(
		"Varlink Test",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
		ifaces,
	)
	var b bytes.Buffer
	c := Call{writer: bufio.NewWriter(&b)}
	err := service.handleMessage(c, []byte{0})
	if err == nil {
		t.Fatal("HandleMessage returned non-error")
	}
	msg := []byte(`{"method":"org.varlink.service.GetInfo"}`)
	err = service.handleMessage(c, msg)
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
