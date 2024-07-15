package apicondition

import (
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodemaster"
	"devtools-nodes/pkg/nodes/api"
	"devtools-nodes/pkg/nodes/condition"
	"errors"
	"fmt"
)

type ConditionDataRestStatus struct {
	ApiData       *api.RestApiData
	ConditionData *condition.ConditionDataRestStatus
}

var ErrInvalidDataType = errors.New("invalid data type")

func ApiConditionRestStatus(mn *mnodemaster.NodeMaster) error {
	data, ok := mn.CurrentNode.Data.(*ConditionDataRestStatus)
	if !ok {
		return ErrInvalidDataType
	}

	mn.CurrentNode.Data = data.ApiData

	fmt.Println("Sending Rest API Request")

	err := api.SendRestApiRequest(mn)
	if err != nil {
		return err
	}

	fmt.Println("Calling ConditionRestStatus")

	_, err = nodemaster.GetVar(mn, api.VarResponseKey)
	if err != nil {
		return err
	}

	mn.CurrentNode.Data = data.ConditionData

	condition.ConditionRestStatus(mn)

	return nil
}
