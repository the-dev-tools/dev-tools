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
		responseStatusCodeStr := fmt.Sprint(response.StatusCode)

		found := false
		// TODO: need to find a better way to match status codes
		for statusCodeKey, valNodeID := range data.StatusCodeExits {
			if statusCodeKey[0] == responseStatusCodeStr[0] || statusCodeKey[0] == '*' {
				if statusCodeKey[1] == responseStatusCodeStr[1] || statusCodeKey[1] == '*' {
					if statusCodeKey[2] == responseStatusCodeStr[2] || statusCodeKey[2] == '*' {
						nodeID = valNodeID
						found = true
					}
				}
			}
		}

		if !found {
			return fmt.Errorf("no node found for status code %s", restStatus)
		}
	}

	mn.NextNodeID = nodeID

	return nil
}
