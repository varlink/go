package varlink

import (
	"bufio"
	"encoding/json"
)

type Interface interface {
	GetName() string
	GetDescription() string
	IsMethod(methodname string) bool
}

type InterfaceDefinition struct {
	Name        string
	Description string
	Methods     map[string]bool
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

type ServerCall struct {
	Method     string           `json:"method"`
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	More       bool             `json:"more,omitempty"`
}

type ServerReply struct {
	Parameters interface{} `json:"parameters,omitempty"`
	Continues  bool        `json:"continues,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type ClientCall struct {
	Method     string      `json:"method"`
	Parameters interface{} `json:"parameters,omitempty"`
	More       bool        `json:"more,omitempty"`
}

type ClientReply struct {
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	Continues  bool             `json:"continues,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type Context interface {
	WantMore() bool
	Args(in interface{}) error
	Reply(reply *ServerReply) error
}

type ContextImpl struct {
	Context
	writer *bufio.Writer
	call   *ServerCall
}

func (this *ContextImpl) WantMore() bool {
	return this.call.More
}

func (this *ContextImpl) Args(in interface{}) error {
	if this.call.Parameters == nil {
		return InvalidParameter(this, "parameters")
	}
	err := json.Unmarshal(*this.call.Parameters, in)
	if err != nil {
		return InvalidParameter(this, "parameters")
	}
	return err
}

func (this *ContextImpl) Reply(reply *ServerReply) error {
	b, e := json.Marshal(reply)
	if e != nil {
		return e
	}

	b = append(b, 0)
	_, e = this.writer.Write(b)
	if e != nil {
		return e
	}
	return this.writer.Flush()
}
