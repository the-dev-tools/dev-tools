package tpostman

import (
	"devtools-backend/pkg/model/mcollection/mitemapi"
	"devtools-backend/pkg/model/postman/v21/mpostmancollection"
	"devtools-backend/pkg/model/postman/v21/murl"
	"encoding/json"

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

func ConvertPostmanCollection(collection mpostmancollection.Collection, collectionID ulid.ULID, ownerID string) []mitemapi.ItemApi {
	var collectionNodes []mitemapi.ItemApi

	for _, item := range collection.Items {

		headers := make(map[string]string)
		for _, v := range item.Request.Header {
			if v.Disabled {
				continue
			}
			headers[v.Key] = v.Value
		}

		ulidID := ulid.Make()
		urlRaw, ok := item.Request.URL.(string)
		if ok {
			collectionItem := mitemapi.ItemApi{
				ID:           ulidID,
				CollectionID: collectionID,
				Name:         item.Name,
				Url:          urlRaw,
				Method:       item.Request.Method,
				QueryParams: mitemapi.QueryParams{
					QueryMap: map[string]string{},
				},
				Headers: mitemapi.Headers{
					HeaderMap: headers,
				},
				Body: []byte(item.Request.Body.Raw),
			}

			collectionNodes = append(collectionNodes, collectionItem)
			continue
		}

		urlObject, ok := item.Request.URL.(murl.URL)
		if ok {

			queryParams := make(map[string]string)

			for _, v := range urlObject.Query {
				queryParams[v.Key] = v.Value
			}

			collectionItem := mitemapi.ItemApi{
				ID:           ulidID,
				CollectionID: collectionID,
				Name:         item.Name,
				Url:          urlObject.Raw,
				Method:       item.Request.Method,
				QueryParams: mitemapi.QueryParams{
					QueryMap: queryParams,
				},
				Headers: mitemapi.Headers{
					HeaderMap: headers,
				},
				Body: []byte(item.Request.Body.Raw),
			}

			collectionNodes = append(collectionNodes, collectionItem)
		}

	}

	return collectionNodes
}
