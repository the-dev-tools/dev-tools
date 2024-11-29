package mresolver

import "google.golang.org/protobuf/proto"

type ResolverProto func(proto.Message) (interface{}, error)
