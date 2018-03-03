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

func (this *InterfaceDefinition) GetName() string {
	return this.Name
}

func (this *InterfaceDefinition) GetDescription() string {
	return this.Description
}

func (this *InterfaceDefinition) IsMethod(methodname string) bool {
	_, ok := this.Methods[methodname]
	return ok
}
