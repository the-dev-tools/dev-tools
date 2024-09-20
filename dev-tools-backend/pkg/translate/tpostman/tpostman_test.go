package tpostman_test

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/model/postman/v21/mbody"
	"dev-tools-backend/pkg/model/postman/v21/mheader"
	"dev-tools-backend/pkg/model/postman/v21/mitem"
	"dev-tools-backend/pkg/model/postman/v21/mpostmancollection"
	"dev-tools-backend/pkg/model/postman/v21/mrequest"
	"dev-tools-backend/pkg/model/postman/v21/mresponse"
	"dev-tools-backend/pkg/model/postman/v21/murl"
	"dev-tools-backend/pkg/translate/tpostman"
	"net/url"
	"strings"
	"testing"
)

func TestTranslatePostmanOrder(test *testing.T) {
	NestedApiLen := 100
	PerNestedApiExampleLen := 10
	RootFolderLen := 10

	request := mrequest.Request{
		Method:      "GET",
		Header:      nil,
		Body:        nil,
		Description: "test",
		URL:         "http://localhost:8080",
	}

	response := mresponse.Response{
		Name:            "test",
		OriginalRequest: nil,
		ResponseTime:    0,
	}

	responses := []mresponse.Response{}
	for i := 0; i < PerNestedApiExampleLen; i++ {
		responses = append(responses, response)
	}

	item := mitem.Items{
		ID:          "test",
		Name:        "test",
		Responses:   responses,
		Request:     &request,
		Description: "test",
		Variables:   nil,
		Items:       nil,
	}

	// folder
	RootItem := mitem.Items{
		ID:          "test",
		Name:        "test",
		Request:     nil,
		Description: "test",
		Variables:   nil,
		Items:       nil,
	}

	for i := 0; i < NestedApiLen; i++ {
		RootItem.Items = append(RootItem.Items, item)
	}

	items := []mitem.Items{}
	for i := 0; i < RootFolderLen; i++ {
		items = append(items, RootItem)
	}

	postmanCollection := mpostmancollection.Collection{
		Auth: nil,
		Info: mpostmancollection.Info{
			Name: "test",
		},
		Items:     items,
		Events:    nil,
		Variables: nil,
	}

	collectionID := idwrap.NewNow()
	pairs, err := tpostman.ConvertPostmanCollection(postmanCollection, collectionID)
	if err != nil {
		test.Errorf("Error: %v", err)
	}

	if len(pairs.Folders) != RootFolderLen {
		test.Errorf("Error: %v", len(pairs.Folders))
	}

	if len(pairs.Apis) != RootFolderLen*NestedApiLen {
		test.Errorf("Error: %v", len(pairs.Apis))
	}

	// 10 * 100 * (10 + 1) cuz there's 1 default example
	if len(pairs.ApiExamples) != RootFolderLen*NestedApiLen*(PerNestedApiExampleLen+1) {
		test.Errorf("Error: %v", len(pairs.ApiExamples))
	}

	// folder order
	var rootFolder *mitemfolder.ItemFolder
	foundFolder := false
	for _, folder := range pairs.Folders {
		if folder.Prev == nil {
			if foundFolder {
				test.Errorf("Error: %v", "found more than one root folder")
			}

			rootFolder = &folder
			foundFolder = true
		}
	}

	if !foundFolder {
		test.Errorf("Error: %v", "root folder not found")
	}
	if rootFolder == nil {
		test.Errorf("Error: %v", "root folder is nil")
	}

	// api order
	TotatExpectedRootApi := RootFolderLen
	for _, api := range pairs.Apis {
		if api.Prev == nil {
			TotatExpectedRootApi--
		}
	}
	if TotatExpectedRootApi != 0 {
		test.Errorf("Error: %v", "api order")
	}

	// api example order
	expectedApiExampleLen := NestedApiLen * RootFolderLen * 2
	for _, apiExample := range pairs.ApiExamples {
		if apiExample.Prev == nil {
			expectedApiExampleLen--
		}
	}
	if expectedApiExampleLen != 0 {
		test.Errorf("Error: %v, %v", "api example order", expectedApiExampleLen)
	}
}

