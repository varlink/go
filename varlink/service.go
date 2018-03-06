package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"
)

// ServiceIn represents the incoming message received by the Service from a Client.
type ServiceIn struct {
	Method     string           `json:"method"`
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	More       bool             `json:"more,omitempty"`
}

// ServiceOut represents the outgoing message sent by the service to a CLient.
type ServiceOut struct {
	Parameters interface{} `json:"parameters,omitempty"`
	Continues  bool        `json:"continues,omitempty"`
	Error      string      `json:"error,omitempty"`
}

func keyList(mymap *map[string]intf) []string {
	keys := make([]string, len(*mymap))

	i := 0
	for k := range *mymap {
		keys[i] = k
		i++
	}
	return keys
}

// Service represents an active varlink service. In addition to the custom varlink Interfaces, every service
// implements the org.varlink.service interface, which allows clients to retrieve information about the
// running service.
type Service struct {
	vendor     string
	product    string
	version    string
	url        string
	interfaces map[string]intf
	running    bool
}

// GetInfo returns information about the running service.
func (s *Service) getInfo(c Call) error {
	return c.Reply(&ServiceOut{
		Parameters: getInfo_Out{
			Vendor:     s.vendor,
			Product:    s.product,
			Version:    s.version,
			Url:        s.url,
			Interfaces: keyList(&s.interfaces),
		},
	})
}

// GetInterfaceDescription returns the varlink interface description of the given interface.
func (s *Service) getInterfaceDescription(c Call) error {
	var in getInterfaceDescription_In
	err := c.GetParameters(&in)
	if err != nil {
		return c.ReplyError("org.varlink.service.InvalidParameter", InvalidParameter_Error{Parameter: "interface"})
	}

	ifacep, ok := s.interfaces[in.Interface]
	ifacen := ifacep.(intf)
	if !ok {
		return c.ReplyError("org.varlink.service.InvalidParameter", InvalidParameter_Error{Parameter: "description"})
	}

	return c.Reply(&ServiceOut{
		Parameters: getInterfaceDescription_Out{ifacen.getDescription()},
	})
}

func (s *Service) handleMessage(writer *bufio.Writer, request []byte) error {
	var in ServiceIn

	err := json.Unmarshal(request, &in)

	if err != nil {
		return err
	}

	c := Call{writer: writer, in: &in}

	r := strings.LastIndex(in.Method, ".")
	if r <= 0 {
		return c.ReplyError("org.varlink.service.InvalidParameter", InvalidParameter_Error{Parameter: "method"})
	}

	interfacename := in.Method[:r]
	methodname := in.Method[r+1:]

	// Handle org.varlink.service calls
	if interfacename == "org.varlink.service" {
		switch methodname {
		case "GetInfo":
			return s.getInfo(c)

		case "GetInterfaceDescription":
			return s.getInterfaceDescription(c)

		default:
			return c.ReplyError("org.varlink.service.MethodNotFound", MethodNotFound_Error{Method: methodname})
		}
	}

	// Find the interface and method in our service
	iface, ok := s.interfaces[interfacename]
	if !ok {
		return c.ReplyError("org.varlink.service.InterfaceNotFound", InterfaceNotFound_Error{Interface: interfacename})
	}
	if !iface.isMethod(methodname) {
		return c.ReplyError("org.varlink.service.MethodNotFound", MethodNotFound_Error{Method: methodname})
	}

	// Dynamically find and call an implementation of the method in our module
	v := reflect.ValueOf(iface).MethodByName(methodname)
	if v.Kind() != reflect.Func {
		return c.ReplyError("org.varlink.service.MethodNotImplemented", MethodNotImplemented_Error{Method: methodname})
	}

	args := []reflect.Value{
		reflect.ValueOf(c),
	}
	ret := v.Call(args)

	if ret[0].Interface() == nil {
		return nil
	}

	return ret[0].Interface().(error)
}

func activationListener() net.Listener {
	defer os.Unsetenv("LISTEN_PID")
	defer os.Unsetenv("LISTEN_FDS")

	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil || pid != os.Getpid() {
		return nil
	}

	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil || nfds != 1 {
		return nil
	}

	syscall.CloseOnExec(3)

	file := os.NewFile(uintptr(3), "LISTEN_FD_3")
	listener, err := net.FileListener(file)
	if err != nil {
		return nil
	}
	return listener
}

// Stop stops a running Service.
func (s *Service) Stop() {
	s.running = false
}

// Run starts a Service.
func (s *Service) Run(address string) error {
	words := strings.SplitN(address, ":", 2)
	protocol := words[0]
	addr := words[1]

	// Ignore parameters after ';'
	words = strings.SplitN(addr, ";", 2)
	if words != nil {
		addr = words[0]
	}

	switch protocol {
	case "unix":
		if addr[0] != '@' {
			os.Remove(addr)
		}

	case "tcp":
		break

	default:
		return fmt.Errorf("Unknown protocol")
	}

	l := activationListener()
	if l == nil {
		var err error
		l, err = net.Listen(protocol, addr)
		if err != nil {
			return err
		}
	}

	endrunning := func() { s.running = false }
	defer l.Close()
	defer endrunning()

	s.running = true

	handleConnection := func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)

		for s.running {
			request, err := reader.ReadBytes('\x00')
			if err != nil {
				break
			}

			err = s.handleMessage(writer, request[:len(request)-1])
			if err != nil {
				break
			}
		}
		conn.Close()
		if !s.running {
			l.Close()
		}
	}

	for s.running {
		conn, err := l.Accept()
		if err != nil && s.running {
			return err
		}
		go handleConnection(conn)
	}

	return nil
}

// RegisterInterface registers a varlink.Interface containing struct to the Service
func (s *Service) RegisterInterface(iface intf) error {
	name := iface.getName()
	if _, ok := s.interfaces[name]; ok {
		return fmt.Errorf("interface '%s' already registered", name)
	}
	if s.running {
		return fmt.Errorf("service is already running")
	}
	s.interfaces[name] = iface
	return nil
}

// NewService creates a new Service which implements the list of given varlink interfaces.
func NewService(vendor string, product string, version string, url string) Service {
	s := Service{
		vendor:     vendor,
		product:    product,
		version:    version,
		url:        url,
		interfaces: make(map[string]intf),
	}

	// Every service has the org.varlink.service interface
	d := orgvarlinkserviceNew()
	s.RegisterInterface(&d)

	return s
}
