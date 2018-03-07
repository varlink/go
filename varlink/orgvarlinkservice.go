// Generated with varlink-generator -- https://github.com/varlink/go/

package varlink

type getInfo_Out struct {
	Vendor     string   `json:"vendor,omitempty"`
	Product    string   `json:"product,omitempty"`
	Version    string   `json:"version,omitempty"`
	Url        string   `json:"url,omitempty"`
	Interfaces []string `json:"interfaces,omitempty"`
}

type getInterfaceDescription_In struct {
	Interface string `json:"interface"`
}

type getInterfaceDescription_Out struct {
	Description string `json:"description,omitempty"`
}

type interfaceNotFound_Error struct {
	Interface string `json:"interface,omitempty"`
}

type methodNotFound_Error struct {
	Method string `json:"method,omitempty"`
}

type methodNotImplemented_Error struct {
	Method string `json:"method,omitempty"`
}

type invalidParameter_Error struct {
	Parameter string `json:"parameter,omitempty"`
}

func orgvarlinkserviceNew() Interface {
	return Interface{
		Name: `org.varlink.service`,
		Description: `# The Varlink Service Interface is provided by every varlink service. It
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
error InvalidParameter (parameter: string)`,
		Methods: MethodMap{
			"GetInfo":                 nil,
			"GetInterfaceDescription": nil,
		},
	}
}
