package tpostman

import (
	"devtools-backend/pkg/model/mcollection/mitemapi"
	"devtools-backend/pkg/model/mcollection/mitemfolder"
	"devtools-backend/pkg/model/postman/v21/mitem"
	"devtools-backend/pkg/model/postman/v21/mpostmancollection"
	"devtools-backend/pkg/model/postman/v21/murl"
	"encoding/json"
	"errors"
	"fmt"

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
	Api    []mitemapi.ItemApi
	Folder []mitemfolder.ItemFolder
}

func ConvertPostmanCollection(collection mpostmancollection.Collection, collectionID ulid.ULID, ownerID string) (*ItemsPair, error) {
	var pair ItemsPair

	for _, item := range collection.Items {
		// means it is a folder
		if item.Request == nil {
			err := GetRecursiveFolders(item, nil, collectionID, &pair)
			if err != nil {
				return nil, err
			}
			continue
		} else {
			err := GetRequest(item, nil, collectionID, &pair)
			if err != nil {
				fmt.Println("error: ", err)
			}

		}
	}

	// reverse all the items

	return &pair, nil
}

func GetRecursiveFolders(item *mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, pair *ItemsPair) error {
	for _, item := range item.Items {
		if item.Request == nil {
			fmt.Println("folder: ", item.Name)
			folder := mitemfolder.ItemFolder{
				ID:           ulid.Make(),
				Name:         item.Name,
				ParentID:     parentID,
				CollectionID: collectionID,
			}
			pair.Folder = append(pair.Folder, folder)
			GetRecursiveFolders(item, &folder.ID, collectionID, pair)
		} else {
			GetRequest(item, parentID, collectionID, pair)
		}
	}
	return nil
}

func GetRequest(item *mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, pair *ItemsPair) error {
	headers := make(map[string]string)
	fmt.Println(parentID)

	if item.Request == nil {
		return errors.New("item is not an api")
	}

	if item.Request.Header != nil {
		for _, v := range item.Request.Header {
			if v.Disabled {
				continue
			}
			headers[v.Key] = v.Value
		}
	}

	ulidID := ulid.Make()
	urlRaw, ok := item.Request.URL.(string)

	var bodyData []byte
	if item.Request.Body == nil {
		bodyData = []byte("")
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
			QueryParams: mitemapi.QueryParams{
				QueryMap: map[string]string{},
			},
			Headers: mitemapi.Headers{
				HeaderMap: headers,
			},
			Body: bodyData,
		}
		pair.Api = append(pair.Api, api)
		return nil
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
			QueryParams: mitemapi.QueryParams{
				QueryMap: queryParams,
			},
			Headers: mitemapi.Headers{
				HeaderMap: headers,
			},
			Body: bodyData,
		}
		pair.Api = append(pair.Api, api)
		return nil
	}

	return errors.New("url is not a string or object")
}
