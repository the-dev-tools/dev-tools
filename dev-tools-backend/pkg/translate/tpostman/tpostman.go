package tpostman

import (
	"dev-tools-backend/pkg/model/mcollection/mitemapi"
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	"dev-tools-backend/pkg/model/postman/v21/mitem"
	"dev-tools-backend/pkg/model/postman/v21/mpostmancollection"
	"dev-tools-backend/pkg/model/postman/v21/murl"
	"errors"
	"sync"

	"github.com/goccy/go-json"

	"github.com/oklog/ulid/v2"
)

func ParsePostmanCollection(data []byte) (mpostmancollection.Collection, error) {
	var collection mpostmancollection.Collection

	err := json.Unmarshal(data, &collection)
	if err != nil {
		return collection, err
	}
	return collection, nil
}

type ItemsPair struct {
	mx     *sync.Mutex
	Api    []mitemapi.ItemApi
	Folder []mitemfolder.ItemFolder
}

func ConvertPostmanCollection(collection mpostmancollection.Collection, collectionID ulid.ULID) (*ItemsPair, error) {
	var pair ItemsPair = ItemsPair{
		mx:     &sync.Mutex{},
		Api:    make([]mitemapi.ItemApi, 0, len(collection.Items)),
		Folder: make([]mitemfolder.ItemFolder, 0, len(collection.Items)),
	}

	var wg sync.WaitGroup
	var errChan chan error
	waitCh := make(chan struct{})

	go func() {
		for _, item := range collection.Items {
			// means it is a folder
			if item.Request == nil {
				rootFolder := mitemfolder.ItemFolder{
					ID:           ulid.Make(),
					Name:         item.Name,
					ParentID:     nil,
					CollectionID: collectionID,
				}
				pair.Folder = append(pair.Folder, rootFolder)
				wg.Add(1)
				go GetRecursiveFolders(item, &rootFolder.ID, collectionID, &pair, &wg, errChan)
			} else {
				wg.Add(1)
				go GetRequest(item, nil, collectionID, &pair, &wg, errChan)
			}
		}
		wg.Wait()
		close(waitCh)
	}()

	select {
	case err := <-errChan:
		return nil, err
	case <-waitCh:
		return &pair, nil
	}
}

func GetRecursiveFolders(item *mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, pair *ItemsPair, wg *sync.WaitGroup, errChan chan error) {
	for _, item := range item.Items {
		if item.Request == nil {
			folder := mitemfolder.ItemFolder{
				ID:           ulid.Make(),
				Name:         item.Name,
				ParentID:     parentID,
				CollectionID: collectionID,
			}
			pair.mx.Lock()
			pair.Folder = append(pair.Folder, folder)
			pair.mx.Unlock()
			wg.Add(1)
			go GetRecursiveFolders(item, &folder.ID, collectionID, pair, wg, errChan)
		} else {
			wg.Add(1)
			go GetRequest(item, parentID, collectionID, pair, wg, errChan)
		}
	}
	wg.Done()
}

func GetRequest(item *mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, pair *ItemsPair, wg *sync.WaitGroup, errChan chan error) {
	headers := make(map[string]string)

	if item.Request == nil {
		errChan <- errors.New("item is not an api")
	}

	if item.Request.Header != nil {
		for _, v := range item.Request.Header {
			if !v.Disabled {
				headers[v.Key] = v.Value
			}
		}
	}

	ulidID := ulid.Make()
	urlRaw, ok := item.Request.URL.(string)

	var bodyData []byte
	if item.Request.Body == nil {
		bodyData = nil
	} else {
		bodyData = []byte(item.Request.Body.Raw)
	}

	if ok {
		api := mitemapi.ItemApi{
			ID:           ulidID,
			CollectionID: collectionID,
			ParentID:     parentID,
			Name:         item.Name,
			Url:          urlRaw,
			Method:       item.Request.Method,
			Query: mitemapi.Query{
				QueryMap: map[string]string{},
			},
			Headers: mitemapi.Headers{
				HeaderMap: headers,
			},
			Body: bodyData,
		}

		pair.mx.Lock()
		pair.Api = append(pair.Api, api)
		pair.mx.Unlock()
		wg.Done()
		return
	}

	urlObject, ok := item.Request.URL.(murl.URL)
	if ok {

		queryParams := make(map[string]string)

		for _, v := range urlObject.Query {
			queryParams[v.Key] = v.Value
		}

		api := mitemapi.ItemApi{
			ID:           ulidID,
			CollectionID: collectionID,
			ParentID:     parentID,
			Name:         item.Name,
			Url:          urlObject.Raw,
			Method:       item.Request.Method,
			Query: mitemapi.Query{
				QueryMap: queryParams,
			},
			Headers: mitemapi.Headers{
				HeaderMap: headers,
			},
			Body: bodyData,
		}
		pair.mx.Lock()
		pair.Api = append(pair.Api, api)
		pair.mx.Unlock()

		wg.Done()
		return
	}

	mapObject, ok := item.Request.URL.(map[string]interface{})
	if ok {
		url, ok := mapObject["raw"].(string)
		if !ok {
			errChan <- errors.New("url is not a string")
		}
		api := mitemapi.ItemApi{
			ID:           ulidID,
			CollectionID: collectionID,
			ParentID:     parentID,
			Name:         item.Name,
			Url:          url,
			Method:       item.Request.Method,
			Query: mitemapi.Query{
				QueryMap: make(map[string]string, 0),
			},
			Headers: mitemapi.Headers{
				HeaderMap: headers,
			},
			Body: bodyData,
		}
		pair.mx.Lock()
		pair.Api = append(pair.Api, api)
		pair.mx.Unlock()
		wg.Done()
		return
	}

	errChan <- errors.New("url is not a string or object")
}
