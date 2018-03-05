package varlink

import (
	"encoding/json"
)

// ClientIn represents the outgoing message sent by a Client to a Service.
type ClientOut struct {
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	Continues  bool             `json:"continues,omitempty"`
	Error      string           `json:"error,omitempty"`
}

// ClientIn represents the incoming message received by the Client from a Service.
type ClientIn struct {
	Method     string      `json:"method"`
	Parameters interface{} `json:"parameters,omitempty"`
	More       bool        `json:"more,omitempty"`
}
