// Package varlink implements the varlink protocol.
// See http://varlink.org for more information.
package varlink

type method func(c Call) error
type MethodMap map[string]method

type intf interface {
	getName() string
	getDescription() string
	getMethod(name string) (method, bool)
	addMethods(methods MethodMap) error
}

// Interface represents an active interface derived from a varlink interface description.
// An Interface for a varlink interface might be created by running varlink-generator which
// creates a .go file from a .varlink interface description file.
type Interface struct {
	Name        string
	Description string
	methods     MethodMap
}

func (d *Interface) getName() string {
	return d.Name
}

func (d *Interface) getDescription() string {
	return d.Description
}

func (d *Interface) addMethods(methods MethodMap) error {
	d.methods = methods
	return nil
}

func (d *Interface) getMethod(name string) (method, bool) {
	val, ok := d.methods[name]
	return val, ok
}
