package varlink

import (
	"bufio"
	"encoding/json"
	"sort"
)

type Interface interface {
	GetName() string
	GetDescription() string
	IsMethod(methodname string) bool
	Handle(method string, in ServerCall, out *Writer) error
}

type InterfaceImpl struct {
	Name        string
	Description string
	Methods     []string
	sorted      bool
}

func (this *InterfaceImpl) GetName() string {
	return this.Name
}

func (this *InterfaceImpl) GetDescription() string {
	return this.Description
}

func (this *InterfaceImpl) IsMethod(methodname string) bool {
	if !this.sorted {
		sort.Strings(this.Methods)
		this.sorted = true
	}
	i := sort.Search(len(this.Methods),
		func(i int) bool { return this.Methods[i] >= methodname })
	if i < len(this.Methods) && this.Methods[i] == methodname {
		return true
	}
	return false
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

type Writer struct {
	writer *bufio.Writer
}

func NewWriter(writer *bufio.Writer) Writer {
	return Writer{writer}
}

func (this Writer) Reply(reply ServerReply) error {
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
