package httpmockclient

import "net/http"

type MockHttpClient struct {
	ReturnResponse *http.Response
}

func (m *MockHttpClient) Do(req *http.Request) (*http.Response, error) {
	return m.ReturnResponse, nil
}
