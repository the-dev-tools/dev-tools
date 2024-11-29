package mitemapiexample

import (
	"dev-tools-backend/pkg/idwrap"
	"time"
)

type BodyType int8

const (
	BodyTypeUndefined  BodyType = 0
	BodyTypeNone       BodyType = 1
	BodyTypeRaw        BodyType = 2
	BodyTypeForm       BodyType = 3
	BodyTypeUrlencoded BodyType = 4
)

type ItemApiExampleMeta struct {
	Name string
	ID   idwrap.IDWrap
}

type ItemApiExample struct {
	ParentExampleID *idwrap.IDWrap
	Prev            *idwrap.IDWrap
	Next            *idwrap.IDWrap
	Updated         time.Time
	Name            string
	IsDefault       bool
	BodyType        BodyType

	ID           idwrap.IDWrap
	ItemApiID    idwrap.IDWrap
	CollectionID idwrap.IDWrap
}

func NewItemApiExample(id, itemApiID, collectionID idwrap.IDWrap, parentExampleId *idwrap.IDWrap, isDefault bool, name string) *ItemApiExample {
	return &ItemApiExample{
		ID:              id,
		ItemApiID:       itemApiID,
		ParentExampleID: parentExampleId,
		CollectionID:    collectionID,
		IsDefault:       isDefault,
		Name:            name,
		Updated:         time.Now(),
	}
}

func (i ItemApiExample) GetCreatedTime() time.Time {
	return i.ID.Time()
}
