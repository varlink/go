package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
)

// Call defines a method call retrieved by a Service or sent by a Client.
type Call interface {
	WantMore() bool
	GetParameters(in interface{}) error
	Reply(out *ServiceOut) error
	ReplyError(name string, parameters interface{}) error
}

type serverCall struct {
	Call
	writer *bufio.Writer
	in     *ServiceIn
}

func (c *serverCall) WantsMore() bool {
	return c.in.More
}

func (c *serverCall) GetParameters(in interface{}) error {
	if c.in.Parameters == nil {
		return fmt.Errorf("Empty Parameters")
	}
	return json.Unmarshal(*c.in.Parameters, in)
}

func (c *serverCall) Reply(out *ServiceOut) error {
	b, e := json.Marshal(out)
	if e != nil {
		return e
	}

	b = append(b, 0)
	_, e = c.writer.Write(b)
	if e != nil {
		return e
	}
	return c.writer.Flush()
}

func (c *serverCall) ReplyError(name string, parameters interface{}) error {
	return c.Reply(&ServiceOut{
		Error:      name,
		Parameters: parameters,
	})
}
