package httpmockclient

import "net/http"

type MockHttpClient struct {
	ReturnResponse *http.Response
}

func NewMockHttpClient(returnResponse *http.Response) *MockHttpClient {
	return &MockHttpClient{
		ReturnResponse: returnResponse,
	}
}

func (m *MockHttpClient) Do(req *http.Request) (*http.Response, error) {
	return m.ReturnResponse, nil
}
