package varlink

import (
	"encoding/json"
)

type ServerIn struct {
	Method     string           `json:"method"`
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	More       bool             `json:"more,omitempty"`
}

type ServerOut struct {
	Parameters interface{} `json:"parameters,omitempty"`
	Continues  bool        `json:"continues,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type ClientIn struct {
	Method     string      `json:"method"`
	Parameters interface{} `json:"parameters,omitempty"`
	More       bool        `json:"more,omitempty"`
}

type ClientReply struct {
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	Continues  bool             `json:"continues,omitempty"`
	Error      string           `json:"error,omitempty"`
}
