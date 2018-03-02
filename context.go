package varlink

import (
	"bufio"
	"encoding/json"
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
