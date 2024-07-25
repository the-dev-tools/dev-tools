package nodecondition

import (
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/nodeapi"
	"devtools-nodes/pkg/parser"
	"fmt"

	"github.com/PaesslerAG/gval"
)

func ConditionRestStatus(mn *mnodemaster.NodeMaster) error {
	data := mn.CurrentNode.Data.(*mnodedata.NodeConditionRestStatusData)
	if data == nil {
		return fmt.Errorf("no data provided for condition")
	}

	response, err := nodeapi.GetHttpVarResponse(mn)
	if err != nil {
		return err
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

func ConditionJsonMatch(mn *mnodemaster.NodeMaster) error {
	data, ok := mn.CurrentNode.Data.(*mnodedata.NodeConditionJsonMatchData)

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

func ConditionExpression(mn *mnodemaster.NodeMaster) error {
	data, ok := mn.CurrentNode.Data.(*mnodedata.NodeConditionExpressionData)
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
