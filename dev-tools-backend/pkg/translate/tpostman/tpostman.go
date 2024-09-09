package tpostman

import (
	"bytes"
	"compress/gzip"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/model/postman/v21/mitem"
	"dev-tools-backend/pkg/model/postman/v21/mpostmancollection"
	"dev-tools-backend/pkg/model/postman/v21/mresponse"
	"dev-tools-backend/pkg/model/postman/v21/murl"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

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
	ApiExample []mitemapiexample.ItemApiExample
	Api        []mitemapi.ItemApi
	Folder     []mitemfolder.ItemFolder
}

type ItemChannels struct {
	ApiExample chan mitemapiexample.ItemApiExample
	Api        chan mitemapi.ItemApi
	Folder     chan mitemfolder.ItemFolder
	Wg         *sync.WaitGroup
	Err        chan error
	Done       chan struct{}
}

func ConvertPostmanCollection(collection mpostmancollection.Collection, collectionID ulid.ULID) (*ItemsPair, error) {
	var pair ItemsPair = ItemsPair{
		Api:    make([]mitemapi.ItemApi, 0, len(collection.Items)),
		Folder: make([]mitemfolder.ItemFolder, 0, len(collection.Items)),
	}

	var wg sync.WaitGroup
	ItemChannels := ItemChannels{
		ApiExample: make(chan mitemapiexample.ItemApiExample),
		Api:        make(chan mitemapi.ItemApi),
		Folder:     make(chan mitemfolder.ItemFolder),
		Wg:         &wg,
		Done:       make(chan struct{}),
		Err:        make(chan error),
	}

	afterTime := time.After(2 * time.Minute)

	for _, item := range collection.Items {
		wg.Add(1)
		go func() {
			// means it is a folder
			if item.Request == nil {
				rootFolder := mitemfolder.ItemFolder{
					ID:           ulid.Make(),
					Name:         item.Name,
					ParentID:     nil,
					CollectionID: collectionID,
				}
				ItemChannels.Folder <- rootFolder
				go GetRecursiveFolders(item, &rootFolder.ID, collectionID, &ItemChannels)
			} else {
				go GetRequest(item, nil, collectionID, &ItemChannels)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ItemChannels.Done)
	}()

	for {
		select {
		case <-afterTime:
			return nil, errors.New("timeout")
		case err := <-ItemChannels.Err:
			return nil, err
		case folder := <-ItemChannels.Folder:
			pair.Folder = append(pair.Folder, folder)
		case api := <-ItemChannels.Api:
			pair.Api = append(pair.Api, api)
		case apiExample := <-ItemChannels.ApiExample:
			pair.ApiExample = append(pair.ApiExample, apiExample)
		case <-ItemChannels.Done:
			return &pair, nil
		}
	}
}

func GetRecursiveFolders(item *mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, channels *ItemChannels) {
	defer channels.Wg.Done()
	for _, item := range item.Items {
		channels.Wg.Add(1)
		if item.Request == nil {
			folder := mitemfolder.ItemFolder{
				ID:           ulid.Make(),
				Name:         item.Name,
				ParentID:     parentID,
				CollectionID: collectionID,
			}
			channels.Folder <- folder
			go GetRecursiveFolders(item, &folder.ID, collectionID, channels)
		} else {
			go GetRequest(item, parentID, collectionID, channels)
		}
	}
}

func GetRequest(item *mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, channels *ItemChannels) {
	defer channels.Wg.Done()
	headers := make(map[string]string)

	if item.Request == nil {
		channels.Err <- errors.New("item is not an api")
		return
	}

	if item.Request.Header != nil {
		for _, v := range item.Request.Header {
			if !v.Disabled {
				headers[v.Key] = v.Value
			}
		}
	}

	ulidID := ulid.Make()
	queryParams := make(map[string]string)
	var rawURL string
	switch item.Request.URL.(type) {
	case string:
		rawURL = item.Request.URL.(string)
	case murl.URL:
		url := item.Request.URL.(murl.URL)
		rawURL = url.Raw
		for _, v := range url.Query {
			queryParams[v.Key] = v.Value
		}
	default:
		channels.Err <- errors.New("url is not a string or murl.URL")
		return
	}
	var bodyBuff bytes.Buffer
	compresed := false
	if item.Request.Body != nil {
		if len(item.Request.Body.Raw) > 1000 {
			compresed = true
			w := gzip.NewWriter(&bodyBuff)
			w.Write([]byte(item.Request.Body.Raw))
		} else {
			bodyBuff.Write([]byte(item.Request.Body.Raw))
		}
	}

	api := mitemapi.ItemApi{
		ID:           ulidID,
		CollectionID: collectionID,
		ParentID:     parentID,
		Name:         item.Name,
		Url:          rawURL,
		Method:       item.Request.Method,
	}

	channels.Api <- api

	buffBytes := bodyBuff.Bytes()
	example := mitemapiexample.NewItemApiExample(ulid.Make(), ulidID, collectionID, nil, true,
		"Default Example", *mitemapiexample.NewHeaders(headers), *mitemapiexample.NewQuery(queryParams), compresed, buffBytes)
	channels.ApiExample <- *example

	for _, v := range item.Responses {
		name := v.Name
		if name == "" {
			name = fmt.Sprintf("Example %d of %s", v.Code, api.Name)
		}
		channels.Wg.Add(1)
		go GetResponse(v, buffBytes, name, queryParams, ulidID, collectionID, channels)
	}

	return
}

func GetResponse(item *mresponse.Response, body []byte, name string, queryParams map[string]string,
	apiID, collectionID ulid.ULID, channels *ItemChannels,
) {
	defer channels.Wg.Done()
	headers := make(map[string]string)
	for _, v := range item.Headers {
		headers[v.Key] = v.Value
	}

	cookies := make(map[string]string)
	for _, v := range item.Cookies {
		cookies[v.Name] = v.Value
	}

	apiExample := mitemapiexample.ItemApiExample{
		ID:           ulid.Make(),
		ItemApiID:    apiID,
		CollectionID: collectionID,
		Name:         name,
		IsDefault:    false,
		Headers:      *mitemapiexample.NewHeaders(headers),
		Cookies:      *mitemapiexample.NewCookies(cookies),
		Query:        *mitemapiexample.NewQuery(queryParams),
		Body:         body,
	}

	channels.ApiExample <- apiExample
}

func RemoveItem[I any](slice []I, s int) []I {
	if s >= len(slice) {
		return slice
	}
	return append(slice[:s], slice[s+1:]...)
}

func GetQueryParams(urlStr string) (map[string]string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	queryParams := make(map[string]string)
	for k, v := range u.Query() {
		queryParams[k] = v[0]
	}
	return queryParams, nil
}
