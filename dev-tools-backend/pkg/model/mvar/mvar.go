package mvar

import "dev-tools-backend/pkg/idwrap"

type Var struct {
	ID     idwrap.IDWrap
	EnvID  idwrap.IDWrap
	VarKey string
	Value  string
}
