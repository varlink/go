package varlink

import (
	"bufio"
	"encoding/json"
)

type Method func(*Interface, ServerCall, *Writer) error

type InterfaceImpl struct {
	Name        string
	Description string
	Methods     map[string]Method
}

type Interface interface {
	Get() *InterfaceImpl
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
