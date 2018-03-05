package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
)

// Call is a method call retrieved by a Service or sent by a Client.
type Call struct {
	writer *bufio.Writer
	in     *ServiceIn
}

// WantsMore indicates if the clients accepts more than one reply.
func (c *Call) WantsMore() bool {
	return c.in.More
}

// GetParameters retrieves the method call parameters.
func (c *Call) GetParameters(in interface{}) error {
	if c.in.Parameters == nil {
		return fmt.Errorf("Empty Parameters")
	}
	return json.Unmarshal(*c.in.Parameters, in)
}

// Reply sends a reply to a method call.
func (c *Call) Reply(out *ServiceOut) error {
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

// ReplyError sends an error reply to a method call.
func (c *Call) ReplyError(name string, parameters interface{}) error {
	return c.Reply(&ServiceOut{
		Error:      name,
		Parameters: parameters,
	})
}