func TestTranslatePostmanHeader(test *testing.T) {
	RootApiLen := 100
	ResponseLen := 10
	ReponseHeaderLen := 100
	QueryParamterLen := 10
	// expected
	ExpectedResponseLen := RootApiLen * (ResponseLen + 1)
	ExpectedHeaderLen := ExpectedResponseLen * ReponseHeaderLen
	ExpectedQueryLen := ExpectedResponseLen * QueryParamterLen

	var headers []mheader.Header
	for i := 0; i < ReponseHeaderLen; i++ {
		headers = append(headers, mheader.Header{
			Key:         "test",
			Value:       "test",
			Disabled:    false,
			Description: "test",
		})
	}
	var query []murl.QueryParamter
	for i := 0; i < QueryParamterLen; i++ {
		query = append(query, murl.QueryParamter{
			Key:         "test",
			Value:       "test",
			Disabled:    false,
			Description: "test",
		})
	}

	URL := murl.URL{
		Version:   "test",
		Raw:       "test",
		Protocol:  "test",
		Host:      []string{"test"},
		Port:      "test",
		Variables: nil,
		Query:     query,
		Hash:      "test",
	}

	request := mrequest.Request{
		Method:      "GET",
		Header:      headers,
		Body:        nil,
		Description: "test",
		URL:         URL,
	}

	response := mresponse.Response{
		Name:            "test",
		OriginalRequest: nil,
		ResponseTime:    0,
		Headers:         headers,
	}

	responses := []mresponse.Response{}
	for i := 0; i < ResponseLen; i++ {
		responses = append(responses, response)
	}

	RootApi := mitem.Items{
		ID:          "test",
		Name:        "test",
		Responses:   responses,
		Request:     &request,
		Description: "test",
		Variables:   nil,
		Items:       nil,
	}

	items := []mitem.Items{}
	for i := 0; i < RootApiLen; i++ {
		items = append(items, RootApi)
	}

	postmanCollection := mpostmancollection.Collection{
		Auth: nil,
		Info: mpostmancollection.Info{
			Name: "test",
		},
		Items:     items,
		Events:    nil,
		Variables: nil,
	}

	collectionID := idwrap.NewNow()
	pairs, err := tpostman.ConvertPostmanCollection(postmanCollection, collectionID)
	if err != nil {
		test.Errorf("Error: %v", err)
	}

	if len(pairs.Apis) != RootApiLen {
		test.Errorf("Error: %v", len(pairs.Apis))
	}

	if len(pairs.ApiExamples) != ExpectedResponseLen {
		test.Errorf("Error: %v", len(pairs.ApiExamples))
	}

	if len(pairs.Headers) != ExpectedHeaderLen {
		test.Errorf("Error: %v", len(pairs.Headers))
	}

	if len(pairs.Queries) != ExpectedQueryLen {
		test.Errorf("Error: %v", len(pairs.Queries))
	}

	apiUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, api := range pairs.Apis {
		if _, ok := apiUlidMap[api.ID]; ok {
			test.Errorf("Error: %v", "api ulid duplicate")
		}
		apiUlidMap[api.ID] = struct{}{}
	}

	apiExampleUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, apiExample := range pairs.ApiExamples {
		if _, ok := apiExampleUlidMap[apiExample.ID]; ok {
			test.Errorf("Error: %v", "api example ulid duplicate")
		}
		apiExampleUlidMap[apiExample.ID] = struct{}{}
	}

	headerUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, header := range pairs.Headers {
		if _, ok := headerUlidMap[header.ID]; ok {
			test.Errorf("Error: %v", "header ulid duplicate")
		}
		headerUlidMap[header.ID] = struct{}{}
	}

	queryUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, query := range pairs.Queries {
		if _, ok := queryUlidMap[query.ID]; ok {
			test.Errorf("Error: %v", "query ulid duplicate")
		}
		queryUlidMap[query.ID] = struct{}{}
	}
}

