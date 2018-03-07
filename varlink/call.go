package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
)

// Call is a method call retrieved by a Service. The connection from the
// client can be terminated by returning an error from the call instead
// of sending a reply or error reply.
type Call struct {
	writer *bufio.Writer
	in     *ServiceCall
}

// WantsMore indicates if the calling client accepts more than one reply.
func (c *Call) WantsMore() bool {
	return c.in.More
}

// GetParameters retrieves the method call parameters.
func (c *Call) GetParameters(p interface{}) error {
	if c.in.Parameters == nil {
		return fmt.Errorf("Empty Parameters")
	}
	return json.Unmarshal(*c.in.Parameters, p)
}

// Reply sends a reply to this method call.
func (c *Call) Reply(r *ServiceReply) error {
	b, e := json.Marshal(r)
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

// ReplyError sends an error reply to this method call.
func (c *Call) ReplyError(name string, parameters interface{}) error {
	return c.Reply(&ServiceReply{
		Error:      name,
		Parameters: parameters,
	})
}
