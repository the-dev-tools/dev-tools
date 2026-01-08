//nolint:revive // exported
package mflow

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// FlowVariable represents a variable associated with a flow
type FlowVariable struct {
	ID          idwrap.IDWrap `json:"id"`
	FlowID      idwrap.IDWrap `json:"flow_id"`
	Name        string        `json:"key"`
	Value       string        `json:"value"`
	Enabled     bool          `json:"enabled"`
	Description string        `json:"description"`
	Order       float64       `json:"order"`
}

type FlowVariableUpdate struct {
	ID          idwrap.IDWrap `json:"id"`
	Name        *string       `json:"key"`
	Value       *string       `json:"value"`
	Enabled     *bool         `json:"enabled"`
	Description *string       `json:"description"`
}

func (fv FlowVariable) IsEnabled() bool {
	return fv.Enabled
}
