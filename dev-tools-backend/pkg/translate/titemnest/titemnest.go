package titemnest

import (
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemfolder"
	itemapiv1 "dev-tools-services/gen/itemapi/v1"
	itemfolderv1 "dev-tools-services/gen/itemfolder/v1"
	"fmt"

	"github.com/oklog/ulid/v2"
)

type CollectionPair struct {
	itemFolders []mitemfolder.ItemFolderNested
	itemApis    []mitemapi.ItemApi
}

// TODO: can be more efficient by MultiThreading
func (c CollectionPair) GetItemFolders() []*itemfolderv1.Item {
	items := make([]*itemfolderv1.Item, 0, len(c.itemApis)+len(c.itemFolders))

	for _, item := range c.itemFolders {
		folderItem := &itemfolderv1.Item{
			Data: &itemfolderv1.Item_Folder{
				Folder: &itemfolderv1.Folder{
					Meta: &itemfolderv1.FolderMeta{
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
		apiItem := &itemfolderv1.Item{
			Data: &itemfolderv1.Item_ApiCall{
				ApiCall: &itemapiv1.ApiCall{
					Meta: &itemapiv1.ApiCallMeta{
						Name: item.Name,
						Id:   item.ID.String(),
					},
					CollectionId: item.CollectionID.String(),
					Url:          item.Url,
					Method:       item.Method,
				},
			},
		}
		items = append(items, apiItem)
	}

	return items
}

func RecursiveTranslate(item mitemfolder.ItemFolderNested) []*itemfolderv1.Item {
	var items []*itemfolderv1.Item
	for _, child := range item.Children {
		folder, ok := child.(mitemfolder.ItemFolderNested)
		if ok {
			folderCollection := &itemfolderv1.Item{
				Data: &itemfolderv1.Item_Folder{
					Folder: &itemfolderv1.Folder{
						Meta: &itemfolderv1.FolderMeta{
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
				item := &itemfolderv1.Item{
					Data: &itemfolderv1.Item_ApiCall{
						ApiCall: &itemapiv1.ApiCall{
							Meta: &itemapiv1.ApiCallMeta{
								Name: api.Name,
								Id:   api.ID.String(),
							},
							ParentId: api.ParentID.String(),
							Url:      api.Url,
							Method:   api.Method,
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
