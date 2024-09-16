package titemnest

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/model/mitemfolder"
	itemapiv1 "dev-tools-services/gen/itemapi/v1"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	itemfolderv1 "dev-tools-services/gen/itemfolder/v1"
	"fmt"
)

type CollectionPair struct {
	itemFolders []mitemfolder.ItemFolderNested
	itemApis    []mitemapi.ItemApiWithExamples
}

// TODO: can be more efficient by MultiThreading
func (c CollectionPair) GetItemsFull() []*itemfolderv1.Item {
	items := make([]*itemfolderv1.Item, 0, len(c.itemApis)+len(c.itemFolders))

	for _, item := range c.itemFolders {
		folderItem := &itemfolderv1.Item{
			Data: &itemfolderv1.Item_Folder{
				Folder: &itemfolderv1.Folder{
					Meta: &itemfolderv1.FolderMeta{
						Id:   item.ID.String(),
						Name: item.Name,
					},
					Items: RecursiveTranslateFull(item),
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
						Name:   item.Name,
						Id:     item.ID.String(),
						Method: item.Method,
					},
					CollectionId: item.CollectionID.String(),
					Url:          item.Url,
				},
			},
		}
		items = append(items, apiItem)
	}

	return items
}

func (c CollectionPair) GetItemsMeta() []*itemfolderv1.ItemMeta {
	items := make([]*itemfolderv1.ItemMeta, len(c.itemApis)+len(c.itemFolders))

	for i, item := range c.itemFolders {
		folderItem := &itemfolderv1.ItemMeta{
			Meta: &itemfolderv1.ItemMeta_FolderMeta{
				FolderMeta: &itemfolderv1.FolderMeta{
					Id:    item.ID.String(),
					Name:  item.Name,
					Items: RecursiveTranslateMeta(item),
				},
			},
		}
		items[i] = folderItem
	}

	index := len(c.itemFolders)

	for i, item := range c.itemApis {
		apiItem := &itemfolderv1.ItemMeta{
			Meta: &itemfolderv1.ItemMeta_ApiCallMeta{
				ApiCallMeta: &itemapiv1.ApiCallMeta{
					Name: item.Name,
					Id:   item.ID.String(),
				},
			},
		}
		items[index+i] = apiItem
	}

	return items
}

func RecursiveTranslateFull(item mitemfolder.ItemFolderNested) []*itemfolderv1.Item {
	var items []*itemfolderv1.Item
	for _, child := range item.Children {
		switch child.(type) {
		case mitemfolder.ItemFolderNested:
			folder := child.(mitemfolder.ItemFolderNested)
			folderCollection := &itemfolderv1.Item{
				Data: &itemfolderv1.Item_Folder{
					Folder: &itemfolderv1.Folder{
						Meta: &itemfolderv1.FolderMeta{
							Id:   folder.ID.String(),
							Name: folder.Name,
						},

						ParentId: folder.ParentID.String(),
						Items:    RecursiveTranslateFull(folder),
					},
				},
			}
			items = append(items, folderCollection)
		case mitemapi.ItemApiWithExamples:
			api := child.(mitemapi.ItemApiWithExamples)
			rpcExamples := make([]*itemapiexamplev1.ApiExampleMeta, len(api.Examples))
			for i, example := range api.Examples {
				rpcExamples[i] = &itemapiexamplev1.ApiExampleMeta{
					Id:   example.ID.String(),
					Name: example.Name,
				}
			}

			item := &itemfolderv1.Item{
				Data: &itemfolderv1.Item_ApiCall{
					ApiCall: &itemapiv1.ApiCall{
						Meta: &itemapiv1.ApiCallMeta{
							Name:     api.Name,
							Id:       api.ID.String(),
							Examples: rpcExamples,
							Method:   api.Method,
						},
						ParentId: api.ParentID.String(),
						Url:      api.Url,
					},
				},
			}
			items = append(items, item)
		default:
			return nil
		}
	}

	return items
}

