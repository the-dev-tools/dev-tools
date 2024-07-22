package convert

import (
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/model/mresolver"
	"devtools-nodes/pkg/nodes/api"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func ConvertStructToMsg(rawData interface{}) (*anypb.Any, error) {
	anyData := new(anypb.Any)
	var err error

	switch msgType := rawData.(type) {
	case *api.RestApiData:
		data := rawData.(*api.RestApiData)
		nodeData := &nodedatav1.NodeApiCallData{
			Url:         data.Url,
			QueryParams: data.QueryParams,
			Method:      data.Method,
			Headers:     data.Headers,
			Body:        data.Body,
		}
		anyData, err = anypb.New(nodeData)
	case *mnodedata.LoopRemoteData:
		data := rawData.(*mnodedata.LoopRemoteData)
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
