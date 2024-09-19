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
	ID   idwrap.IDWrap
	Name string
}

type ItemApiExample struct {
	ID              idwrap.IDWrap
	ItemApiID       idwrap.IDWrap
	ParentExampleID *idwrap.IDWrap
	CollectionID    idwrap.IDWrap
	IsDefault       bool
	BodyType        BodyType
	Name            string
	Updated         time.Time
	Prev            *idwrap.IDWrap
	Next            *idwrap.IDWrap
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
