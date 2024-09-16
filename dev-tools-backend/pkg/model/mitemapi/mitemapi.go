package mitemapi

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapiexample"
)

type ItemApi struct {
	ID           idwrap.IDWrap
	CollectionID idwrap.IDWrap
	ParentID     *idwrap.IDWrap
	Name         string
	Url          string
	Method       string
	Prev         *idwrap.IDWrap
	Next         *idwrap.IDWrap
}

type ItemApiWithExamples struct {
	ItemApi

	DefaultExample mitemapiexample.ItemApiExample
	Examples       []mitemapiexample.ItemApiExampleMeta
}
