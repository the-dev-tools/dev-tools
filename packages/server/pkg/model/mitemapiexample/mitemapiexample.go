package mitemapiexample

import (
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

type BodyType int8

const (
	BodyTypeNone BodyType = iota
	BodyTypeForm
	BodyTypeUrlencoded
	BodyTypeRaw
)

type ItemApiExampleMeta struct {
	Name string
	ID   idwrap.IDWrap
}

type ItemApiExample struct {
	Prev      *idwrap.IDWrap
	Next      *idwrap.IDWrap
	Updated   time.Time
	Name      string
	IsDefault bool
	BodyType  BodyType

	VersionParentID *idwrap.IDWrap

	ID           idwrap.IDWrap
	ItemApiID    idwrap.IDWrap
	CollectionID idwrap.IDWrap
}

func (i ItemApiExample) GetCreatedTime() time.Time {
	return i.ID.Time()
}
