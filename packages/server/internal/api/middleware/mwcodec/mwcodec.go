// Package mwcodec provides custom codecs for Connect RPC.
package mwcodec

import (
	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// protoJSONCodec is a custom JSON codec that emits unpopulated/zero-value fields.
// This is needed because proto3 omits zero values by default, which breaks
// delta/sync updates where we need to explicitly set fields to their zero value.
type protoJSONCodec struct {
	name string
}

var _ connect.Codec = (*protoJSONCodec)(nil)

var (
	marshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true, // Include zero-value fields in JSON output
	}
	unmarshalOptions = protojson.UnmarshalOptions{
		DiscardUnknown: true, // Tolerate unknown fields for forward compatibility
	}
)

// Name returns the codec name.
func (c *protoJSONCodec) Name() string {
	return c.name
}

// Marshal serializes a protobuf message to JSON with zero values included.
func (c *protoJSONCodec) Marshal(msg any) ([]byte, error) {
	protoMsg, ok := msg.(proto.Message)
	if !ok {
		return nil, errNotProto(msg)
	}
	return marshalOptions.Marshal(protoMsg)
}

// Unmarshal deserializes JSON to a protobuf message.
func (c *protoJSONCodec) Unmarshal(data []byte, msg any) error {
	protoMsg, ok := msg.(proto.Message)
	if !ok {
		return errNotProto(msg)
	}
	return unmarshalOptions.Unmarshal(data, protoMsg)
}

func errNotProto(msg any) error {
	return connect.NewError(
		connect.CodeInternal,
		&errNotProtoMessage{msg: msg},
	)
}

type errNotProtoMessage struct {
	msg any
}

func (e *errNotProtoMessage) Error() string {
	return "message is not a proto.Message"
}

// NewJSONCodec creates a new JSON codec that emits unpopulated fields.
// Use this with connect.WithCodec() to ensure zero values are included in JSON.
func NewJSONCodec() connect.Codec {
	return &protoJSONCodec{name: "json"}
}

// WithJSONCodec returns a connect.Option that uses the custom JSON codec.
func WithJSONCodec() connect.HandlerOption {
	return connect.WithCodec(NewJSONCodec())
}
