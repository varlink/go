package varlink

// tests with access to internals

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func expect(t *testing.T, expected string, returned string) {
	if strings.Compare(returned, expected) != 0 {
		t.Fatalf("Expected(%d): `%s`\nGot(%d): `%s`\n",
			len(expected), expected,
			len(returned), returned)
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

	t.Run("GetInterfaceDescriptionNoInterface", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{}}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"parameter":"interface"},"error":"org.varlink.service.InvalidParameter"}`+"\000",
			b.String())
	})

	t.Run("GetInterfaceDescriptionWrongInterface", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{"interface":"foo"}}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"parameter":"interface"},"error":"org.varlink.service.InvalidParameter"}`+"\000",
			b.String())
	})

	t.Run("GetInterfaceDescription", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{"interface":"org.varlink.service"}}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"description":"# The Varlink Service Interface is provided by every varlink service. It\n# describes the service and the interfaces it implements.\ninterface org.varlink.service\n\n# Get a list of all the interfaces a service provides and information\n# about the implementation.\nmethod GetInfo() -\u003e (\n  vendor: string,\n  product: string,\n  version: string,\n  url: string,\n  interfaces: string[]\n)\n\n# Get the description of an interface that is implemented by this service.\nmethod GetInterfaceDescription(interface: string) -\u003e (description: string)\n\n# The requested interface was not found.\nerror InterfaceNotFound (interface: string)\n\n# The requested method was not found\nerror MethodNotFound (method: string)\n\n# The interface defines the requested method, but the service does not\n# implement it.\nerror MethodNotImplemented (method: string)\n\n# One of the passed parameters is invalid.\nerror InvalidParameter (parameter: string)"}}`+"\000",
			b.String())
	})

	t.Run("GetInfo", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInfo"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"vendor":"Varlink Test","product":"Varlink Test","version":"1","url":"https://github.com/varlink/go/varlink","interfaces":["org.varlink.service"]}}`+"\000",
			b.String())
	})
}

func TestMoreService(t *testing.T) {
	testFunc := func(c Call) error {
		return nil
	}

	newTestInterface := func() Interface {
		return Interface{
			Name:        `org.example.more`,
			Description: `#`,
			Methods: MethodMap{
				"Ping":        nil,
				"TestMore":    nil,
				"StopServing": nil,
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

	m := MethodMap{
		"TestMore":    testFunc,
		"StopServing": testFunc,
	}

	if err := service.RegisterInterface(&d, m); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}

	t.Run("MethodNotImplemented", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.example.more.Ping"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"method":"Ping"},"error":"org.varlink.service.MethodNotImplemented"}`+"\000",
			b.String())
	})
}
