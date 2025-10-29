package menv

import (
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

type EnvType int8

const (
	EnvUnkown EnvType = 0
	EnvGlobal EnvType = 1
	EnvNormal EnvType = 2
)

const EnvVariablePrefix = "env."

type Env struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Type        EnvType
	Description string
	Name        string
	Updated     time.Time
	Order       float64
}
