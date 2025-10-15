package ttag

import (
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mtag"
	tagv1 "the-dev-tools/spec/dist/buf/go/tag/v1"
)

var (
	modelToProtoColor = map[mtag.Color]tagv1.TagColor{
		mtag.ColorSlate:   tagv1.TagColor_TAG_COLOR_SLATE,
		mtag.ColorGreen:   tagv1.TagColor_TAG_COLOR_GREEN,
		mtag.ColorAmber:   tagv1.TagColor_TAG_COLOR_AMBER,
		mtag.ColorSky:     tagv1.TagColor_TAG_COLOR_SKY,
		mtag.ColorPurple:  tagv1.TagColor_TAG_COLOR_PURPLE,
		mtag.ColorRose:    tagv1.TagColor_TAG_COLOR_ROSE,
		mtag.ColorBlue:    tagv1.TagColor_TAG_COLOR_BLUE,
		mtag.ColorFuchsia: tagv1.TagColor_TAG_COLOR_FUCHSIA,
	}

	protoToModelColor = func() map[tagv1.TagColor]mtag.Color {
		inverse := make(map[tagv1.TagColor]mtag.Color, len(modelToProtoColor))
		for model, proto := range modelToProtoColor {
			if proto == fallbackTagColor() {
				continue
			}
			inverse[proto] = model
		}
		return inverse
	}()
)

func SeralizeModelToRPCItem(e mtag.Tag) *tagv1.TagListItem {
	color := fallbackTagColor()
	if converted, err := modelTagColorToProto(e.Color); err == nil {
		color = converted
	}

	return &tagv1.TagListItem{
		TagId: e.ID.Bytes(),
		Name:  e.Name,
		Color: color,
	}
}

func SeralizeModelToRPC(e mtag.Tag) *tagv1.Tag {
	color := fallbackTagColor()
	if converted, err := modelTagColorToProto(e.Color); err == nil {
		color = converted
	}

	return &tagv1.Tag{
		TagId: e.ID.Bytes(),
		Name:  e.Name,
		Color: color,
	}
}

func SeralizeRpcToModel(e *tagv1.Tag, workspaceID idwrap.IDWrap) (*mtag.Tag, error) {
	flow, err := SeralizeRpcToModelWithoutID(e, workspaceID)
	if err != nil {
		return nil, err
	}

	id, err := idwrap.NewFromBytes(e.GetTagId())
	if err != nil {
		return nil, err
	}
	flow.ID = id
	return flow, nil
}

func SeralizeRpcToModelWithoutID(e *tagv1.Tag, workspaceID idwrap.IDWrap) (*mtag.Tag, error) {
	color, err := protoTagColorToModel(e.Color)
	if err != nil {
		return nil, err
	}

	return &mtag.Tag{
		Name:        e.Name,
		Color:       color,
		WorkspaceID: workspaceID,
	}, nil
}

func modelTagColorToProto(color mtag.Color) (tagv1.TagColor, error) {
	if proto, ok := modelToProtoColor[color]; ok {
		return proto, nil
	}

	return fallbackTagColor(), fmt.Errorf("unknown tag color %d", color)
}

func protoTagColorToModel(color tagv1.TagColor) (mtag.Color, error) {
	if color == fallbackTagColor() {
		return 0, fmt.Errorf("tag color cannot be unspecified")
	}

	if model, ok := protoToModelColor[color]; ok {
		return model, nil
	}

	return 0, fmt.Errorf("unknown tag color enum %v", color)
}

func fallbackTagColor() tagv1.TagColor {
	return tagv1.TagColor_TAG_COLOR_UNSPECIFIED
}
