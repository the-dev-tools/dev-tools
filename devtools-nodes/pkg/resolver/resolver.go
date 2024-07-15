package resolver

import (
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/api"
	"devtools-nodes/pkg/nodes/communication"
	"devtools-nodes/pkg/nodes/condition"
	"errors"
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
