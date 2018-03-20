package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// ResolverAddress is the well-known address of the varlink interface resolver,
// it translates varlink interface names to varlink service addresses.
const ResolverAddress = "unix:/run/org.varlink.resolver"

type clientCall struct {
	Method     string      `json:"method"`
	Parameters interface{} `json:"parameters,omitempty"`
	More       bool        `json:"more,omitempty"`
	OneShot    bool        `json:"oneshot,omitempty"`
}

type clientReply struct {
	Parameters *json.RawMessage `json:"parameters"`
	Continues  bool             `json:"continues"`
	Error      string           `json:"error"`
}

// Connection is a connection from a client to a service.
type Connection struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func (c *Connection) sendMessage(message *clientCall) error {
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

func (c *Connection) receiveMessage(message *clientReply) error {
	out, err := c.reader.ReadBytes('\x00')
	if err != nil {
		return err
	}

	return json.Unmarshal(out[:len(out)-1], message)
}

// Call sends a method call and returns the result of the call.
func (c *Connection) Call(method string, parameters interface{}, result interface{}) error {
	call := clientCall{
		Method:     method,
		Parameters: parameters,
	}

	err := c.sendMessage(&call)
	if err != nil {
		return err
	}

	var r clientReply
	err = c.receiveMessage(&r)
	if err != nil {
		return err
	}

	if r.Error != "" {
		return fmt.Errorf(r.Error)
	}

	if result != nil {
		return json.Unmarshal(*r.Parameters, result)
	}

	return nil
}

// Close terminates the connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// NewConnection returns a new connection to the given address.
func NewConnection(address string) (*Connection, error) {
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
		return nil, err
	}

	c.reader = bufio.NewReader(c.conn)
	c.writer = bufio.NewWriter(c.conn)

	return &c, nil
}