func RecursiveTranslateMeta(item mitemfolder.ItemFolderNested) []*itemfolderv1.ItemMeta {
	items := make([]*itemfolderv1.ItemMeta, len(item.Children))
	for i, child := range item.Children {
		switch child.(type) {
		case mitemfolder.ItemFolderNested:
			folder := child.(mitemfolder.ItemFolderNested)
			folderCollection := &itemfolderv1.ItemMeta{
				Meta: &itemfolderv1.ItemMeta_FolderMeta{
					FolderMeta: &itemfolderv1.FolderMeta{
						Id:    folder.ID.String(),
						Name:  folder.Name,
						Items: RecursiveTranslateMeta(folder),
					},
				},
			}
			items[i] = folderCollection
		case mitemapi.ItemApiWithExamples:
			api := child.(mitemapi.ItemApiWithExamples)
			rpcExamples := make([]*itemapiexamplev1.ApiExampleMeta, len(api.Examples))
			for i, example := range api.Examples {
				rpcExamples[i] = &itemapiexamplev1.ApiExampleMeta{
					Id:   example.ID.String(),
					Name: example.Name,
				}
			}

			item := &itemfolderv1.ItemMeta{
				Meta: &itemfolderv1.ItemMeta_ApiCallMeta{
					ApiCallMeta: &itemapiv1.ApiCallMeta{
						Name:     api.Name,
						Id:       api.ID.String(),
						Method:   api.Method,
						Examples: rpcExamples,
					},
				},
			}
			items[i] = item
		default:
			return nil
		}
	}

	return items
}

// sort and create root fodler and check sub folder recoversive
// also put api with parentID in the folder
func TranslateItemFolderNested(folders []mitemfolder.ItemFolder, apis []mitemapi.ItemApi,
	examples []mitemapiexample.ItemApiExample,
) (*CollectionPair, error) {
	var collection CollectionPair
	sortedFolders := SortFoldersByUlidTime(folders)
	sortedFolderIds := make([]idwrap.IDWrap, len(sortedFolders))
	for i, item := range sortedFolders {
		sortedFolderIds[i] = item.ID
	}
	folderMap := make(map[idwrap.IDWrap]mitemfolder.ItemFolderNested, len(sortedFolders))
	for _, item := range sortedFolders {
		folderMap[item.ID] = mitemfolder.ItemFolderNested{
			ItemFolder: item,
			Children:   []interface{}{},
		}
	}

	apiMap := make(map[idwrap.IDWrap]mitemapi.ItemApiWithExamples, len(apis))
	for _, item := range apis {
		apiMap[item.ID] = mitemapi.ItemApiWithExamples{
			ItemApi:        item,
			DefaultExample: mitemapiexample.ItemApiExample{},
			Examples:       []mitemapiexample.ItemApiExampleMeta{},
		}
	}

	exampleMap := make(map[idwrap.IDWrap]mitemapiexample.ItemApiExample, len(examples))
	for _, item := range examples {
		exampleMap[item.ID] = item
	}

	for _, example := range examples {
		if example.IsDefault {
			api, ok := apiMap[example.ItemApiID]
			if !ok {
				return nil, fmt.Errorf("Parent Api not found for example %s", api.ParentID)
			}
			api.DefaultExample = example
			continue
		}
		if example.Prev != nil {
			continue
		}
		api, ok := apiMap[example.ItemApiID]
		if !ok {
			return nil, fmt.Errorf("Parent Api not found for example %s", api.ParentID)
		}
		for {
			meta := mitemapiexample.ItemApiExampleMeta{
				ID:   example.ID,
				Name: example.Name,
			}
			api.Examples = append(api.Examples, meta)
			if example.Next == nil {
				break
			}
			example = exampleMap[*example.Next]
		}
		apiMap[api.ID] = api
	}

	for _, api := range apiMap {
		if api.ParentID == nil {
			collection.itemApis = append(collection.itemApis, api)
			continue
		}
		if api.Prev != nil {
			continue
		}
		folder, ok := folderMap[*api.ParentID]
		if !ok {
			return nil, fmt.Errorf("Parent folder not found %s", api.ParentID)
		}
		for {
			folder.Children = append(folder.Children, api)
			if api.Next == nil {
				break
			}
			api = apiMap[*api.Next]
		}
		folderMap[folder.ID] = folder
	}

	for _, folder := range sortedFolderIds {
		folder := folderMap[folder]
		if folder.ParentID == nil {
			collection.itemFolders = append(collection.itemFolders, folder)
			continue
		}
		if folder.Prev != nil {
			continue
		}

		parentFolder, ok := folderMap[*folder.ParentID]
		if !ok {
			return nil, fmt.Errorf("Parent folder not found for folder %s", folder.ParentID)
		}

		for {
			parentFolder.Children = append(parentFolder.Children, folder)
			folderMap[*folder.ParentID] = parentFolder
			if folder.Next == nil {
				break
			}
			folder = folderMap[*folder.Next]
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
	pivot := arr[high].ID
	i := low - 1
	for j := low; j < high; j++ {
		if arr[j].ID.Compare(pivot) == 1 {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}
