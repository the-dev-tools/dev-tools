//nolint:revive // exported
package menv

import "the-dev-tools/server/pkg/idwrap"

const (
	Prefix = "{{"
	Suffix = "}}"
)

const (
	PrefixSize = len(Prefix)
	SuffixSize = len(Suffix)
)

type Variable struct {
	ID          idwrap.IDWrap
	EnvID       idwrap.IDWrap
	VarKey      string
	Value       string
	Enabled     bool
	Description string
	Order       float64
}

// IsEnabled returns whether the variable is enabled
func (v Variable) IsEnabled() bool {
	return v.Enabled
}
