package resolver

import (
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodes/nodeapi"
	"dev-tools-nodes/pkg/nodes/nodecom"
	"dev-tools-nodes/pkg/nodes/nodecondition"
	"dev-tools-nodes/pkg/nodes/nodeloop"
	nodedatav1 "dev-tools-services/gen/nodedata/v1"
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
	LoopFor            = "loopFor"
	NodeTypeLoopRemote = "loopRemote"
)

var ErrInvalidDataType = errors.New("invalid data type")

func ResolveNodeFunc(nodeType string) (func(*mnodemaster.NodeMaster) error, error) {
	switch nodeType {
	case ApiCallRest:
		return nodeapi.SendRestApiRequest, nil
	case IFStatusCode:
		return nodecondition.ConditionRestStatus, nil
	case CommunicationEmail:
		return nodecom.SendEmail, nil
	case NodeTypeLoopRemote:
		return nodeloop.ForRemoteLoop, nil
	}
	return nil, nil
}

func ConvertProtoMsg(msg proto.Message) (interface{}, error) {
	switch msgType := msg.(type) {
	case *nodedatav1.NodeApiCallData:
		casted := msg.(*nodedatav1.NodeApiCallData)
		data := &mnodedata.NodeApiRestData{
			Url:     casted.Url,
			Query:   casted.QueryParams,
			Method:  casted.Method,
			Headers: casted.Headers,
			Body:    casted.Body,
		}
		return data, nil
	case *nodedatav1.NodeForRemote:
		casted := msg.(*nodedatav1.NodeForRemote)
		data := &mnodedata.NodeLoopRemoteData{
			Count:             casted.Count,
			LoopStartNode:     casted.LoopStartNode,
			MachinesAmount:    casted.MachineEmount,
			SlaveHttpEndpoint: casted.SlaveHttpEndpoint,
		}
		return data, nil
	default:
		return msgType, ErrInvalidDataType
	}
}
