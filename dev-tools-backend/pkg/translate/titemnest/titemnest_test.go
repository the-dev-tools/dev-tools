package titemnest_test

import (
	"dev-tools-backend/pkg/model/mcollection/mitemapi"
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	"dev-tools-backend/pkg/translate/titemnest"
	collectionv1 "dev-tools-services/gen/collection/v1"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func TestTranslateItemFolderNested(t *testing.T) {
	timeNow := time.Now()
	rootFolderUlid := ulid.MustNew(ulid.Timestamp(timeNow), ulid.DefaultEntropy())
	collectionUlid := ulid.Make()

	folders := []mitemfolder.ItemFolder{
		{
			ID:           rootFolderUlid,
			Name:         "test folder root",
			ParentID:     nil,
			CollectionID: collectionUlid,
		},
		{
			ID:           ulid.MustNew(ulid.Timestamp(timeNow.Add(time.Second)), ulid.DefaultEntropy()),
			Name:         "test folder #1",
			ParentID:     &rootFolderUlid,
			CollectionID: collectionUlid,
		},
		{
			ID:           ulid.MustNew(ulid.Timestamp(timeNow.Add(time.Second*2)), ulid.DefaultEntropy()),
			Name:         "test folder #2",
			ParentID:     &rootFolderUlid,
			CollectionID: collectionUlid,
		},
	}
	apis := []mitemapi.ItemApi{
		{
			ID:           ulid.MustNew(ulid.Timestamp(timeNow.Add(time.Millisecond*0)), ulid.DefaultEntropy()),
			Name:         "test api #1",
			CollectionID: collectionUlid,
			Url:          "http://localhost:8080",
			Method:       "GET",
			ParentID:     nil,
		},
		{
			ID:           ulid.MustNew(ulid.Timestamp(timeNow.Add(time.Second*3)), ulid.DefaultEntropy()),
			Name:         "test api #2",
			CollectionID: collectionUlid,
			Url:          "http://localhost:8080",
			Method:       "GET",
			ParentID:     &rootFolderUlid,
		},
	}

	// Root/
	// - test (api)
	// - test/
	// - test/
	// test (api)
	//
	//

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
		t.Errorf("expected 3 sub item, got %d", len(newItems))
		for _, item := range newItems {
			fmt.Println("Item", item)
		}

	}
}
