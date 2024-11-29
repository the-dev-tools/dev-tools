package assertsys_test

import (
	"fmt"
	"testing"
	"the-dev-tools/backend/pkg/assertsys"
	"the-dev-tools/backend/pkg/model/massert"
	"the-dev-tools/backend/pkg/model/mexamplerespheader"
	"the-dev-tools/nodes/pkg/httpclient"
)

var (
	HeaderPath = fmt.Sprintf("%s", assertsys.HeaderKey)
	BodyPath   = fmt.Sprintf("%s", assertsys.BodyKey)
	StatusPath = fmt.Sprintf("%s", assertsys.StatusKey)
)

func TestAssertSys_Eval_EqualTrue(t *testing.T) {
	statuscode := 200
	respHttp := httpclient.Response{
		StatusCode: statuscode,
	}
	assertSys := assertsys.New()

	ok, err := assertSys.Eval(respHttp, massert.AssertTypeEqual, StatusPath, "200")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}

func TestAssertSys_Eval_EqualFalse(t *testing.T) {
	statuscode := 400
	respHttp := httpclient.Response{
		StatusCode: statuscode,
	}

	assertSys := assertsys.New()

	ok, err := assertSys.Eval(respHttp, massert.AssertTypeEqual, StatusPath, "200")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if ok {
		t.Errorf("Expected false, got %v", ok)
	}
}

func TestAssertSys_Eval_ContainsTrue(t *testing.T) {
	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	respHttp := httpclient.Response{
		Body: jsonBytes,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, BodyPath, "John")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}

func TestAssertSys_Eval_ContainsFalse(t *testing.T) {
	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	respHttp := httpclient.Response{
		Body: jsonBytes,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, BodyPath, "Doe")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if ok {
		t.Errorf("Expected false, got %v", ok)
	}
}

func TestAssertSys_Eval_HeadersContainsTrue(t *testing.T) {
	headers := []mexamplerespheader.ExampleRespHeader{
		{HeaderKey: "Content-Type", Value: "application/json"},
	}
	respHttp := httpclient.Response{
		Headers: headers,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, HeaderPath, "Content-Type")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}

func TestAssertSys_Eval_HeadersContainsFalse(t *testing.T) {
	headers := []mexamplerespheader.ExampleRespHeader{
		{HeaderKey: "Content-Type", Value: "application/json"},
	}
	respHttp := httpclient.Response{
		Headers: headers,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, HeaderPath, "Content-Length")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if ok {
		t.Errorf("Expected false, got %v", ok)
	}
}

func TestAssertSys_Eval_HeadersEqualsTrue(t *testing.T) {
	headers := []mexamplerespheader.ExampleRespHeader{
		{HeaderKey: "Content-Type", Value: "application/json"},
	}
	respHttp := httpclient.Response{
		Headers: headers,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeEqual, HeaderPath+".Content-Type", "application/json")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}

func TestAssertSys_Eval_HeadersAnyEqualsTrue(t *testing.T) {
	header1Key := "Content-Type"
	header1Value := "application/json"

	headers := []mexamplerespheader.ExampleRespHeader{
		{HeaderKey: header1Key, Value: header1Value},
		{HeaderKey: "HeaderKey2", Value: "something2"},
		{HeaderKey: "HeaderKey3", Value: "something3"},
	}
	respHttp := httpclient.Response{
		Headers: headers,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, HeaderPath+".any", header1Value)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}
