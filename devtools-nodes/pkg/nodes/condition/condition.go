package condition

import (
	"fmt"
	"net/http"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/medge"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/api"
	"github.com/DevToolsGit/devtools-nodes/pkg/parser"
	"github.com/PaesslerAG/gval"
)

type ConditionDataRestStatus struct {
	StatusCodeExits map[string]string
}

func ConditionRestStatus(mn *mnodemaster.NodeMaster) error {
	data := mn.CurrentNode.Data.(*ConditionDataRestStatus)
	if data == nil {
		return fmt.Errorf("no data provided for condition")
	}

	rawResponse, ok := mn.Vars[api.VarResponseKey]
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

type ConditionDataJsonMatch struct {
	Data       []byte
	Path       string
	MatchExits map[string]string
}

func ConditionJsonMatch(mn *mnodemaster.NodeMaster) error {
	data, ok := mn.CurrentNode.Data.(*ConditionDataJsonMatch)

	if !ok {
		return fmt.Errorf("no data provided for condition")
	}

	res, err := parser.ParseBytes(data.Data, data.Path)
	if err != nil {
		return fmt.Errorf("error parsing nested value: %s", err)
	}

	if !res.Exists() {
		return fmt.Errorf("result does not exist")
	}

	valStr := res.String()

	found := false
	for key, edge := range data.MatchExits {
		if key == valStr {
			mn.NextNodeID = edge
			found = true
			return nil
		}
	}

	if !found {
		return fmt.Errorf("no node found for value %s", valStr)
	}

	return nil
}

type ConditionDataExpression struct {
	Expression string
	MatchExits map[string]string
}

func ConditionExpression(mn *mnodemaster.NodeMaster) error {
	data, ok := mn.CurrentNode.Data.(*ConditionDataExpression)
	if !ok {
		return fmt.Errorf("no data provided for condition")
	}

	value, err := gval.Evaluate(data.Expression, mn.Vars)
	if err != nil {
		return fmt.Errorf("error evaluating expression: %s", err)
	}

	boolVal, ok := value.(bool)
	if ok {
		if boolVal {
			mn.NextNodeID = data.MatchExits[medge.DefaultSuccessEdge]
		} else {
			mn.NextNodeID = data.MatchExits[medge.DefaultFailureEdge]
		}
		return nil
	}

	strVal := fmt.Sprint(value)

	for key, edge := range data.MatchExits {
		if key == strVal {
			mn.NextNodeID = edge
			return nil
		}
	}

	return fmt.Errorf("no node found for value %s", strVal)
}
