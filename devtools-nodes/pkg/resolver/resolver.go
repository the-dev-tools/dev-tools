package resolver

import (
	"errors"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/api"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/communication"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/condition"
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

type Resolver func(nodeType string) (func(*mnodemaster.NodeMaster) error, error)

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
