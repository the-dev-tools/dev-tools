package ttag

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mtag"
	tagv1 "the-dev-tools/spec/dist/buf/go/tag/v1"
)

func SeralizeModelToRPCItem(e mtag.Tag) *tagv1.TagListItem {
	return &tagv1.TagListItem{
		TagId: e.ID.Bytes(),
		Name:  e.Name,
		Color: tagv1.TagColor(e.Color),
	}
}

func SeralizeModelToRPC(e mtag.Tag) *tagv1.Tag {
	return &tagv1.Tag{
		TagId: e.ID.Bytes(),
		Name:  e.Name,
		Color: tagv1.TagColor(e.Color),
	}
}

func SeralizeRpcToModel(e *tagv1.Tag, workspaceID idwrap.IDWrap) (*mtag.Tag, error) {
	flow := SeralizeRpcToModelWithoutID(e, workspaceID)
	id, err := idwrap.NewFromBytes(e.GetTagId())
	if err != nil {
		return nil, err
	}
	flow.ID = id
	return flow, nil
}

func SeralizeRpcToModelWithoutID(e *tagv1.Tag, workspaceID idwrap.IDWrap) *mtag.Tag {
	return &mtag.Tag{
		Name:        e.Name,
		Color:       uint8(e.Color),
		WorkspaceID: workspaceID,
	}
}
