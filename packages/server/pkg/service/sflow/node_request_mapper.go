package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertToDBNodeHTTP(nr mflow.NodeRequest) (gen.FlowNodeHttp, bool) {
	if nr.HttpID == nil || isZeroID(*nr.HttpID) {
		return gen.FlowNodeHttp{}, false
	}

	var deltaID []byte
	if nr.DeltaHttpID != nil && !isZeroID(*nr.DeltaHttpID) {
		deltaID = nr.DeltaHttpID.Bytes()
	}

	return gen.FlowNodeHttp{
		FlowNodeID:  nr.FlowNodeID,
		HttpID:      *nr.HttpID,
		DeltaHttpID: deltaID,
	}, true
}

func ConvertToModelNodeHTTP(nr gen.FlowNodeHttp) *mflow.NodeRequest {
	var deltaID *idwrap.IDWrap
	if len(nr.DeltaHttpID) > 0 {
		id, err := idwrap.NewFromBytes(nr.DeltaHttpID)
		if err == nil {
			deltaID = &id
		}
	}
	httpID := nr.HttpID

	result := &mflow.NodeRequest{
		FlowNodeID:  nr.FlowNodeID,
		HttpID:      &httpID,
		DeltaHttpID: deltaID,
	}
	result.HasRequestConfig = !isZeroID(nr.HttpID)
	return result
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == idwrap.IDWrap{}
}
