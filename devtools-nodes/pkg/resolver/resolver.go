package resolver

import (
	"errors"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/api"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/communication"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/condition"
)

const (
	ApiCallRest        = "apiCallRest"
	IFStatusCode       = "ifStatusCode"
	CommunicationEmail = "communicationEmail"
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
