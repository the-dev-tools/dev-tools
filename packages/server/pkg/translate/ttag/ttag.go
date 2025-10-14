package ttag

import (
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mtag"
	tagv1 "the-dev-tools/spec/dist/buf/go/tag/v1"
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
	switch color {
	case mtag.ColorSlate:
		return tagv1.TagColor_TAG_COLOR_SLATE, nil
	case mtag.ColorGreen:
		return tagv1.TagColor_TAG_COLOR_GREEN, nil
	case mtag.ColorAmber:
		return tagv1.TagColor_TAG_COLOR_AMBER, nil
	case mtag.ColorSky:
		return tagv1.TagColor_TAG_COLOR_SKY, nil
	case mtag.ColorPurple:
		return tagv1.TagColor_TAG_COLOR_PURPLE, nil
	case mtag.ColorRose:
		return tagv1.TagColor_TAG_COLOR_ROSE, nil
	case mtag.ColorBlue:
		return tagv1.TagColor_TAG_COLOR_BLUE, nil
	case mtag.ColorFuchsia:
		return tagv1.TagColor_TAG_COLOR_FUCHSIA, nil
	default:
		return tagv1.TagColor_TAG_COLOR_UNSPECIFIED, fmt.Errorf("unknown tag color %d", color)
	}
}

func protoTagColorToModel(color tagv1.TagColor) (mtag.Color, error) {
	switch color {
	case tagv1.TagColor_TAG_COLOR_SLATE:
		return mtag.ColorSlate, nil
	case tagv1.TagColor_TAG_COLOR_GREEN:
		return mtag.ColorGreen, nil
	case tagv1.TagColor_TAG_COLOR_AMBER:
		return mtag.ColorAmber, nil
	case tagv1.TagColor_TAG_COLOR_SKY:
		return mtag.ColorSky, nil
	case tagv1.TagColor_TAG_COLOR_PURPLE:
		return mtag.ColorPurple, nil
	case tagv1.TagColor_TAG_COLOR_ROSE:
		return mtag.ColorRose, nil
	case tagv1.TagColor_TAG_COLOR_BLUE:
		return mtag.ColorBlue, nil
	case tagv1.TagColor_TAG_COLOR_FUCHSIA:
		return mtag.ColorFuchsia, nil
	case tagv1.TagColor_TAG_COLOR_UNSPECIFIED:
		return 0, fmt.Errorf("tag color cannot be unspecified")
	default:
		return 0, fmt.Errorf("unknown tag color enum %v", color)
	}
}

func fallbackTagColor() tagv1.TagColor {
	return tagv1.TagColor_TAG_COLOR_UNSPECIFIED
}
