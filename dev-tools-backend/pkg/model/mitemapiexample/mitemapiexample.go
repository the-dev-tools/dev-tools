package mitemapiexample

import (
	"time"

	"github.com/oklog/ulid/v2"
)

type ItemApiExampleMeta struct {
	ID   ulid.ULID
	Name string
}

type ItemApiExample struct {
	ID              ulid.ULID
	ItemApiID       ulid.ULID
	ParentExampleID *ulid.ULID
	CollectionID    ulid.ULID
	IsDefault       bool
	Name            string
	Compressed      bool
	Body            []byte
	Updated         time.Time
	Prev            *ulid.ULID
	Next            *ulid.ULID
}

func NewItemApiExample(id ulid.ULID, itemApiID, collectionID ulid.ULID, parentExampleId *ulid.ULID, isDefault bool, name string, compressed bool, body []byte) *ItemApiExample {
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
	return time.UnixMilli(int64(i.ID.Time()))
}
