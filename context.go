package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
)

type Context interface {
	WantMore() bool
	Parameters(in interface{}) error
	Reply(out *ServerOut) error
	ReplyError(name string, parameters interface{}) error
}

type ContextImpl struct {
	Context
	writer *bufio.Writer
	call   *ServerIn
}

func (this *ContextImpl) WantMore() bool {
	return this.call.More
}

func (this *ContextImpl) Parameters(in interface{}) error {
	if this.call.Parameters == nil {
		return fmt.Errorf("Empty Parameters")
	}
	return json.Unmarshal(*this.call.Parameters, in)
}

func (this *ContextImpl) Reply(out *ServerOut) error {
	b, e := json.Marshal(out)
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

func (this *ContextImpl) ReplyError(name string, parameters interface{}) error {
	return this.Reply(&ServerOut{
		Error:      name,
		Parameters: parameters,
	})
}
