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

type serviceCall struct {
	Call
	writer *bufio.Writer
	in     *ServiceIn
}

func (c *serviceCall) WantsMore() bool {
	return c.in.More
}

func (c *serviceCall) GetParameters(in interface{}) error {
	if c.in.Parameters == nil {
		return fmt.Errorf("Empty Parameters")
	}
	return json.Unmarshal(*c.in.Parameters, in)
}

func (c *serviceCall) Reply(out *ServiceOut) error {
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

func (c *serviceCall) ReplyError(name string, parameters interface{}) error {
	return c.Reply(&ServiceOut{
		Error:      name,
		Parameters: parameters,
	})
}
