package rflow

import (
    "the-dev-tools/server/internal/api"
    nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
    flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
    "the-dev-tools/server/pkg/idwrap"
)

// sendNodeStatus sends a node state update over the stream.
func sendNodeStatus(stream api.ServerStreamAdHoc[flowv1.FlowRunResponse], nodeID idwrap.IDWrap, state nodev1.NodeState, info *string) error {
    nodeResp := &flowv1.FlowRunNodeResponse{NodeId: nodeID.Bytes(), State: state, Info: info}
    return stream.Send(&flowv1.FlowRunResponse{Node: nodeResp})
}

// sendExampleResponse sends example/response linkage over the stream.
func sendExampleResponse(stream api.ServerStreamAdHoc[flowv1.FlowRunResponse], exampleID, responseID idwrap.IDWrap) error {
    example := &flowv1.FlowRunExampleResponse{ExampleId: exampleID.Bytes(), ResponseId: responseID.Bytes()}
    return stream.Send(&flowv1.FlowRunResponse{Example: example})
}

