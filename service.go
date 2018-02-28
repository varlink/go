package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var OrgVarlinkService = `# The Varlink Service Interface is provided by every varlink service. It
# describes the service and the interfaces it implements.
interface org.varlink.service

# Get a list of all the interfaces a service provides and information
# about the implementation.
method GetInfo() -> (
  vendor: string,
  product: string,
  version: string,
  url: string,
  interfaces: string[]
)

# Get the description of an interface that is implemented by this service.
method GetInterfaceDescription(interface: string) -> (description: string)

# The requested interface was not found.
error InterfaceNotFound (interface: string)

# The requested method was not found
error MethodNotFound (method: string)

# The interface defines the requested method, but the service does not
# implement it.
error MethodNotImplemented (method: string)

# One of the passed parameters is invalid.
error InvalidParameter (parameter: string)
`

func keyList(mymap *map[string]*Interface) []string {
	keys := make([]string, len(*mymap))

	i := 0
	for k := range *mymap {
		keys[i] = k
		i++
	}
	return keys
}

func getInfo(iface *Interface, call ServerCall, out *Writer) error {
	type ReplyParameters struct {
		Vendor     string   `json:"vendor"`
		Product    string   `json:"product"`
		Version    string   `json:"version"`
		URL        string   `json:"url"`
		Interfaces []string `json:"interfaces"`
	}

	this, _ := (*iface).(Service)
	return out.Reply(ServerReply{
		Parameters: ReplyParameters{
			Vendor:     this.vendor,
			Product:    this.product,
			Version:    this.version,
			URL:        this.url,
			Interfaces: keyList(&this.services),
		},
	})
}

func getInterfaceDescription(iface *Interface, call ServerCall, out *Writer) error {
	type CallParameters struct {
		Name string `json:"interface"`
	}

	type ReplyParameters struct {
		InterfaceString string `json:"description"`
	}

	var in CallParameters
	err := json.Unmarshal(*call.Parameters, &in)
	if err != nil {
		return InvalidParameter("parameters", out)
	}

	this, _ := (*iface).(Service)
	iface, ok := this.services[in.Name]

	if !ok {
		return InvalidParameter("Name", out)
	}

	return out.Reply(ServerReply{
		Parameters: ReplyParameters{(*iface).Get().Description},
	})
}

func InterfaceNotFound(name string, out *Writer) error {
	type ReplyParameters struct {
		Name string `json:"interface"`
	}
	return out.Reply(ServerReply{
		Error:      "org.varlink.service.InterfaceNotFound",
		Parameters: ReplyParameters{Name: name},
	})
}

func MethodNotFound(name string, out *Writer) error {
	type ReplyParameters struct {
		Name string `json:"method"`
	}
	return out.Reply(ServerReply{
		Error:      "org.varlink.service.MethodNotFound",
		Parameters: ReplyParameters{Name: name},
	})
}

func MethodNotImplemented(name string, out *Writer) error {
	type ReplyParameters struct {
		Name string `json:"method"`
	}
	return out.Reply(ServerReply{
		Error:      "org.varlink.service.MethodNotImplemented",
		Parameters: ReplyParameters{Name: name},
	})
}

func InvalidParameter(parameter string, out *Writer) error {
	type ReplyParameters struct {
		Parameter string `json:"parameter"`
	}
	return out.Reply(ServerReply{
		Error:      "org.varlink.service.InvalidParameter",
		Parameters: ReplyParameters{Parameter: parameter},
	})
}

type Service struct {
	vendor    string
	product   string
	version   string
	url       string
	services  map[string]*Interface
	myservice InterfaceImpl
}

func (this Service) Get() *InterfaceImpl {
	return &this.myservice
}

func (this Service) registerInterface(iface Interface) {
	service := iface.Get()
	this.services[service.Name] = &iface
}

func (this Service) HandleMessage(writer *Writer, request []byte) error {
	var call ServerCall

	err := json.Unmarshal(request, &call)

	if err != nil {
		return err
	}

	r := strings.LastIndex(call.Method, ".")
	if r <= 0 {
		return InvalidParameter("method", writer)
	}

	interfacename := call.Method[:r]
	methodname := call.Method[r+1:]
	var iface *Interface
	iface, ok := this.services[interfacename]

	if !ok {
		return InterfaceNotFound(interfacename, writer)
	}

	service := (*iface).Get()
	method, ok := service.Methods[methodname]

	if !ok {
		return MethodNotFound(methodname, writer)
	}

	return method(iface, call, writer)
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

func (this Service) Run(address string) error {
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

	var l net.Listener
	l = activationListener()
	if l == nil {
		l, _ = net.Listen(protocol, addr)
	}
	defer l.Close()

	handleConnection := func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		writer := NewWriter(bufio.NewWriter(conn))

		for {
			request, err := reader.ReadBytes('\x00')
			if err != nil {
				break
			}

			err = this.HandleMessage(&writer, request[:len(request)-1])
			if err != nil {
				break
			}
		}
		conn.Close()
	}

	for {
		conn, _ := l.Accept()
		go handleConnection(conn)
	}
}

func NewService(vendor string, product string, version string, url string, ifaces []Interface) Service {
	r := Service{
		vendor:  vendor,
		product: product,
		version: version,
		url:     url,
		myservice: InterfaceImpl{
			Name:        "org.varlink.service",
			Description: OrgVarlinkService,
			Methods: map[string]Method{
				"GetInfo":                 getInfo,
				"GetInterfaceDescription": getInterfaceDescription,
			},
		},
		services: make(map[string]*Interface),
	}

	// Register ourselves
	r.registerInterface(Interface(r))
	for _, iface := range ifaces {
		r.registerInterface(iface)
	}
	return r
}
