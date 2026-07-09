package main

import (
	"strings"
	"testing"
)

func TestGenerateTemplateV2(t *testing.T) {
	description := `
interface org.example.test

type Payload (
  object: object,
  value: any
)

method Ping(input: Payload) -> (output: Payload)

# @mode stream
method Watch(input: Payload) -> (output: Payload)

# @mode oneway
method Notify(input: Payload) -> ()

# @mode upgrade
method Bridge(input: Payload) -> (output: Payload)

error Failed(reason: string)
`

	_, out, err := generateTemplateV2(description)
	if err != nil {
		t.Fatalf("generateTemplateV2() error = %v", err)
	}

	text := string(out)
	checks := []string{
		`"github.com/varlink/go/v2/varlink"`,
		`type Payload struct`,
		`Object json.RawMessage`,
		`Value`,
		"`json:\"value\"`",
		`type PingRequest struct`,
		`type PingReply struct`,
		`func (c *Client) Ping(ctx context.Context, in PingRequest) (PingReply, error)`,
		`func (c *Client) Watch(ctx context.Context, in WatchRequest) (*WatchReplyStream, error)`,
		`func (c *Client) Notify(ctx context.Context, in NotifyRequest) error`,
		`func (c *Client) Bridge(ctx context.Context, in BridgeRequest) (BridgeReply, varlink.UpgradedConn, error)`,
		`type ClientBatch struct`,
		`func (b *ClientBatch) Ping(in PingRequest, out *PingReply)`,
		`func (b *ClientBatch) Notify(in NotifyRequest)`,
		`type Handler interface`,
		`func Register(builder *varlink.ServerBuilder, impl Handler) error`,
		`func (c UnaryCall) ReplyFailed(ctx context.Context, out Failed) error`,
		`func (c StreamCall) SendWatch(ctx context.Context, out WatchReply) error`,
		`func (c UpgradeCall) AcceptBridge(ctx context.Context, out BridgeReply) (io.ReadWriteCloser, error)`,
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Fatalf("generated output missing %q\n%s", check, text)
		}
	}

	rejects := []string{
		`PingStream`,
		`PingUpgrade`,
		`NotifyStream`,
		`NotifyUpgrade`,
		`WatchOneway`,
	}
	for _, reject := range rejects {
		if strings.Contains(text, reject) {
			t.Fatalf("generated output unexpectedly contains %q\n%s", reject, text)
		}
	}
}

func TestParseMethodModeIgnoresModePrefixWords(t *testing.T) {
	mode, err := parseMethodMode("@models are documentation\n@mode stream")
	if err != nil {
		t.Fatalf("parseMethodMode() error = %v", err)
	}
	if mode != modeStream {
		t.Fatalf("parseMethodMode() = %q, want %q", mode, modeStream)
	}
}
