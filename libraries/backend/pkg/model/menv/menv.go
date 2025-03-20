package menv

import (
	"the-dev-tools/backend/pkg/idwrap"
	"time"
)

type EnvType int8

const (
	EnvUnkown EnvType = 0
	EnvGlobal EnvType = 1
	EnvNormal EnvType = 2
)

type Env struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Active      bool
	Type        EnvType
	Description string
	Name        string
	Updated     time.Time
}