func TestTranslatePostmanQuery(test *testing.T) {
	RootApiLen := 100
	ResponseLen := 10
	ReponseHeaderLen := 100
	QueryParamterLen := 10
	// expected
	ExpectedResponseLen := RootApiLen * (ResponseLen + 1)
	ExpectedHeaderLen := ExpectedResponseLen * ReponseHeaderLen
	ExpectedQueryLen := ExpectedResponseLen * QueryParamterLen

	var headers []mheader.Header
	for i := 0; i < ReponseHeaderLen; i++ {
		headers = append(headers, mheader.Header{
			Key:         "test",
			Value:       "test",
			Disabled:    false,
			Description: "test",
		})
	}
	urlData, err := url.Parse("http://localhost:8080")
	if err != nil {
		test.Errorf("Error: %v", err)
	}
	for i := 0; i < QueryParamterLen; i++ {
		queryData := urlData.Query()
		queryData.Add("test", "test")
		urlData.RawQuery = queryData.Encode()
	}

	URLWithQuery := urlData.String()

	request := mrequest.Request{
		Method:      "GET",
		Header:      headers,
		Body:        nil,
		Description: "test",
		URL:         URLWithQuery,
	}

	response := mresponse.Response{
		Name:            "test",
		OriginalRequest: nil,
		ResponseTime:    0,
		Headers:         headers,
	}

	responses := []mresponse.Response{}
	for i := 0; i < ResponseLen; i++ {
		responses = append(responses, response)
	}

	RootApi := mitem.Items{
		ID:          "test",
		Name:        "test",
		Responses:   responses,
		Request:     &request,
		Description: "test",
		Variables:   nil,
		Items:       nil,
	}

	items := []mitem.Items{}
	for i := 0; i < RootApiLen; i++ {
		items = append(items, RootApi)
	}

	postmanCollection := mpostmancollection.Collection{
		Auth: nil,
		Info: mpostmancollection.Info{
			Name: "test",
		},
		Items:     items,
		Events:    nil,
		Variables: nil,
	}

	collectionID := idwrap.NewNow()
	pairs, err := tpostman.ConvertPostmanCollection(postmanCollection, collectionID)
	if err != nil {
		test.Errorf("Error: %v", err)
	}

	if len(pairs.Apis) != RootApiLen {
		test.Errorf("Error: %v", len(pairs.Apis))
	}

	if len(pairs.ApiExamples) != ExpectedResponseLen {
		test.Errorf("Error: %v", len(pairs.ApiExamples))
	}

	if len(pairs.Headers) != ExpectedHeaderLen {
		test.Errorf("Error: %v", len(pairs.Headers))
	}

	if len(pairs.Queries) != ExpectedQueryLen {
		test.Errorf("Error: %v", len(pairs.Queries))
	}

	apiUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, api := range pairs.Apis {
		if strings.ContainsRune(api.Url, '?') {
			test.Errorf("Error: %v", "url contains query")
		}
		if _, ok := apiUlidMap[api.ID]; ok {
			test.Errorf("Error: %v", "api ulid duplicate")
		}
		apiUlidMap[api.ID] = struct{}{}
	}

	apiExampleUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, apiExample := range pairs.ApiExamples {
		if _, ok := apiExampleUlidMap[apiExample.ID]; ok {
			test.Errorf("Error: %v", "api example ulid duplicate")
		}
		apiExampleUlidMap[apiExample.ID] = struct{}{}
	}

	headerUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, header := range pairs.Headers {
		if _, ok := headerUlidMap[header.ID]; ok {
			test.Errorf("Error: %v", "header ulid duplicate")
		}
		headerUlidMap[header.ID] = struct{}{}
	}

	queryUlidMap := make(map[idwrap.IDWrap]struct{})
	for _, query := range pairs.Queries {
		if _, ok := queryUlidMap[query.ID]; ok {
			test.Errorf("Error: %v", "query ulid duplicate")
		}
		queryUlidMap[query.ID] = struct{}{}
	}
}

