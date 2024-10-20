package assertsys_test

import (
	"bytes"
	"dev-tools-backend/pkg/assertsys"
	"dev-tools-backend/pkg/model/massert"
	"io"
	"net/http"
	"testing"
)

func TestAssertSys_Eval_EqualTrue(t *testing.T) {
	statuscode := 200

	stringReader := bytes.NewBufferString("shiny!")
	stringReadCloser := io.NopCloser(stringReader)

	respHttp := http.Response{
		StatusCode: statuscode,
		Body:       stringReadCloser,
	}

	assertSys := assertsys.New()

	ok, err := assertSys.Eval(respHttp, massert.AssertTypeEqual, "response.status", "200")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}

func TestAssertSys_Eval_EqualFalse(t *testing.T) {
	statuscode := 400

	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	stringReader := bytes.NewBuffer(jsonBytes)
	stringReadCloser := io.NopCloser(stringReader)

	respHttp := http.Response{
		StatusCode: statuscode,
		Body:       stringReadCloser,
	}

	assertSys := assertsys.New()

	ok, err := assertSys.Eval(respHttp, massert.AssertTypeEqual, "response.status", "200")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if ok {
		t.Errorf("Expected false, got %v", ok)
	}
}

func TestAssertSys_Eval_ContainsTrue(t *testing.T) {
	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	stringReader := bytes.NewBuffer(jsonBytes)
	stringReadCloser := io.NopCloser(stringReader)
	respHttp := http.Response{
		Body: stringReadCloser,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, "response.body", "John")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}

func TestAssertSys_Eval_ContainsFalse(t *testing.T) {
	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	stringReader := bytes.NewBuffer(jsonBytes)
	stringReadCloser := io.NopCloser(stringReader)
	respHttp := http.Response{
		Body: stringReadCloser,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, "response.body", "Doe")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if ok {
		t.Errorf("Expected false, got %v", ok)
	}
}

func TestAssertSys_Eval_HeadersContainsTrue(t *testing.T) {
	header := map[string][]string{
		"Content-Type": {"application/json"},
	}
	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	stringReader := bytes.NewBuffer(jsonBytes)
	stringReadCloser := io.NopCloser(stringReader)
	respHttp := http.Response{
		Header: header,
		Body:   stringReadCloser,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, "response.header", "Content-Type")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}

func TestAssertSys_Eval_HeadersContainsFalse(t *testing.T) {
	header := map[string][]string{
		"Content-Type": {"application/json"},
	}

	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	stringReader := bytes.NewBuffer(jsonBytes)
	stringReadCloser := io.NopCloser(stringReader)
	respHttp := http.Response{
		Header: header,
		Body:   stringReadCloser,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeContains, "response.header", "Content-Length")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if ok {
		t.Errorf("Expected false, got %v", ok)
	}
}

func TestAssertSys_Eval_HeadersEqualsTrue(t *testing.T) {
	header := map[string][]string{
		"Content-Type": {"application/json"},
	}
	jsonBytes := []byte(`{"name":"John","age":30,"city":"New York"}`)
	stringReader := bytes.NewBuffer(jsonBytes)
	stringReadCloser := io.NopCloser(stringReader)
	respHttp := http.Response{
		Header: header,
		Body:   stringReadCloser,
	}
	assertSys := assertsys.New()
	ok, err := assertSys.Eval(respHttp, massert.AssertTypeEqual, "response.header.Content-Type", "application/json")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !ok {
		t.Errorf("Expected true, got %v", ok)
	}
}
