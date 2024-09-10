package tpostman

import (
	"bytes"
	"compress/gzip"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/model/postman/v21/mheader"
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

func SendAllToChannel[I any](items []I, ch chan I) {
	for _, item := range items {
		ch <- item
	}
}

func SendAllToChannelPtr[I any](items []*I, ch chan I) {
	for _, item := range items {
		ch <- *item
	}
}

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
	Headers    []mexampleheader.Header
	// TODO: add query params and body
}

type ItemChannels struct {
	ApiExample chan mitemapiexample.ItemApiExample
	Api        chan mitemapi.ItemApi
	Folder     chan mitemfolder.ItemFolder
	Header     chan mexampleheader.Header
	Wg         *sync.WaitGroup
	Err        chan error
	Done       chan struct{}
}

func ConvertPostmanCollection(collection mpostmancollection.Collection, collectionID ulid.ULID) (*ItemsPair, error) {
	var pair ItemsPair = ItemsPair{
		Api:     make([]mitemapi.ItemApi, 0, len(collection.Items)),
		Folder:  make([]mitemfolder.ItemFolder, 0, len(collection.Items)),
		Headers: make([]mexampleheader.Header, 0, len(collection.Items)*2),
	}

	var wg sync.WaitGroup
	ItemChannels := ItemChannels{
		ApiExample: make(chan mitemapiexample.ItemApiExample),
		Api:        make(chan mitemapi.ItemApi),
		Folder:     make(chan mitemfolder.ItemFolder),
		Header:     make(chan mexampleheader.Header),
		Wg:         &wg,
		Done:       make(chan struct{}),
		Err:        make(chan error),
	}

	afterTime := time.After(2 * time.Minute)

	wg.Add(1)
	go GetRecursiveRoots(collection.Items, collectionID, &ItemChannels)

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
		case header := <-ItemChannels.Header:
			pair.Headers = append(pair.Headers, header)
		case <-ItemChannels.Done:
			return &pair, nil
		}
	}
}

func GetRecursiveRoots(items []mitem.Items, collectionID ulid.ULID, channels *ItemChannels) {
	go GetRecursiveFolders(items, nil, collectionID, channels)
}

func GetRecursiveFolders(items []mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, channels *ItemChannels) {
	defer channels.Wg.Done()
	var folderPrev *mitemfolder.ItemFolder
	var folderArr []*mitemfolder.ItemFolder
	var apiArrRaw []*mitem.Items

	for _, item := range items {
		if item.Request == nil {
			folder := &mitemfolder.ItemFolder{
				ID:           ulid.Make(),
				Name:         item.Name,
				ParentID:     parentID,
				CollectionID: collectionID,
			}

			if folderPrev != nil {
				folderPrev.Next = &folder.ID
				folder.Prev = &folder.ID
			}

			folderPrev = folder
			folderArr = append(folderArr, folderPrev)

			channels.Wg.Add(1)
			go GetRecursiveFolders(item.Items, &folder.ID, collectionID, channels)
		} else {
			apiArrRaw = append(apiArrRaw, &item)
		}
	}

	channels.Wg.Add(1)
	go GetRequest(apiArrRaw, parentID, collectionID, channels)
	SendAllToChannelPtr(folderArr, channels.Folder)
}

func GetRequest(items []*mitem.Items, parentID *ulid.ULID, collectionID ulid.ULID, channels *ItemChannels) {
	defer channels.Wg.Done()
	var apiPrev *mitemapi.ItemApi
	var apiArr []*mitemapi.ItemApi
	for _, item := range items {
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

		api := &mitemapi.ItemApi{
			ID:           ulidID,
			CollectionID: collectionID,
			ParentID:     parentID,
			Name:         item.Name,
			Url:          rawURL,
			Method:       item.Request.Method,
		}

		if apiPrev != nil {
			apiPrev.Next = &api.ID
			api.Prev = &apiPrev.ID
		}

		apiPrev = api
		apiArr = append(apiArr, apiPrev)

		buffBytes := bodyBuff.Bytes()
		defaultExampleUlid := ulid.Make()
		example := mitemapiexample.NewItemApiExample(defaultExampleUlid, ulidID, collectionID, nil, true,
			"Default Example", compresed, buffBytes)

		channels.ApiExample <- *example
		if len(item.Responses) != 0 {
			channels.Wg.Add(1)
			headers := item.Responses[0].Headers
			fmt.Println(headers)
			go GetHeaders(headers, defaultExampleUlid, collectionID, channels)
		}

		channels.Wg.Add(1)
		go GetResponse(item.Responses, buffBytes, ulidID, collectionID, channels)
	}

	SendAllToChannelPtr(apiArr, channels.Api)
}

func GetResponse(items []mresponse.Response, body []byte,
	apiUlid, collectionID ulid.ULID, channels *ItemChannels,
) {
	defer channels.Wg.Done()
	var prevExample *mitemapiexample.ItemApiExample
	var examples []*mitemapiexample.ItemApiExample

	for _, item := range items {

		cookies := make(map[string]string)
		for _, v := range item.Cookies {
			cookies[v.Name] = v.Value
		}
		if item.Name == "" {
			item.Name = "Untitled"
		}

		apiExampleID := ulid.Make()
		apiExample := mitemapiexample.ItemApiExample{
			ID:           apiExampleID,
			ItemApiID:    apiUlid,
			CollectionID: collectionID,
			Name:         item.Name,
			IsDefault:    false,
			Body:         body,
		}

		channels.Wg.Add(1)
		go GetHeaders(item.Headers, apiExampleID, collectionID, channels)

		if prevExample != nil {
			prevExample.Next = &apiExample.ID
			apiExample.Prev = &prevExample.ID
		}

		prevExample = &apiExample
		examples = append(examples, prevExample)
	}

	SendAllToChannelPtr(examples, channels.ApiExample)
}

func GetHeaders(headers []mheader.Header, exampleID ulid.ULID, collectionID ulid.ULID, channels *ItemChannels) {
	defer channels.Wg.Done()
	var headerArr []mexampleheader.Header
	for _, item := range headers {
		header := mexampleheader.Header{
			ID:           ulid.Make(),
			ExampleID:    exampleID,
			CollectionID: collectionID,
			HeaderKey:    item.Key,
			Enable:       !item.Disabled,
			Description:  item.Description,
			Value:        item.Value,
		}
		// TODO: add ordering
		headerArr = append(headerArr, header)
	}
	for _, header := range headerArr {
		channels.Header <- header
	}
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
