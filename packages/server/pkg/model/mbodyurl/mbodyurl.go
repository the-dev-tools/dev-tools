package mbodyurl

import "the-dev-tools/server/pkg/idwrap"

type BodyURLEncoded struct {
	BodyKey       string         `json:"body_key"`
	Description   string         `json:"description"`
	Value         string         `json:"value"`
	Enable        bool           `json:"enable"`
	DeltaParentID *idwrap.IDWrap `json:"delta_parent_id"`
	ID            idwrap.IDWrap  `json:"id"`
	ExampleID     idwrap.IDWrap  `json:"example_id"`
}

func (bue BodyURLEncoded) IsEnabled() bool {
	return bue.Enable
}
