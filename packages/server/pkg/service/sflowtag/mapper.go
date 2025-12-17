package sflowtag

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflowtag"
)

func ConvertDBToModel(item gen.FlowTag) mflowtag.FlowTag {
	return mflowtag.FlowTag{
		ID:     item.ID,
		FlowID: item.FlowID,
		TagID:  item.TagID,
	}
}

func ConvertModelToDB(item mflowtag.FlowTag) gen.FlowTag {
	return gen.FlowTag{
		ID:     item.ID,
		FlowID: item.FlowID,
		TagID:  item.TagID,
	}
}
