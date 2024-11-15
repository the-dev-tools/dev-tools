package mftag

import "dev-tools-backend/pkg/idwrap"

type FlowTag struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Name        string
}
