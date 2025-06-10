package mitemapi

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
)

type ItemApi struct {
	FolderID        *idwrap.IDWrap
	Prev            *idwrap.IDWrap
	Next            *idwrap.IDWrap
	VersionParentID *idwrap.IDWrap
	DeltaParentID   *idwrap.IDWrap
	Name            string
	Url             string
	Method          string
	ID              idwrap.IDWrap
	CollectionID    idwrap.IDWrap
}

type ItemApiWithExamples struct {
	Examples []mitemapiexample.ItemApiExampleMeta
	ItemApi
	DefaultExample mitemapiexample.ItemApiExample
}
