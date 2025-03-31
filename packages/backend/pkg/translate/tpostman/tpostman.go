package tpostman

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/model/mitemfolder"
	"the-dev-tools/backend/pkg/model/postman/v21/mbody"
	"the-dev-tools/backend/pkg/model/postman/v21/mheader"
	"the-dev-tools/backend/pkg/model/postman/v21/mitem"
	"the-dev-tools/backend/pkg/model/postman/v21/mpostmancollection"
	"the-dev-tools/backend/pkg/model/postman/v21/mresponse"
	"the-dev-tools/backend/pkg/model/postman/v21/murl"
	"the-dev-tools/backend/pkg/model/postman/v21/mvariable"
	"the-dev-tools/backend/pkg/zstdcompress"
	"time"

	"github.com/goccy/go-json"
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
	ApiExamples    []mitemapiexample.ItemApiExample
	Apis           []mitemapi.ItemApi
	Folders        []mitemfolder.ItemFolder
	Headers        []mexampleheader.Header
	Queries        []mexamplequery.Query
	BodyForm       []mbodyform.BodyForm
	BodyUrlEncoded []mbodyurl.BodyURLEncoded
	BodyRaw        []mbodyraw.ExampleBodyRaw
}

type ItemChannels struct {
	ApiExample     chan mitemapiexample.ItemApiExample
	Api            chan mitemapi.ItemApi
	Folder         chan mitemfolder.ItemFolder
	Header         chan []mexampleheader.Header
	Query          chan []mexamplequery.Query
	BodyForm       chan []mbodyform.BodyForm
	BodyUrlEncoded chan []mbodyurl.BodyURLEncoded
	BodyRaw        chan mbodyraw.ExampleBodyRaw
	Wg             *sync.WaitGroup
	Err            chan error
	Done           chan struct{}
}

func ConvertPostmanCollection(collection mpostmancollection.Collection, collectionID idwrap.IDWrap) (*ItemsPair, error) {
	pair := ItemsPair{
		Apis:    make([]mitemapi.ItemApi, 0, len(collection.Items)),
		Folders: make([]mitemfolder.ItemFolder, 0, len(collection.Items)),
		Headers: make([]mexampleheader.Header, 0, len(collection.Items)*2),
		Queries: make([]mexamplequery.Query, 0, len(collection.Items)*2),
	}

	var wg sync.WaitGroup
	ItemChannels := ItemChannels{
		ApiExample:     make(chan mitemapiexample.ItemApiExample),
		Api:            make(chan mitemapi.ItemApi),
		Folder:         make(chan mitemfolder.ItemFolder),
		Header:         make(chan []mexampleheader.Header),
		Query:          make(chan []mexamplequery.Query),
		BodyForm:       make(chan []mbodyform.BodyForm),
		BodyUrlEncoded: make(chan []mbodyurl.BodyURLEncoded),
		BodyRaw:        make(chan mbodyraw.ExampleBodyRaw),
		Wg:             &wg,
		Done:           make(chan struct{}),
		Err:            make(chan error),
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
			pair.Folders = append(pair.Folders, folder)
		case api := <-ItemChannels.Api:
			pair.Apis = append(pair.Apis, api)
		case apiExample := <-ItemChannels.ApiExample:
			pair.ApiExamples = append(pair.ApiExamples, apiExample)
		case header := <-ItemChannels.Header:
			pair.Headers = append(pair.Headers, header...)
		case query := <-ItemChannels.Query:
			pair.Queries = append(pair.Queries, query...)
		case bodyForm := <-ItemChannels.BodyForm:
			pair.BodyForm = append(pair.BodyForm, bodyForm...)
		case bodyUrlEncoded := <-ItemChannels.BodyUrlEncoded:
			pair.BodyUrlEncoded = append(pair.BodyUrlEncoded, bodyUrlEncoded...)
		case bodyRaw := <-ItemChannels.BodyRaw:
			pair.BodyRaw = append(pair.BodyRaw, bodyRaw)
		case <-ItemChannels.Done:
			return &pair, nil
		}
	}
}

