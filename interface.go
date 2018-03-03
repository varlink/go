package varlink

type Interface interface {
	GetName() string
	GetDescription() string
	IsMethod(methodname string) bool
}

type InterfaceDefinition struct {
	Interface
	Name        string
	Description string
	Methods     map[string]struct{}
}

func (d *InterfaceDefinition) GetName() string {
	return d.Name
}

func (d *InterfaceDefinition) GetDescription() string {
	return d.Description
}

func (d *InterfaceDefinition) IsMethod(methodname string) bool {
	_, ok := d.Methods[methodname]
	return ok
}
