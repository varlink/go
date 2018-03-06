// Package varlink implements the varlink protocol.
// See http://varlink.org for more information.
package varlink

type intf interface {
	getName() string
	getDescription() string
	isMethod(methodname string) bool
}

// Interface represents an active interface derived from a varlink interface description.
// An Interface for a varlink interface might be created by running varlink-generator which
// creates a .go file from a .varlink interface description file.
type Interface struct {
	Name        string
	Description string
	Methods     map[string]struct{}
}

func (d *Interface) getName() string {
	return d.Name
}

func (d *Interface) getDescription() string {
	return d.Description
}

func (d *Interface) isMethod(methodname string) bool {
	_, ok := d.Methods[methodname]
	return ok
}
