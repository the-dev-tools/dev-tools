package titemnest_test

import (
	"dev-tools-backend/pkg/model/mcollection/mitemapi"
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	"dev-tools-backend/pkg/translate/titemnest"
	collectionv1 "devtools-services/gen/collection/v1"
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestTranslateItemFolderNested(t *testing.T) {
	rootFolderUlid := ulid.Make()

	folders := []mitemfolder.ItemFolder{
		{
			ID:           rootFolderUlid,
			Name:         "test root",
			ParentID:     nil,
			CollectionID: ulid.Make(),
		},
		{
			ID:           ulid.Make(),
			Name:         "test",
			ParentID:     &rootFolderUlid,
			CollectionID: ulid.Make(),
		},
		{
			ID:           ulid.Make(),
			Name:         "test",
			ParentID:     &rootFolderUlid,
			CollectionID: ulid.Make(),
		},
	}
	apis := []mitemapi.ItemApi{
		{
			ID:           ulid.Make(),
			Name:         "test",
			CollectionID: ulid.Make(),
			Url:          "http://localhost:8080",
			Method:       "GET",
			ParentID:     nil,
		},
		{
			ID:           ulid.Make(),
			Name:         "test",
			CollectionID: ulid.Make(),
			Url:          "http://localhost:8080",
			Method:       "GET",
			ParentID:     &rootFolderUlid,
		},
	}

	collectionPair := titemnest.TranslateItemFolderNested(folders, apis)
	items := collectionPair.GetItemFolders()
	if len(items) != 2 {
		for _, item := range items {
			fmt.Println("Item", item)
		}
		t.Errorf("expected 2 items, got %d", len(items))
	}

	if items[0].GetData().(*collectionv1.Item_Folder).Folder.Meta.Id != folders[0].ID.String() {
		t.Errorf("expected %s, got %s", folders[0].ID.String(), items[0].GetData().(*collectionv1.Item_Folder).Folder.Meta.Id)
	}

	newItems := items[0].GetData().(*collectionv1.Item_Folder).Folder.Items
	if len(newItems) != 3 {
		t.Errorf("expected 1 item, got %d", len(newItems))
	}
}
