package tpostman_test

import (
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/model/postman/v21/mitem"
	"dev-tools-backend/pkg/model/postman/v21/mpostmancollection"
	"dev-tools-backend/pkg/model/postman/v21/mrequest"
	"dev-tools-backend/pkg/model/postman/v21/mresponse"
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

	if len(pairs.Folder) != RootFolderLen {
		test.Errorf("Error: %v", len(pairs.Folder))
	}

	if len(pairs.Api) != RootFolderLen*NestedApiLen {
		test.Errorf("Error: %v", len(pairs.Api))
	}

	// 10 * 100 * (10 + 1) cuz there's 1 default example
	if len(pairs.ApiExample) != RootFolderLen*NestedApiLen*(PerNestedApiExampleLen+1) {
		test.Errorf("Error: %v", len(pairs.ApiExample))
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

	if len(pairs.Folder) != RootFolderLen {
		test.Errorf("Error: %v", len(pairs.Folder))
	}

	if len(pairs.Api) != RootFolderLen*NestedApiLen {
		test.Errorf("Error: %v", len(pairs.Api))
	}

	// 10 * 100 * (10 + 1) cuz there's 1 default example
	if len(pairs.ApiExample) != RootFolderLen*NestedApiLen*(PerNestedApiExampleLen+1) {
		test.Errorf("Error: %v", len(pairs.ApiExample))
	}

	// folder order
	var rootFolder *mitemfolder.ItemFolder
	var foundFolder bool = false
	for _, folder := range pairs.Folder {
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
	for _, api := range pairs.Api {
		if api.Prev == nil {
			TotatExpectedRootApi--
		}
	}
	if TotatExpectedRootApi != 0 {
		test.Errorf("Error: %v", "api order")
	}

	// api example order
	expectedApiExampleLen := NestedApiLen * RootFolderLen * 2
	for _, apiExample := range pairs.ApiExample {
		if apiExample.Prev == nil {
			expectedApiExampleLen--
		}
	}
	if expectedApiExampleLen != 0 {
		test.Errorf("Error: %v, %v", "api example order", expectedApiExampleLen)
	}
}