func GetRecursiveRoots(items []mitem.Items, collectionID idwrap.IDWrap, channels *ItemChannels) {
	go GetRecursiveFolders(items, nil, collectionID, channels)
}

func GetRecursiveFolders(items []mitem.Items, parentID *idwrap.IDWrap, collectionID idwrap.IDWrap, channels *ItemChannels) {
	defer channels.Wg.Done()
	var folderPrev *mitemfolder.ItemFolder
	var folderArr []*mitemfolder.ItemFolder
	var apiArrRaw []*mitem.Items

	for _, item := range items {
		if item.Request == nil {
			folder := &mitemfolder.ItemFolder{
				ID:           idwrap.NewNow(),
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

func GetRequest(items []*mitem.Items, parentID *idwrap.IDWrap, collectionID idwrap.IDWrap, channels *ItemChannels) {
	defer channels.Wg.Done()

	var apiPrev *mitemapi.ItemApi
	var apiArr []*mitemapi.ItemApi
	for _, item := range items {
		if item.Request == nil {
			channels.Err <- errors.New("item is not an api")
			return
		}
		var URL *murl.URL
		var err error
		if item.Request.URL != nil {
			URL, err = GetQueryParams(item.Request.URL)
			if err != nil {
				channels.Err <- err
				return
			}
		}
		ApiID := idwrap.NewNow()
		api := &mitemapi.ItemApi{
			ID:           ApiID,
			CollectionID: collectionID,
			FolderID:     parentID,
			Name:         item.Name,
			Url:          URL.Raw,
			Method:       item.Request.Method,
		}

		if apiPrev != nil {
			apiPrev.Next = &api.ID
			api.Prev = &apiPrev.ID
		}

		apiPrev = api
		apiArr = append(apiArr, apiPrev)

		channels.Wg.Add(1)
		go GetResponse(item.Responses, item.Request.Header, item.Request.Body, URL, ApiID, collectionID, channels)
	}

	SendAllToChannelPtr(apiArr, channels.Api)
}

func GetResponse(items []mresponse.Response, reqHeaders []mheader.Header, body *mbody.Body, urlData *murl.URL,
	apiUlid, collectionID idwrap.IDWrap, channels *ItemChannels,
) {
	defer channels.Wg.Done()
	var prevExample *mitemapiexample.ItemApiExample
	var examples []*mitemapiexample.ItemApiExample

	var defaultExample *mresponse.Response
	if len(items) != 0 {
		ExampleLastDefault := items[len(items)-1]
		ExampleLastDefault.Name = "Default Example"
		defaultExample = &ExampleLastDefault
	} else {
		emptyExample := mresponse.Response{
			Name: "Empty Example",
		}
		defaultExample = &emptyExample
	}
	items = append(items, *defaultExample)

	for i, item := range items {
		isDefault := i == len(items)-1

		cookies := make(map[string]string)
		for _, v := range item.Cookies {
			cookies[v.Name] = v.Value
		}
		if item.Name == "" {
			item.Name = "Untitled"
		}

		apiExampleID := idwrap.NewNow()
		apiExample := mitemapiexample.ItemApiExample{
			ID:           apiExampleID,
			ItemApiID:    apiUlid,
			CollectionID: collectionID,
			Name:         item.Name,
			IsDefault:    isDefault,
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		if body != nil {
			apiExample.BodyType = BodyType(body.Mode)
			channels.Wg.Add(1)
			go GetBody(body, apiExampleID, collectionID, channels)
		} else {
			apiExample.BodyType = mitemapiexample.BodyTypeRaw
			bodyRaw := mbodyraw.ExampleBodyRaw{
				ID:            idwrap.NewNow(),
				ExampleID:     apiExampleID,
				VisualizeMode: mbodyraw.VisualizeModeUndefined,
				CompressType:  mbodyraw.CompressTypeNone,
				Data:          []byte{},
			}
			channels.BodyRaw <- bodyRaw
		}
		if len(reqHeaders) > 0 {
			channels.Wg.Add(1)
			go GetHeaders(reqHeaders, apiExampleID, collectionID, channels)
		}
		if len(urlData.Query) > 0 {
			channels.Wg.Add(1)
			go GetQueries(urlData.Query, apiExampleID, collectionID, channels)
		}

		if prevExample != nil && !isDefault {
			prevExample.Next = &apiExample.ID
			apiExample.Prev = &prevExample.ID
		}

		prevExample = &apiExample
		examples = append(examples, prevExample)
	}

	SendAllToChannelPtr(examples, channels.ApiExample)
}

func GetHeaders(headers []mheader.Header, exampleID, collectionID idwrap.IDWrap, channels *ItemChannels) {
	defer channels.Wg.Done()
	headerArr := make([]mexampleheader.Header, len(headers))
	for i, item := range headers {
		header := mexampleheader.Header{
			ID:          idwrap.NewNow(),
			ExampleID:   exampleID,
			HeaderKey:   item.Key,
			Enable:      !item.Disabled,
			Description: item.Description,
			Value:       item.Value,
		}
		headerArr[i] = header
		// TODO: add ordering
	}
	channels.Header <- headerArr
}

func GetQueries(queries []murl.QueryParamter, exampleID, collectionID idwrap.IDWrap, channels *ItemChannels) {
	defer channels.Wg.Done()
	queryArr := make([]mexamplequery.Query, len(queries))
	for i, item := range queries {
		query := mexamplequery.Query{
			ID:          idwrap.NewNow(),
			ExampleID:   exampleID,
			QueryKey:    item.Key,
			Enable:      !item.Disabled,
			Description: item.Description,
			Value:       item.Value,
		}
		queryArr[i] = query
		// TODO: add ordering
	}
	channels.Query <- queryArr
}

// returns RAW URL and GetQueryParams
func GetQueryParams(urlData interface{}) (*murl.URL, error) {
	var murlData murl.URL
	// TODO: put avarage size of query params
	switch urlType := urlData.(type) {
	case string:
		url, err := url.Parse(urlData.(string))
		if err != nil {
			return nil, err
		}
		queryParamsArr := make([]murl.QueryParamter, 0)
		for k, vArr := range url.Query() {
			for _, v := range vArr {
				queryParamsArr = append(queryParamsArr, murl.QueryParamter{
					Key:         k,
					Value:       v,
					Disabled:    false,
					Description: "",
				})
			}
		}
		url.RawQuery = ""
		murlData = murl.URL{
			Protocol:  url.Scheme,
			Host:      []string{url.Host},
			Raw:       url.String(),
			Port:      url.Port(),
			Variables: []mvariable.Variable{},
			Path:      strings.Split(url.Path, "/"),
			Query:     queryParamsArr,
			Hash:      url.Fragment,
		}
	case murl.URL:
		murlData = urlData.(murl.URL)
	case map[string]interface{}:
		urlDataNest := urlData.(map[string]interface{})
		// TODO: seems like ok can fail check later
		queryData, _ := urlDataNest["query"].([]interface{})

		queryParamsArr := make([]murl.QueryParamter, 0)

		raw, ok := urlDataNest["raw"].(string)
		if !ok {
			return nil, fmt.Errorf("raw %T not supported", raw)
		}
		rawData, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}

		if ok {
			for k, vArr := range rawData.Query() {
				for _, v := range vArr {
					queryParamsArr = append(queryParamsArr, murl.QueryParamter{
						Key:         k,
						Value:       v,
						Disabled:    false,
						Description: "",
					})
				}
			}
		} else {
			for _, query := range queryData {
				queryMap := query.(map[string]interface{})
				key, ok := queryMap["key"].(string)
				if !ok {
					return nil, fmt.Errorf("key %T not supported", key)
				}
				value, ok := queryMap["value"].(string)
				if !ok {
					value = ""
				}
				disabled, ok := queryMap["disabled"].(bool)
				if !ok {
					disabled = false
				}
				description, ok := queryMap["description"].(string)
				if !ok {
					description = ""
				}

				queryParamsArr = append(queryParamsArr, murl.QueryParamter{
					Key:         key,
					Value:       value,
					Disabled:    disabled,
					Description: description,
				})
			}
		}
		rawData.RawQuery = ""
		raw = rawData.String()

		protocol, ok := urlDataNest["protocol"].(string)
		if !ok {
			return nil, fmt.Errorf("protocol %T not supported", protocol)
		}
		hostInterface, ok := urlDataNest["host"].([]interface{})
		hosts := make([]string, len(hostInterface))
		for i, host := range hostInterface {
			hosts[i] = host.(string)
		}
		if !ok {
			return nil, fmt.Errorf("host %T not supported", urlDataNest["host"])
		}
		port, ok := urlDataNest["port"].(string)
		if !ok {
			port = "443"
		}
		variables, ok := urlDataNest["variables"].([]mvariable.Variable)
		if !ok {
			variables = []mvariable.Variable{}
		}
		hash, ok := urlDataNest["hash"].(string)
		if !ok {
			hash = ""
		}

		murlData = murl.URL{
			Raw:       raw,
			Protocol:  protocol,
			Host:      hosts,
			Port:      port,
			Variables: variables,
			Query:     queryParamsArr,
			Hash:      hash,
		}
	default:
		return nil, fmt.Errorf("url type %T not supported", urlType)
	}
	return &murlData, nil
}

func GetBody(body *mbody.Body, exampleID, collectionID idwrap.IDWrap, channels *ItemChannels) {
	defer channels.Wg.Done()
	switch body.Mode {
	case mbody.ModeFormData:
		formArr := make([]mbodyform.BodyForm, len(body.FormData))
		for i, v := range body.FormData {
			formArr[i] = mbodyform.BodyForm{
				ID:          idwrap.NewNow(),
				ExampleID:   exampleID,
				BodyKey:     v.Key,
				Enable:      !v.Disabled,
				Description: v.Description,
				Value:       v.Value,
			}
		}
		channels.BodyForm <- formArr
	case mbody.ModeURLEncoded:
		encodedArr := make([]mbodyurl.BodyURLEncoded, len(body.URLEncoded))
		for i, v := range body.URLEncoded {
			encodedArr[i] = mbodyurl.BodyURLEncoded{
				ID:          idwrap.NewNow(),
				ExampleID:   exampleID,
				BodyKey:     v.Key,
				Enable:      !v.Disabled,
				Description: v.Description,
				Value:       v.Value,
			}
		}
		channels.BodyUrlEncoded <- encodedArr
	case mbody.ModeRaw:
		rawBytes := []byte(body.Raw)
		bodyRaw := mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			VisualizeMode: mbodyraw.VisualizeModeUndefined,
		}
		if len(rawBytes) > zstdcompress.CompressThreshold {
			bodyRaw.CompressType = mbodyraw.CompressTypeZstd
			bodyRaw.Data = zstdcompress.Compress(rawBytes)
			if len(bodyRaw.Data) > len(rawBytes) {
				bodyRaw.CompressType = mbodyraw.CompressTypeNone
				bodyRaw.Data = rawBytes
			}
		} else {
			bodyRaw.CompressType = mbodyraw.CompressTypeNone
			bodyRaw.Data = rawBytes
		}

		channels.BodyRaw <- bodyRaw
		return
	default:
		channels.Err <- errors.New("body mode not supported")
	}
	rawDefault := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		VisualizeMode: mbodyraw.VisualizeModeUndefined,
		CompressType:  mbodyraw.CompressTypeNone,
		Data:          []byte{},
	}
	channels.BodyRaw <- rawDefault
}

func RemoveItem[I any](slice []I, s int) []I {
	if s >= len(slice) {
		return slice
	}
	return append(slice[:s], slice[s+1:]...)
}

func BodyType(bodyStr string) mitemapiexample.BodyType {
	switch bodyStr {
	case mbody.ModeRaw:
		return mitemapiexample.BodyTypeRaw
	case mbody.ModeFormData:
		return mitemapiexample.BodyTypeForm
	case mbody.ModeURLEncoded:
		return mitemapiexample.BodyTypeUrlencoded
	default:
		return mitemapiexample.BodyTypeNone
	}
}
