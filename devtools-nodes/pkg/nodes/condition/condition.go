package condition

import (
	"fmt"
	"net/http"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
)

type ConditionDataRestStatus struct {
	StatusCodeExits map[string]string
}

func ConditionRestStatus(mn *mnodemaster.NodeMaster) error {
	data := mn.CurrentNode.Data.(*ConditionDataRestStatus)

	rawResponse, ok := mn.Vars["response"]
	if !ok {
		return mnodemaster.ErrInvalidDataType
	}

	response, ok := rawResponse.(*http.Response)
	if !ok {
		return mnodemaster.ErrInvalidDataType
	}

	restStatus := fmt.Sprint(response.StatusCode)
	nodeID, ok := data.StatusCodeExits[restStatus]
	if !ok {
		return mnodemaster.ErrInvalidDataType
	}

	mn.NextNodeID = nodeID

	return nil
}
