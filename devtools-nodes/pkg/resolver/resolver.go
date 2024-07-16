package resolver

import (
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/api"
	"devtools-nodes/pkg/nodes/communication"
	"devtools-nodes/pkg/nodes/condition"
	"devtools-nodes/pkg/nodes/loop"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	"errors"

	"google.golang.org/protobuf/proto"
)

const (
	// Api calls
	ApiCallRest = "apiCallRest"

	// Conditions
	IFStatusCode = "ifStatusCode"

	// Communication such as email sms etc
	CommunicationEmail = "communicationEmail"

	// Loops
	LoopFor = "loopFor"
)

var ErrInvalidDataType = errors.New("invalid data type")

func ResolveNodeFunc(nodeType string) (func(*mnodemaster.NodeMaster) error, error) {
	switch nodeType {
	case ApiCallRest:
		return api.SendRestApiRequest, nil
	case IFStatusCode:
		return condition.ConditionRestStatus, nil
	case CommunicationEmail:
		return communication.SendEmail, nil
	}
	return nil, nil
}

func ConvertProtoMsg(msg proto.Message) (interface{}, error) {
	switch msgType := msg.(type) {
	case *nodedatav1.NodeApiCallData:
		casted := msg.(*nodedatav1.NodeApiCallData)
		data := &api.RestApiData{
			Url:         casted.Url,
			QueryParams: casted.QueryParams,
			Method:      casted.Method,
			Headers:     casted.Headers,
			Body:        casted.Body,
		}
		return data, nil
	case *nodedatav1.NodeForRemote:
		casted := msg.(*nodedatav1.NodeForRemote)
		data := &loop.LoopRemoteData{
			Count:          casted.Count,
			LoopStartNode:  casted.LoopStartNode,
			MachinesAmount: casted.MachinesAmount,
		}
		return data, nil
	default:
		return msgType, ErrInvalidDataType
	}
}
