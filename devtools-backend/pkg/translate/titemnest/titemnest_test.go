package titemnest_test

import (
	"devtools-backend/pkg/model/mcollection/mitemapi"
	"devtools-backend/pkg/model/mcollection/mitemfolder"
	"devtools-backend/pkg/translate/titemnest"
	collectionv1 "devtools-services/gen/collection/v1"
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestTranslateItemFolderNested(t *testing.T) {
	folders := []mitemfolder.ItemFolder{
		{
			ID:           ulid.Make(),
			Name:         "test",
			ParentID:     nil,
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
		},
		{
			ID:           ulid.Make(),
			Name:         "test",
			CollectionID: ulid.Make(),
			Url:          "http://localhost:8080",
			Method:       "GET",
			ParentID:     &folders[0].ID,
		},
	}

	collectionPair := titemnest.TranslateItemFolderNested(folders, apis)
	items := collectionPair.GetItemFolders()
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	if items[0].GetData().(*collectionv1.Item_Folder).Folder.Meta.Id != folders[0].ID.String() {
		t.Errorf("expected %s, got %s", folders[0].ID.String(), items[0].GetData().(*collectionv1.Item_Folder).Folder.Meta.Id)
	}

	newItems := items[0].GetData().(*collectionv1.Item_Folder).Folder.Items
	fmt.Println("New Items", newItems)
	for _, item := range newItems {
		fmt.Println("Nested item", item)
	}
}
