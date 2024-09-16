package mitemfolder

import "dev-tools-backend/pkg/idwrap"

type ItemFolder struct {
	ID           idwrap.IDWrap
	CollectionID idwrap.IDWrap
	Name         string
	ParentID     *idwrap.IDWrap
	Prev         *idwrap.IDWrap
	Next         *idwrap.IDWrap
}

type ItemFolderNested struct {
	ItemFolder
	Children []interface{} // can be  ItemFolderNest or ItemApi
}
