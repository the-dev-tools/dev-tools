package titemnest

import (
	"dev-tools-backend/pkg/model/mcollection/mitemapi"
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	collectionv1 "dev-tools-services/gen/collection/v1"
	nodedatav1 "dev-tools-services/gen/nodedata/v1"
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
						QueryParams: item.Query.QueryMap,
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
								QueryParams: api.Query.QueryMap,
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
func TranslateItemFolderNested(folders []mitemfolder.ItemFolder, apis []mitemapi.ItemApi) (*CollectionPair, error) {
	var collection CollectionPair
	sortedFolders := SortFoldersByUlidTime(folders)
	sortedFolderIds := make([]ulid.ULID, len(sortedFolders))
	for i, item := range sortedFolders {
		sortedFolderIds[i] = item.ID
	}
	folderMap := make(map[ulid.ULID]mitemfolder.ItemFolderNested, len(sortedFolders))
	for _, item := range sortedFolders {
		folderMap[item.ID] = mitemfolder.ItemFolderNested{
			ItemFolder: item,
			Children:   []interface{}{},
		}
	}

	for _, api := range apis {
		if api.ParentID != nil {
			folder, ok := folderMap[*api.ParentID]
			if ok {
				folder.Children = append(folder.Children, api)
				folderMap[*api.ParentID] = folder
			} else {
				return nil, fmt.Errorf("Parent folder not found %s", api.ParentID)
			}
		} else {
			collection.itemApis = append(collection.itemApis, api)
		}
	}

	for _, folder := range sortedFolderIds {
		folder := folderMap[folder]
		if folder.ParentID != nil {
			parentFolder, ok := folderMap[*folder.ParentID]
			if ok {
				parentFolder.Children = append(parentFolder.Children, folder)
				folderMap[*folder.ParentID] = parentFolder
			} else {
				return nil, fmt.Errorf("Parent folder not found %s", folder.ParentID)
			}
		} else {
			collection.itemFolders = append(collection.itemFolders, folder)
		}
	}

	return &collection, nil
}

func SortFoldersByUlidTime(folders []mitemfolder.ItemFolder) []mitemfolder.ItemFolder {
	sortedFolders := make([]mitemfolder.ItemFolder, len(folders))
	copy(sortedFolders, folders)

	// quick sort by ulid timestamp in descending order
	quickSort(sortedFolders, 0, len(sortedFolders)-1)

	return sortedFolders
}

func quickSort(arr []mitemfolder.ItemFolder, low, high int) {
	if low < high {
		pi := partition(arr, low, high)
		quickSort(arr, low, pi-1)
		quickSort(arr, pi+1, high)
	}
}

func partition(arr []mitemfolder.ItemFolder, low, high int) int {
	pivot := arr[high].ID.Time()
	i := low - 1
	for j := low; j < high; j++ {
		if arr[j].ID.Time() > pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}
