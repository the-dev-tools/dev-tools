package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertDBToFlowTag(item gen.FlowTag) mflow.FlowTag {
	return mflow.FlowTag{
		ID:     item.ID,
		FlowID: item.FlowID,
		TagID:  item.TagID,
	}
}

func ConvertFlowTagToDB(item mflow.FlowTag) gen.FlowTag {
	return gen.FlowTag{
		ID:     item.ID,
		FlowID: item.FlowID,
		TagID:  item.TagID,
	}
}
