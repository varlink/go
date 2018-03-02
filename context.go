package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
)

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
		return fmt.Errorf("Empty Parameters")
	}
	return json.Unmarshal(*this.call.Parameters, in)
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
