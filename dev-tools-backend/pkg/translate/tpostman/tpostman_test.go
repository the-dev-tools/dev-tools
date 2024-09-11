package tpostman_test

import (
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/model/postman/v21/mheader"
	"dev-tools-backend/pkg/model/postman/v21/mitem"
	"dev-tools-backend/pkg/model/postman/v21/mpostmancollection"
	"dev-tools-backend/pkg/model/postman/v21/mrequest"
	"dev-tools-backend/pkg/model/postman/v21/mresponse"
	"dev-tools-backend/pkg/model/postman/v21/murl"
	"dev-tools-backend/pkg/translate/tpostman"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func TestTranslatePostman(test *testing.T) {
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

	collectionID := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.DefaultEntropy())
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
}

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

	collectionID := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.DefaultEntropy())
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
	var foundFolder bool = false
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
	var TotatExpectedRootApi int = RootFolderLen
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

	collectionID := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.DefaultEntropy())
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

	apiUlidMap := make(map[ulid.ULID]struct{})
	for _, api := range pairs.Apis {
		if _, ok := apiUlidMap[api.ID]; ok {
			test.Errorf("Error: %v", "api ulid duplicate")
		}
		apiUlidMap[api.ID] = struct{}{}
	}

	apiExampleUlidMap := make(map[ulid.ULID]struct{})
	for _, apiExample := range pairs.ApiExamples {
		if _, ok := apiExampleUlidMap[apiExample.ID]; ok {
			test.Errorf("Error: %v", "api example ulid duplicate")
		}
		apiExampleUlidMap[apiExample.ID] = struct{}{}
	}

	headerUlidMap := make(map[ulid.ULID]struct{})
	for _, header := range pairs.Headers {
		if _, ok := headerUlidMap[header.ID]; ok {
			test.Errorf("Error: %v", "header ulid duplicate")
		}
		headerUlidMap[header.ID] = struct{}{}
	}

	queryUlidMap := make(map[ulid.ULID]struct{})
	for _, query := range pairs.Queries {
		if _, ok := queryUlidMap[query.ID]; ok {
			test.Errorf("Error: %v", "query ulid duplicate")
		}
		queryUlidMap[query.ID] = struct{}{}
	}
}
