package mitemapi

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapiexample"
)

type ItemApi struct {
	ParentID     *idwrap.IDWrap
	Prev         *idwrap.IDWrap
	Next         *idwrap.IDWrap
	Name         string
	Url          string
	Method       string
	ID           idwrap.IDWrap
	CollectionID idwrap.IDWrap
}

type ItemApiWithExamples struct {
	Examples []mitemapiexample.ItemApiExampleMeta
	ItemApi
	DefaultExample mitemapiexample.ItemApiExample
}
