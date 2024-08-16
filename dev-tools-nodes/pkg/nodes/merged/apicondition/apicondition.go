package apicondition

import (
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodemaster"
	api "dev-tools-nodes/pkg/nodes/nodeapi"
	"dev-tools-nodes/pkg/nodes/nodecondition"
	"errors"
	"fmt"
)

type ConditionDataRestStatus struct {
	ApiData       *mnodedata.NodeApiRestData
	ConditionData *mnodedata.NodeConditionRestStatusData
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

	err = nodecondition.ConditionRestStatus(mn)
	return err
}
