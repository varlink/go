package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// ClientOut represents the outgoing message sent by a Client to a Service.
type ClientOut struct {
	Method     string      `json:"method"`
	Parameters interface{} `json:"parameters,omitempty"`
	More       bool        `json:"more,omitempty"`
}

// ClientIn represents the incoming message received by the Client from a Service.
type ClientIn struct {
	Parameters *json.RawMessage `json:"parameters"`
	Continues  bool             `json:"continues"`
	Error      string           `json:"error"`
}

// Connection is an active connection from a Client to a Service.
type Connection struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func (c *Connection) sendMessage(message *ClientOut) error {
	b, err := json.Marshal(message)
	if err != nil {
		return err
	}

	b = append(b, 0)
	_, err = c.writer.Write(b)
	if err != nil {
		return err
	}

	return c.writer.Flush()
}

func (c *Connection) receiveMessage(message *ClientIn) error {
	out, err := c.reader.ReadBytes('\x00')
	if err != nil {
		return err
	}

	return json.Unmarshal(out[:len(out)-1], message)
}

// Call sends a method call and returns the result of the call.
func (c *Connection) Call(method string, out, in interface{}) error {
	call := ClientOut{
		Method:     method,
		Parameters: out,
	}

	err := c.sendMessage(&call)
	if err != nil {
		return err
	}

	var r ClientIn
	err = c.receiveMessage(&r)
	if err != nil {
		return err
	}

	if r.Error != "" {
		return fmt.Errorf("%s", r.Error)
	}

	err = json.Unmarshal(*r.Parameters, in)
	if err != nil {
		return err
	}

	return nil
}

// Close terminates the connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// Returns a new connection to the given address.
func NewConnection(address string) (Connection, error) {
	var err error

	words := strings.SplitN(address, ":", 2)
	protocol := words[0]
	addr := words[1]

	// Ignore parameters after ';'
	words = strings.SplitN(addr, ";", 2)
	if words != nil {
		addr = words[0]
	}

	switch protocol {
	case "unix":
		break

	case "tcp":
		break
	}

	c := Connection{}
	c.conn, err = net.Dial(protocol, addr)
	if err != nil {
		return c, err
	}

	c.reader = bufio.NewReader(c.conn)
	c.writer = bufio.NewWriter(c.conn)

	return c, nil
}
