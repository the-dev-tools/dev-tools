package mitemfolder

import "github.com/oklog/ulid/v2"

type ItemFolder struct {
	ID           ulid.ULID
	CollectionID ulid.ULID
	Name         string
	ParentID     *ulid.ULID
}

type ItemFolderNested struct {
	ItemFolder
	Children []interface{} // can be  ItemFolderNest or ItemApi
}
