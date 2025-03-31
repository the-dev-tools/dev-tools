package mcollection

import (
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

const (
	CollectionNodeTypeUnspecified int32 = 0
	CollectionNodeTypeRequest     int32 = 1
	CollectionNodeTypeFolder      int32 = 2
)

type Collection struct {
	Updated time.Time
	Name    string
	ID      idwrap.IDWrap
	OwnerID idwrap.IDWrap
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
