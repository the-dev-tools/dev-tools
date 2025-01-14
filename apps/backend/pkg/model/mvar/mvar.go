package mvar

import "the-dev-tools/backend/pkg/idwrap"

const (
	Prefix = "{{env."
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
