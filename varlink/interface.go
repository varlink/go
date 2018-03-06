// Package varlink implements the varlink protocol.
// See http://varlink.org for more information.
package varlink

// Interface defines a varlink interface.
type intf interface {
	GetName() string
	GetDescription() string
	IsMethod(methodname string) bool
}

// Interface represents an active interface derived from a varlink interface description.
// An Interface for a varlink interface might be created by running varlink-generator which
// creates a .go file from a .varlink interface description file.
type Interface struct {
	Name        string
	Description string
	Methods     map[string]struct{}
}

// GetName returns the reverse-domain varlink interface name.
func (d *Interface) GetName() string {
	return d.Name
}

// GetDescription returns the interface description. The interface description can be retrieved from
// the running service by calling org.varlink.service.GetInterfaceDescription().
func (d *Interface) GetDescription() string {
	return d.Description
}

// IsMethod indicates if the given method name is defined in the varlink interface description.
func (d *Interface) IsMethod(methodname string) bool {
	_, ok := d.Methods[methodname]
	return ok
}
