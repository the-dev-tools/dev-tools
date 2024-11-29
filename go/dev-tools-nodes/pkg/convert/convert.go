package convert

/*

import (
	"dev-tools-nodes/pkg/model/medge"
	"dev-tools-nodes/pkg/model/mnode"
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mresolver"
	"dev-tools-nodes/pkg/model/mstatus"
	nodedatav1 "dev-tools-services/gen/nodedata/v1"
	nodemasterv1 "dev-tools-services/gen/nodemaster/v1"
	nodestatusv1 "dev-tools-services/gen/nodestatus/v1"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func ConvertStructToMsg(rawData interface{}) (*anypb.Any, error) {
	var anyData *anypb.Any
	var err error

	switch msgType := rawData.(type) {
	case *mnodedata.NodeApiRestData:
		data := rawData.(*mnodedata.NodeApiRestData)
		nodeData := &nodedatav1.NodeApiCallData{
			Url: data.Url,
			// TODO: change to QueryParams
			// QueryParams: data.Query,
			Method: data.Method,
			// TODO: change to Headers
			// Headers:     data.Headers,
			Body: data.Body,
		}
		anyData, err = anypb.New(nodeData)
	case *mnodedata.NodeLoopRemoteData:
		data := rawData.(*mnodedata.NodeLoopRemoteData)
		nodeData := &nodedatav1.NodeForRemote{
			Count:             data.Count,
			LoopStartNode:     data.LoopStartNode,
			MachineEmount:     data.MachinesAmount,
			SlaveHttpEndpoint: data.SlaveHttpEndpoint,
		}
		anyData, err = anypb.New(nodeData)
	default:
		return nil, fmt.Errorf("unsupported type %T", msgType)
	}
	if err != nil {
		return nil, err
	}
	return anyData, nil
}

func ConvertNodeStatusToMsg(status mstatus.NodeStatus) (*anypb.Any, error) {
	statusData := status.Data
	var anyStatus *anypb.Any
	var err error
	switch statusType := statusData.(type) {
	case mstatus.NodeStatusNextNode:
		data, ok := statusData.(mstatus.NodeStatusNextNode)
		if !ok {
			return nil, fmt.Errorf("failed to cast NodeStatusNextNode")
		}
		nodeStatus := &nodestatusv1.NodeStatusNextNode{
			NextNodeId: data.NodeID,
		}
		anyStatus, err = anypb.New(nodeStatus)
		return anyStatus, err
	case mstatus.NodeStatusSetVar:
		data, ok := statusData.(mstatus.NodeStatusSetVar)
		if !ok {
			return nil, fmt.Errorf("failed to cast NodeStatusSetVar")
		}
		anyVal, innerErr := ConvertPrimitiveInterfaceToWrapper(data.Val)
		if innerErr != nil {
			return nil, innerErr
		}
		nodeStatus := &nodestatusv1.NodeStatusSetVariable{
			VariableName:  data.Key,
			VariableValue: anyVal,
		}
		anyStatus, err = anypb.New(nodeStatus)
		return anyStatus, err
	default:
		return nil, fmt.Errorf("unsupported type %T", statusType)
	}
}

func ConvertMsgToNodeStatus(msg protoreflect.ProtoMessage) (interface{}, error) {
	var nodeStatusData interface{}
	switch msgType := msg.(type) {
	case *nodestatusv1.NodeStatusNextNode:
		casted := msg.(*nodestatusv1.NodeStatusNextNode)
		nodeStatusData = mstatus.NodeStatusNextNode{NodeID: casted.NextNodeId}
	case *nodestatusv1.NodeStatusSetVariable:
		casted := msg.(*nodestatusv1.NodeStatusSetVariable)
		nodeStatusData = mstatus.NodeStatusSetVar{Key: casted.VariableName, Val: casted.VariableValue}
	default:
		return nil, fmt.Errorf("unsupported type %T", msgType)
	}

	return nodeStatusData, nil
}

func ConvertPrimitiveInterfaceToWrapper(value interface{}) (*anypb.Any, error) {
	var wrappedMsg proto.Message
	switch valueType := value.(type) {
	case int32:
		wrappedMsg = wrapperspb.Int32(value.(int32))
	case int64:
		wrappedMsg = wrapperspb.Int64(value.(int64))
	case float32:
		wrappedMsg = wrapperspb.Float(value.(float32))
	case float64:
		wrappedMsg = wrapperspb.Double(value.(float64))
	case string:
		wrappedMsg = wrapperspb.String(value.(string))
	case bool:
		wrappedMsg = wrapperspb.Bool(value.(bool))
	default:
		return nil, fmt.Errorf("unsupported type %T", valueType)
	}
	return anypb.New(wrappedMsg)
}

func ConvertVarsToAny(vars map[string]interface{}) (map[string]*anypb.Any, error) {
	anyPbArray := make(map[string]*anypb.Any, len(vars))

	for key, v := range vars {
		msgMapElement, err := ConvertPrimitiveInterfaceToWrapper(v)
		if err != nil {
			// TODO: if cannot find type send as bytes of something
			continue
		}
		anyPbArray[key] = msgMapElement
	}
	return anyPbArray, nil
}

func ConvertMsgNodesToNodes(nodes map[string]*nodemasterv1.Node, resolverFunc mresolver.ResolverProto) (map[string]mnode.Node, error) {
	convertedNodes := make(map[string]mnode.Node, len(nodes))

	for key, node := range nodes {
		msg, err := anypb.UnmarshalNew(node.Data, proto.UnmarshalOptions{})
		if err != nil {
			return nil, err
		}

		castedData, err := resolverFunc(msg)
		if err != nil {
			return nil, err
		}

		tempNode := mnode.Node{ID: node.Id, Type: node.Type, Data: castedData, OwnerID: node.OwnerId, GroupID: node.GroupId, Edges: medge.Edges{OutNodes: node.Edges.OutNodes}}
		convertedNodes[key] = tempNode
	}
	return convertedNodes, nil
}

func ConvertMsgNodeToNode(node *nodemasterv1.Node, resolverFunc mresolver.ResolverProto) (*mnode.Node, error) {
	msg, err := anypb.UnmarshalNew(node.Data, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	castedData, err := resolverFunc(msg)
	if err != nil {
		return nil, err
	}
	tempNode := mnode.Node{ID: node.Id, Type: node.Type, Data: castedData, OwnerID: node.OwnerId, GroupID: node.GroupId, Edges: medge.Edges{OutNodes: node.Edges.OutNodes}}
	return &tempNode, nil
}
*/
