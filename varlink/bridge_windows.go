package varlink

import (
	"bufio"
	"io"
	"log"
	"net"
	"os/exec"
)

type PipeCon struct {
	net.Conn
	reader *io.ReadCloser
	writer *io.WriteCloser
}

func (p PipeCon) Close() error {
	p.reader = nil
	p.writer = nil
	return nil
}

// NewConnection returns a new connection to the given address.
func NewBridge(bridge string) (*Connection, error) {
	//var err error

	c := Connection{}
	cmd := exec.Command("cmd", "/C", bridge)
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	c.conn = PipeCon{nil, &r, &w}
	c.address = ""
	c.reader = bufio.NewReader(r)
	c.writer = bufio.NewWriter(w)

	go func() {
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}()

	return &c, nil
}

