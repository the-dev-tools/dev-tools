package snodejs

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
)

// INFO: for some reason sqlc generate `Js` as `J`, will check later why it is not working
func ConvertDBToModel(nf gen.FlowNodeJ) mnjs.MNJS {
	return mnjs.MNJS{
		FlowNodeID:       nf.FlowNodeID,
		Code:             nf.Code,
		CodeCompressType: nf.CodeCompressType,
	}
}

func ConvertModelToDB(mn mnjs.MNJS) gen.FlowNodeJ {
	return gen.FlowNodeJ{
		FlowNodeID:       mn.FlowNodeID,
		Code:             mn.Code,
		CodeCompressType: mn.CodeCompressType,
	}
}
