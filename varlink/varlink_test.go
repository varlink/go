package varlink

// tests with access to internals

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func expect(t *testing.T, expected string, returned string) {
	if strings.Compare(returned, expected) != 0 {
		t.Fatal(fmt.Sprintf("Expected(%d): `%s`\nGot(%d): `%s`\n",
			len(expected), expected,
			len(returned), returned))
	}
}

func TestService(t *testing.T) {
	service := NewService(
		"Varlink Test",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	t.Run("ZeroMessage", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		if err := service.handleMessage(w, []byte{0}); err == nil {
			t.Fatal("HandleMessage returned non-error")
		}
	})

	t.Run("Invalid json", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"foo.GetInterfaceDescription" fdgdfg}`)
		if err := service.handleMessage(w, msg); err == nil {
			t.Fatal("HandleMessage returned no error on invalid json")
		}
	})

	t.Run("WrongInterface", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"foo.GetInterfaceDescription"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatal("HandleMessage returned error on wrong interface")
		}
		expect(t, `{"parameters":{"interface":"foo"},"error":"org.varlink.service.InterfaceNotFound"}`+"\000",
			b.String())
	})

	t.Run("InvalidMethod", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"InvalidMethod"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatal("HandleMessage returned error on invalid method")
		}
		expect(t, `{"parameters":{"parameter":"method"},"error":"org.varlink.service.InvalidParameter"}`+"\000",
			b.String())
	})

	t.Run("WrongMethod", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.WrongMethod"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatal("HandleMessage returned error on wrong method")
		}
		expect(t, `{"parameters":{"method":"WrongMethod"},"error":"org.varlink.service.MethodNotFound"}`+"\000",
			b.String())
	})

	t.Run("GetInterfaceDescription", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{"interface":"org.varlink.service"}}`)
		if err := service.handleMessage(w, msg); err != nil {
			fmt.Println(err)
			t.Fatal("HandleMessage returned error")
		}
	})

	t.Run("GetInfo", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInfo"}`)
		if err := service.handleMessage(w, msg); err != nil {
			fmt.Println(err)
			t.Fatal("HandleMessage returned error")
		}
		expect(t, `{"parameters":{"vendor":"Varlink Test","product":"Varlink Test","version":"1","url":"https://github.com/varlink/go/varlink","interfaces":["org.varlink.service"]}}`+"\000",
			b.String())
	})

}
