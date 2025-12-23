package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

// INFO: for some reason sqlc generate `Js` as `J`, will check later why it is not working
func ConvertDBToNodeJs(nf gen.FlowNodeJ) *mflow.NodeJS {
	return &mflow.NodeJS{
		FlowNodeID:       nf.FlowNodeID,
		Code:             nf.Code,
		CodeCompressType: nf.CodeCompressType,
	}
}

func ConvertNodeJsToDB(mn mflow.NodeJS) gen.FlowNodeJ {
	return gen.FlowNodeJ{
		FlowNodeID:       mn.FlowNodeID,
		Code:             mn.Code,
		CodeCompressType: mn.CodeCompressType,
	}
}
