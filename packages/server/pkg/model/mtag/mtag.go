package mtag

import "the-dev-tools/server/pkg/idwrap"

type Tag struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Name        string
	Color       uint8
}
