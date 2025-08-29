package mcollection

import (
	"fmt"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
	"time"
)

const (
	CollectionNodeTypeUnspecified int32 = 0
	CollectionNodeTypeRequest     int32 = 1
	CollectionNodeTypeFolder      int32 = 2
)

type Collection struct {
	Updated     time.Time
	Name        string
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
}

func (c Collection) GetCreatedTime() time.Time {
	return c.ID.Time()
}

func (c Collection) GetCreatedTimeUnix() int64 {
	return idwrap.GetUnixMilliFromULID(c.ID)
}

type MetaCollection struct {
	Name string
	ID   idwrap.IDWrap
}

func (mc MetaCollection) GetCreatedTime() time.Time {
	return mc.ID.Time()
}

// Implement movable.Movable interface for Collection

// GetID returns the unique identifier of the collection
func (c Collection) GetID() idwrap.IDWrap {
	return c.ID
}

// GetListTypes returns all list types this collection can participate in
// Collections only participate in the collections list type within workspaces
func (c Collection) GetListTypes() []movable.ListType {
	return []movable.ListType{movable.CollectionListTypeCollections}
}

// GetPosition returns the current position of the collection in the specified list type
// For collections, this requires database access, so we'll implement this in the service layer
func (c Collection) GetPosition(listType movable.ListType) (int, error) {
	// This method requires database access, so it should be implemented via the repository
	// We'll return an error here to indicate this needs to be handled at the service level
	return 0, fmt.Errorf("GetPosition must be called through the repository layer")
}

// SetPosition updates the position of the collection in the specified list type
// For collections, this requires database access, so we'll implement this in the service layer
func (c Collection) SetPosition(listType movable.ListType, position int) error {
	// This method requires database access, so it should be implemented via the repository
	// We'll return an error here to indicate this needs to be handled at the service level
	return fmt.Errorf("SetPosition must be called through the repository layer")
}

// GetParentID returns the parent container ID for the specified list type
// For collections, the parent is always the workspace
func (c Collection) GetParentID(listType movable.ListType) (*idwrap.IDWrap, error) {
	switch listType {
	case movable.CollectionListTypeCollections:
		return &c.WorkspaceID, nil
	default:
		return nil, fmt.Errorf("collection does not support list type: %s", listType.String())
	}
}
