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

func keyList(mymap *map[string]Interface) []string {
	keys := make([]string, len(*mymap))

	i := 0
	for k := range *mymap {
		keys[i] = k
		i++
	}
	return keys
}

type Service struct {
	InterfaceDefinition
	vendor   string
	product  string
	version  string
	url      string
	services map[string]Interface
	quit     bool
}

func (this *Service) GetInfo(ctx Context) error {
	type ReplyParameters struct {
		Vendor     string   `json:"vendor"`
		Product    string   `json:"product"`
		Version    string   `json:"version"`
		URL        string   `json:"url"`
		Interfaces []string `json:"interfaces"`
	}

	return ctx.Reply(&ServerOut{
		Parameters: ReplyParameters{
			Vendor:     this.vendor,
			Product:    this.product,
			Version:    this.version,
			URL:        this.url,
			Interfaces: keyList(&this.services),
		},
	})
}

func (this *Service) GetInterfaceDescription(ctx Context) error {
	type CallParameters struct {
		Name string `json:"interface"`
	}

	type ReplyParameters struct {
		InterfaceString string `json:"description"`
	}

	var in CallParameters
	err := ctx.Args(&in)
	if err != nil {
		return InvalidParameter(ctx, "parameters")
	}

	ifacep, ok := this.services[in.Name]
	ifacen := ifacep.(Interface)
	if !ok {
		return InvalidParameter(ctx, "Name")
	}

	return ctx.Reply(&ServerOut{
		Parameters: ReplyParameters{ifacen.GetDescription()},
	})
}

func (this *Service) registerInterface(iface Interface) {
	name := iface.GetName()
	this.services[name] = iface
}

func (this *Service) HandleMessage(ctx ContextImpl, request []byte) error {
	var call ServerIn

	err := json.Unmarshal(request, &call)

	if err != nil {
		return err
	}
	ctx.call = &call
	r := strings.LastIndex(call.Method, ".")
	if r <= 0 {
		return InvalidParameter(&ctx, "method")
	}

	interfacename := call.Method[:r]
	methodname := call.Method[r+1:]
	_, ok := this.services[interfacename]

	if !ok {
		return InterfaceNotFound(&ctx, interfacename)
	}
	if !this.services[interfacename].IsMethod(methodname) {
		return MethodNotFound(&ctx, methodname)
	}

	v := reflect.ValueOf(this.services[interfacename]).MethodByName(methodname)
	if v.Kind() != reflect.Func {
		return MethodNotImplemented(&ctx, methodname)
	}

	args := []reflect.Value{
		reflect.ValueOf(&ctx),
	}
	ret := v.Call(args)

	if ret[0].Interface() == nil {
		return nil
	}

	return ret[0].Interface().(error)
}

func IsActivated() bool {
	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil || pid != os.Getpid() {
		return false
	}

	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil || nfds != 1 {
		return false
	}
	return true
}

func activationListener() net.Listener {
	defer os.Unsetenv("LISTEN_PID")
	defer os.Unsetenv("LISTEN_FDS")

	if !IsActivated() {
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

func (this *Service) Stop() {
	this.quit = true
}

func (this *Service) Run(address string) error {
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
		var err error
		l, err = net.Listen(protocol, addr)
		if err != nil {
			return err
		}
	}
	defer l.Close()
	this.quit = false

	handleConnection := func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		context := ContextImpl{writer: bufio.NewWriter(conn)}

		for !this.quit {
			request, err := reader.ReadBytes('\x00')
			if err != nil {
				break
			}

			err = this.HandleMessage(context, request[:len(request)-1])
			if err != nil {
				break
			}
		}
		conn.Close()
		if this.quit {
			l.Close()
		}
	}

	for !this.quit {
		conn, err := l.Accept()
		if err != nil && !this.quit {
			return err
		}
		go handleConnection(conn)
	}

	return nil
}

func NewService(vendor string, product string, version string, url string, ifaces []Interface) Service {
	r := Service{
		InterfaceDefinition: InterfaceDefinition{
			Name:        "org.varlink.service",
			Description: OrgVarlinkService,
			Methods:     map[string]bool{"GetDescription": true, "GetInfo": true},
		},
		vendor:   vendor,
		product:  product,
		version:  version,
		url:      url,
		services: make(map[string]Interface),
	}

	// Register ourselves
	r.registerInterface(&r)
	for _, iface := range ifaces {
		r.registerInterface(iface)
	}
	return r
}
