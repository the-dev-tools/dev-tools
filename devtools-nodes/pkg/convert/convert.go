package convert

import (
	"devtools-nodes/pkg/nodes/api"
	nodedatav1 "devtools-services/gen/nodedata/v1"
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
		return nil, fmt.Errorf("unsupported type: %T", valueType)
	}
	return anypb.New(wrappedMsg)
}
