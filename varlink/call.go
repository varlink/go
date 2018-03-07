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
	in     *serviceCall
}

// WantsMore indicates if the calling client accepts more than one reply.
func (c *Call) WantsMore() bool {
	return c.in.More
}

// IsOneShot indicates if the calling client does not expect a reply.
func (c *Call) IsOneShot() bool {
	return c.in.OneShot
}

// GetParameters retrieves the method call parameters.
func (c *Call) GetParameters(p interface{}) error {
	if c.in.Parameters == nil {
		return fmt.Errorf("empty parameters")
	}
	return json.Unmarshal(*c.in.Parameters, p)
}

func (c *Call) sendMessage(r *serviceReply) error {
	if c.in.OneShot {
		return nil
	}

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

// Reply sends a reply to this method call.
func (c *Call) Reply(parameters interface{}) error {
	return c.sendMessage(&serviceReply{
		Parameters: parameters,
	})
}

// ReplyContinues sends a reply to this method call. The caller asked with the "more"
// flag, this reply carries the "continues" flag.
func (c *Call) ReplyContinues(parameters interface{}) error {
	if !c.in.More {
		return fmt.Errorf("call did not set more, it does not expect continues")
	}

	return c.sendMessage(&serviceReply{
		Continues: true,
		Parameters: parameters,
	})
}

// ReplyError sends an error reply to this method call.
func (c *Call) ReplyError(name string, parameters interface{}) error {
	return c.sendMessage(&serviceReply{
		Error:      name,
		Parameters: parameters,
	})
}
