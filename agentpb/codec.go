// Package agentpb is the canonical control-plane<->agent wire contract. It
// defines the message types and gRPC service stubs for the bidirectional agent
// stream, and registers a JSON codec under the name "proto" so grpc-go uses
// encoding/json instead of the protobuf wire format — no protoc or generated
// code is involved (the wire is hand-written JSON). Both the agent
// (client) and the control plane (server) import this single package; keep the
// JSON tags on the message types stable and evolve them additively only.
package agentpb

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                               { return "proto" } // overrides default proto codec
