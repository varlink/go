package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
)

type Call interface {
	WantMore() bool
	Parameters(in interface{}) error
	Reply(out *ServerOut) error
	ReplyError(name string, parameters interface{}) error
}

type context struct {
	Call
	writer *bufio.Writer
	in     *ServerIn
}

func (this *context) WantMore() bool {
	return this.in.More
}

func (this *context) Parameters(in interface{}) error {
	if this.in.Parameters == nil {
		return fmt.Errorf("Empty Parameters")
	}
	return json.Unmarshal(*this.in.Parameters, in)
}

func (this *context) Reply(out *ServerOut) error {
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

func (this *context) ReplyError(name string, parameters interface{}) error {
	return this.Reply(&ServerOut{
		Error:      name,
		Parameters: parameters,
	})
}
