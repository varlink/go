/*
Package varlink provides varlink client and server implementations.

Example varlink interface definition in a org.example.this.varlink file:
	interface org.example.this

	method Ping(in: string) -> (out: string)

Generated Go module in a orgexamplethis/orgexamplethis.go file:
	// Generated with varlink-generator -- https://github.com/varlink/go/cmd/varlink-generator
	package orgexamplethis

	import "github.com/varlink/go/varlink"

	type orgexamplethisInterface interface {
		Ping(c VarlinkCall, in string) error
	}

	type VarlinkCall struct{ varlink.Call }

	func (c *VarlinkCall) ReplyPing(out string) error {
		var out struct {
			Out string `json:"out,omitempty"`
		}
		out.Out = out
		return c.Reply(&out)
	}

	func (s *VarlinkInterface) Ping(c VarlinkCall, in string) error {
		return c.ReplyMethodNotImplemented("Ping")
	}

	[...]

Example service:
	import ("orgexamplethis")

	data := Data{data: "test"}

	service, _ = varlink.NewService(
	        "Example",
	        "This",
	        "1",
	         "https://example.org/this",
	)

	service.RegisterInterface(orgexample.VarlinkNew(&m))
	err := service.Listen("tcp:0.0.0.0", 0)

See http://varlink.org for more information about varlink.
*/
package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type dispatcher interface {
	VarlinkDispatch(c Call, methodname string) error
	VarlinkGetName() string
	VarlinkGetDescription() string
}

type serviceCall struct {
	Method     string           `json:"method"`
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	More       bool             `json:"more,omitempty"`
	OneShot    bool             `json:"oneshot,omitempty"`
}

type serviceReply struct {
	Parameters interface{} `json:"parameters,omitempty"`
	Continues  bool        `json:"continues,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Service represents an active varlink service. In addition to the registered custom varlink Interfaces, every service
// implements the org.varlink.service interface which allows clients to retrieve information about the
// running service.
type Service struct {
	vendor       string
	product      string
	version      string
	url          string
	interfaces   map[string]dispatcher
	names        []string
	descriptions map[string]string
	running      bool
	listener     net.Listener
	conncounter  int64
	sync.Mutex
}

func (s *Service) getInfo(c Call) error {
	return c.replyGetInfo(s.vendor, s.product, s.version, s.url, s.names)
}

func (s *Service) getInterfaceDescription(c Call, name string) error {
	if name == "" {
		return c.ReplyInvalidParameter("interface")
	}

	description, ok := s.descriptions[name]
	if !ok {
		return c.ReplyInvalidParameter("interface")
	}

	return c.replyGetInterfaceDescription(description)
}

func (s *Service) handleMessage(writer *bufio.Writer, request []byte) error {
	var in serviceCall

	err := json.Unmarshal(request, &in)

	if err != nil {
		return err
	}

	c := Call{
		writer: writer,
		in:     &in,
	}

	r := strings.LastIndex(in.Method, ".")
	if r <= 0 {
		return c.ReplyInvalidParameter("method")
	}

	interfacename := in.Method[:r]
	methodname := in.Method[r+1:]

	if interfacename == "org.varlink.service" {
		return s.orgvarlinkserviceDispatch(c, methodname)
	}

	// Find the interface and method in our service
	iface, ok := s.interfaces[interfacename]
	if !ok {
		return c.ReplyInterfaceNotFound(interfacename)
	}

	return iface.VarlinkDispatch(c, methodname)
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

// Shutdown shuts down the listener of a running service.
func (s *Service) Shutdown() {
	s.running = false
	s.Lock()
	if s.listener != nil {
		s.listener.Close()
	}
	s.Unlock()
}

func (s *Service) handleConnection(conn net.Conn, wg *sync.WaitGroup) {
	defer func() { s.Lock(); s.conncounter--; s.Unlock(); wg.Done() }()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		request, err := reader.ReadBytes('\x00')
		if err != nil {
			break
		}

		err = s.handleMessage(writer, request[:len(request)-1])
		if err != nil {
			// FIXME: report error
			//fmt.Fprintf(os.Stderr, "handleMessage: %v", err)
			break
		}
	}

	conn.Close()
}

func (s *Service) teardown() {
	s.Lock()
	s.listener = nil
	s.Unlock()
}

// Listen starts a Service.
func (s *Service) Listen(address string, timeout time.Duration) error {
	defer func() { s.running = false }()
	s.running = true

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
		break
	case "tcp":
		break

	default:
		return fmt.Errorf("Unknown protocol")
	}

	defer s.teardown()

	l := activationListener()
	if l == nil {
		if protocol == "unix" && addr[0] != '@' {
			os.Remove(addr)
		}

		var err error
		l, err = net.Listen(protocol, addr)
		if err != nil {
			return err
		}

		if protocol == "unix" && addr[0] != '@' {
			l.(*net.UnixListener).SetUnlinkOnClose(true)
		}
	}

	var wg sync.WaitGroup
	defer func() { wg.Wait() }()

	s.Lock()
	s.listener = l
	s.Unlock()

	for s.running {
		if timeout != 0 {
			switch protocol {
			case "unix":
				if err := l.(*net.UnixListener).SetDeadline(time.Now().Add(timeout)); err != nil {
					return err
				}

			case "tcp":
				if err := l.(*net.TCPListener).SetDeadline(time.Now().Add(timeout)); err != nil {
					return err
				}
			}
		}
		conn, err := l.Accept()
		if err != nil {
			if err.(net.Error).Timeout() {
				var last bool
				s.Lock()
				if s.conncounter == 0 {
					last = true
				}
				s.Unlock()
				if last {
					return nil
				}
				continue
			}
			if !s.running {
				return nil
			}
			return err
		}
		s.Lock()
		s.conncounter++
		s.Unlock()
		wg.Add(1)
		go s.handleConnection(conn, &wg)
	}

	return nil
}

// RegisterInterface registers a varlink.Interface containing struct to the Service
func (s *Service) RegisterInterface(iface dispatcher) error {
	name := iface.VarlinkGetName()
	if _, ok := s.interfaces[name]; ok {
		return fmt.Errorf("interface '%s' already registered", name)
	}

	if s.running {
		return fmt.Errorf("service is already running")
	}
	s.interfaces[name] = iface
	s.descriptions[name] = iface.VarlinkGetDescription()
	s.names = append(s.names, name)

	return nil
}

// NewService creates a new Service which implements the list of given varlink interfaces.
func NewService(vendor string, product string, version string, url string) (*Service, error) {
	s := Service{
		vendor:       vendor,
		product:      product,
		version:      version,
		url:          url,
		interfaces:   make(map[string]dispatcher),
		descriptions: make(map[string]string),
	}
	err := s.RegisterInterface(orgvarlinkserviceNew())

	return &s, err
}
