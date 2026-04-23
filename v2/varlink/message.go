package varlink

import "github.com/varlink/go/v2/internal/codec"

// Message is the JSON wire representation used by the v2 codec/runtime stack.
type Message = codec.Message

// MaxFrameSizeDefault is the default maximum message size accepted by the runtime.
const MaxFrameSizeDefault = codec.DefaultMaxFrameSize