func TestTranslatePostmanBody(test *testing.T) {
	PerNestedApiExampleLen := 3
	PerRootApi := 9
	bodyFormDataLen := 5
	bodyUrlEncodedLen := 5
	ExpectedBodyBytes := []byte("Abc")

	bodyForm := mbody.Body{
		Mode:     mbody.ModeFormData,
		Raw:      "",
		FormData: []mbody.BodyFormData{},
		Disabled: false,
		Options:  mbody.BodyOptions{},
	}

	bodyUrlEncoded := mbody.Body{
		Mode:       mbody.ModeURLEncoded,
		Raw:        "",
		URLEncoded: []mbody.BodyURLEncoded{},
		Disabled:   false,
		Options:    mbody.BodyOptions{},
	}

	bodyRaw := mbody.Body{
		Mode:     mbody.ModeRaw,
		Raw:      string(ExpectedBodyBytes),
		Disabled: false,
		Options:  mbody.BodyOptions{},
	}

	for i := 0; i < bodyFormDataLen; i++ {
		bodyForm.FormData = append(bodyForm.FormData, mbody.BodyFormData{
			Key:         "test",
			Value:       "test",
			Disabled:    false,
			Description: "test",
		})
	}

	for i := 0; i < bodyUrlEncodedLen; i++ {
		bodyUrlEncoded.URLEncoded = append(bodyUrlEncoded.URLEncoded, mbody.BodyURLEncoded{
			Key:         "test",
			Value:       "test",
			Disabled:    false,
			Description: "test",
		})
	}

	requestForm := mrequest.Request{
		Method:      "GET",
		Header:      nil,
		Body:        &bodyForm,
		Description: "test",
		URL:         "http://localhost:8080",
	}

	requestUrlEncoded := mrequest.Request{
		Method:      "GET",
		Header:      nil,
		Body:        &bodyUrlEncoded,
		Description: "test",
		URL:         "http://localhost:8080",
	}

	requestRaw := mrequest.Request{
		Method:      "GET",
		Header:      nil,
		Body:        &bodyRaw,
		Description: "test",
		URL:         "http://localhost:8080",
	}

	response := mresponse.Response{
		Name:            "test",
		OriginalRequest: nil,
		ResponseTime:    0,
	}

	responses := []mresponse.Response{}
	for i := 0; i < PerNestedApiExampleLen; i++ {
		responses = append(responses, response)
	}

	item := mitem.Items{
		ID:          "test",
		Name:        "test",
		Responses:   responses,
		Description: "test",
		Variables:   nil,
		Items:       nil,
	}

	items := []mitem.Items{}

	for i := 0; i < PerRootApi; i++ {
		item.Request = &requestForm
		items = append(items, item)
	}

	for i := 0; i < PerRootApi; i++ {
		item.Request = &requestUrlEncoded
		items = append(items, item)
	}

	for i := 0; i < PerRootApi; i++ {
		item.Request = &requestRaw
		items = append(items, item)
	}

	postmanCollection := mpostmancollection.Collection{
		Auth: nil,
		Info: mpostmancollection.Info{
			Name: "test",
		},
		Items:     items,
		Events:    nil,
		Variables: nil,
	}

	collectionID := idwrap.NewNow()
	pairs, err := tpostman.ConvertPostmanCollection(postmanCollection, collectionID)
	if err != nil {
		test.Errorf("Error: %v", err)
	}

	expectedRootApi := PerRootApi * PerNestedApiExampleLen
	if len(pairs.Apis) != expectedRootApi {
		test.Errorf("Error: %v", len(pairs.Apis))
	}

	if len(pairs.ApiExamples) != expectedRootApi*(PerNestedApiExampleLen+1) {
		test.Errorf("Error: %v", len(pairs.ApiExamples))
	}

	if len(pairs.BodyForm) != PerRootApi*(PerNestedApiExampleLen+1)*bodyFormDataLen {
		test.Errorf("Error: %v", len(pairs.BodyForm))
	}

	if len(pairs.BodyUrlEncoded) != PerRootApi*(PerNestedApiExampleLen+1)*bodyUrlEncodedLen {
		test.Errorf("Error: %v", len(pairs.BodyUrlEncoded))
	}

	// TODO: Fix this test case
	expectedRawLen := expectedRootApi * (PerNestedApiExampleLen + 1) * (bodyUrlEncodedLen + bodyFormDataLen) / 10
	if len(pairs.BodyRaw) != expectedRawLen {
		test.Errorf("Error: %v %v", expectedRawLen, len(pairs.BodyRaw))
	}
}

func TestTranslatePostmanBodyRaw(test *testing.T) {
	ExpectedBodyBytes := []byte("A")
	LongBody := make([]byte, 1024*1024)
	for i := 0; i < 1024*1024; i++ {
		LongBody[i] = ExpectedBodyBytes[0]
	}

	bodyRaw := mbody.Body{
		Mode:     mbody.ModeRaw,
		Raw:      string(ExpectedBodyBytes),
		Disabled: false,
		Options:  mbody.BodyOptions{},
	}

	requestRaw := mrequest.Request{
		Method:      "GET",
		Header:      nil,
		Body:        &bodyRaw,
		Description: "test",
		URL:         "http://localhost:8080",
	}

	response := mresponse.Response{
		Name:            "test",
		OriginalRequest: nil,
		ResponseTime:    0,
	}

	responses := []mresponse.Response{
		response,
	}

	item := mitem.Items{
		ID:          "test",
		Name:        "test",
		Request:     &requestRaw,
		Responses:   responses,
		Description: "test",
		Variables:   nil,
		Items:       nil,
	}

	items := []mitem.Items{
		item,
	}

	postmanCollection := mpostmancollection.Collection{
		Auth: nil,
		Info: mpostmancollection.Info{
			Name: "test",
		},
		Items:     items,
		Events:    nil,
		Variables: nil,
	}

	collectionID := idwrap.NewNow()
	pairs, err := tpostman.ConvertPostmanCollection(postmanCollection, collectionID)
	if err != nil {
		test.Errorf("Error: %v", err)
	}

	if len(pairs.BodyRaw) != 2 {
		test.Errorf("Error: %v", len(pairs.BodyRaw))
	}
	CompressedBody := pairs.BodyRaw[0]
	if len(CompressedBody.Data) > len(LongBody) {
		test.Errorf("Error: %v", len(CompressedBody.Data))
	}
}
