package apicondition

import (
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodemaster"
	api "dev-tools-nodes/pkg/nodes/nodeapi"
	"dev-tools-nodes/pkg/nodes/nodecondition"
	"errors"
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

	err := api.SendRestApiRequest(mn)
	if err != nil {
		return err
	}

	_, err = nodemaster.GetVar(mn, api.VarResponseKey)
	if err != nil {
		return err
	}

	mn.CurrentNode.Data = data.ConditionData

	err = nodecondition.ConditionRestStatus(mn)
	return err
}
