package mvar

import "the-dev-tools/server/pkg/idwrap"

const (
	Prefix = "{{"
	Suffix = "}}"
)

const (
	PrefixSize = len(Prefix)
	SuffixSize = len(Suffix)
)

type Var struct {
	ID          idwrap.IDWrap
	EnvID       idwrap.IDWrap
	VarKey      string
	Value       string
	Enabled     bool
	Description string
}

// IsEnabled returns whether the variable is enabled
func (v Var) IsEnabled() bool {
	return v.Enabled
}
