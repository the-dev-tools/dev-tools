package mtag

import "dev-tools-backend/pkg/idwrap"

type Tag struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Name        string
	Color       uint8
}
