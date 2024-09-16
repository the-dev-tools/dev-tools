package mitemapiexample

import (
	"dev-tools-backend/pkg/idwrap"
	"time"
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
	Name            string
	Compressed      bool
	Body            []byte
	Updated         time.Time
	Prev            *idwrap.IDWrap
	Next            *idwrap.IDWrap
}

func NewItemApiExample(id idwrap.IDWrap, itemApiID, collectionID idwrap.IDWrap, parentExampleId *idwrap.IDWrap, isDefault bool, name string, compressed bool, body []byte) *ItemApiExample {
	return &ItemApiExample{
		ID:              id,
		ItemApiID:       itemApiID,
		ParentExampleID: parentExampleId,
		CollectionID:    collectionID,
		IsDefault:       isDefault,
		Name:            name,
		Compressed:      compressed,
		Body:            body,
		Updated:         time.Now(),
	}
}

func (i ItemApiExample) GetCreatedTime() time.Time {
	return i.ID.Time()
}
