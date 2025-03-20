package mitemfolder

import "the-dev-tools/backend/pkg/idwrap"

type ItemFolder struct {
	ParentID     *idwrap.IDWrap
	Prev         *idwrap.IDWrap
	Next         *idwrap.IDWrap
	Name         string
	ID           idwrap.IDWrap
	CollectionID idwrap.IDWrap
}

type ItemFolderNested struct {
	Children []interface{} // can be  ItemFolderNest or ItemApi
	ItemFolder
}
