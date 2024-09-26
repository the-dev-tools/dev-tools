package menv

import "dev-tools-backend/pkg/idwrap"

type EnvType int8

const (
	EnvUnkown    EnvType = 0
	EnvGlobal    EnvType = 1
	EnvWorkspace EnvType = 2
)

type Env struct {
	ID           idwrap.IDWrap
	Workspace_ID idwrap.IDWrap
	Type         EnvType
	Name         string
}
