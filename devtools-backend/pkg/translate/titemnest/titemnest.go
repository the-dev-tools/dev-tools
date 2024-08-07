package titemnest

import (
	"devtools-backend/pkg/model/mcollection/mitemapi"
	"devtools-backend/pkg/model/mcollection/mitemfolder"
	collectionv1 "devtools-services/gen/collection/v1"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	"fmt"

	"github.com/oklog/ulid/v2"
)

type CollectionPair struct {
	itemFolders []mitemfolder.ItemFolderNested
	itemApis    []mitemapi.ItemApi
}

func (c CollectionPair) GetItemFolders() []*collectionv1.Item {
	var items []*collectionv1.Item

	for _, item := range c.itemFolders {
		folderItem := &collectionv1.Item{
			Data: &collectionv1.Item_Folder{
				Folder: &collectionv1.Folder{
					Meta: &collectionv1.FolderMeta{
						Id:   item.ID.String(),
						Name: item.Name,
					},
					Items: RecursiveTranslate(item),
				},
			},
		}
		items = append(items, folderItem)
	}

	for _, item := range c.itemApis {
		apiItem := &collectionv1.Item{
			Data: &collectionv1.Item_ApiCall{
				ApiCall: &collectionv1.ApiCall{
					Meta: &collectionv1.ApiCallMeta{
						Name: item.Name,
						Id:   item.ID.String(),
					},
					CollectionId: item.CollectionID.String(),
					Data: &nodedatav1.NodeApiCallData{
						Url:         item.Url,
						Method:      item.Method,
						QueryParams: item.QueryParams.QueryMap,
						Headers:     item.Headers.HeaderMap,
					},
				},
			},
		}
		items = append(items, apiItem)
	}

	return items
}

func RecursiveTranslate(item mitemfolder.ItemFolderNested) []*collectionv1.Item {
	var items []*collectionv1.Item
	for _, child := range item.Children {
		folder, ok := child.(mitemfolder.ItemFolderNested)
		if ok {
			folderCollection := &collectionv1.Item{
				Data: &collectionv1.Item_Folder{
					Folder: &collectionv1.Folder{
						Meta: &collectionv1.FolderMeta{
							Id:   folder.ID.String(),
							Name: folder.Name,
						},

						ParentId: folder.ParentID.String(),
						Items:    RecursiveTranslate(folder),
					},
				},
			}
			items = append(items, folderCollection)
		} else {
			api, ok := child.(mitemapi.ItemApi)
			if ok {
				item := &collectionv1.Item{
					Data: &collectionv1.Item_ApiCall{
						ApiCall: &collectionv1.ApiCall{
							Meta: &collectionv1.ApiCallMeta{
								Name: api.Name,
								Id:   api.ID.String(),
							},
							ParentId: api.ParentID.String(),
							Data: &nodedatav1.NodeApiCallData{
								Url:         api.Url,
								Method:      api.Method,
								QueryParams: api.QueryParams.QueryMap,
								Headers:     api.Headers.HeaderMap,
								Body:        api.Body,
							},
						},
					},
				}
				items = append(items, item)
			}
		}
	}

	return items
}

// sort and create root fodler and check sub folder recoversive
// also put api with parentID in the folder
func TranslateItemFolderNested(folders []mitemfolder.ItemFolder, apis []mitemapi.ItemApi) CollectionPair {
	var collection CollectionPair
	sortedFolders := SortFoldersByUlidTime(folders)

	for i, item := range apis {
		if item.ParentID == nil {
			collection.itemApis = append(collection.itemApis, item)
			apis = append(apis[:i], apis[i+1:]...)
		}
	}

	tempNestedArr := make([]mitemfolder.ItemFolderNested, len(folders))
	for i, item := range sortedFolders {
		tempNestedArr[i] = mitemfolder.ItemFolderNested{
			ItemFolder: item,
			Children:   []interface{}{},
		}
	}

	newFolders := PutFoldersToSubFolder(tempNestedArr, &apis)
	collection.itemFolders = newFolders

	return collection
}

func SortFoldersByUlidTime(folders []mitemfolder.ItemFolder) []mitemfolder.ItemFolder {
	sortedFolders := make([]mitemfolder.ItemFolder, len(folders))
	copy(sortedFolders, folders)

	// Sort Folders older to newer
	for i := 0; i < len(sortedFolders); i++ {
		for j := i + 1; j < len(sortedFolders); j++ {
			if sortedFolders[i].ID.Compare(sortedFolders[j].ID) == 1 {
				sortedFolders[i], sortedFolders[j] = sortedFolders[j], sortedFolders[i]
			}
		}
	}

	return sortedFolders
}

func PutFoldersToSubFolder(folders []mitemfolder.ItemFolderNested, apis *[]mitemapi.ItemApi) []mitemfolder.ItemFolderNested {
	rootFolders := []mitemfolder.ItemFolderNested{}
	// check folders and sub folder if find parentID match set put them inside

	for _, folder := range folders {
		if folder.ParentID == nil {

			folder.Children = searchAndPut(&folder.ID, folders, apis)
			rootFolders = append(rootFolders, folder)
		}
	}

	return rootFolders
}

func searchAndPut(parentID *ulid.ULID, folders []mitemfolder.ItemFolderNested, apis *[]mitemapi.ItemApi) []interface{} {
	var children []interface{}

	for i, item := range folders {
		if item.ParentID != nil && item.ParentID.Compare(*parentID) == 0 {
			if i < len(folders) {
				folders = append(folders[:i], folders[i+1:]...)
				fmt.Println("remove folder: ", item.Name)
			}
			item.Children = searchAndPut(&item.ID, folders, apis)
			children = append(children, item)
		}
	}

	for _, item := range *apis {
		if item.ParentID != nil && item.ParentID.Compare(*parentID) == 0 {
			children = append(children, item)
		}
	}

	return children
}
