package mvar

import "dev-tools-backend/pkg/idwrap"

const (
	Prefix = '$'
	Suffix = '$'
)

type Var struct {
	ID     idwrap.IDWrap
	EnvID  idwrap.IDWrap
	VarKey string
	Value  string
}
