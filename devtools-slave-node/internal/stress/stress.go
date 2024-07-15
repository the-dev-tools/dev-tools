package stress

import (
	"bytes"
	"devtools-tasks/pkg/model/mrequest"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

func HttpGet(req mrequest.Request) (*mrequest.RequestResult, error) {
	url, err := url.Parse(req.URI)
	if err != nil {
		return nil, err
	}

	query := url.Query()
	for k, v := range req.QueryParams {
		query.Add(k, v)
	}
	url.RawQuery = query.Encode()

	reader := bytes.NewReader(req.Data)
	rawReq, err := http.NewRequest("GET", url.String(), reader)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		rawReq.Header.Add(k, v)
	}

	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	resault := mrequest.RequestResult{
		ID:        uuid.String(),
		RequestID: req.RequestID,
	}

	start := time.Now()

	// TODO: change this to custom client
	resp, err := http.DefaultClient.Do(rawReq)
	if err != nil {
		return nil, err
	}

	resault.Duration = time.Since(start)
	resault.StatusCode = resp.StatusCode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resault.Body = body

	return &resault, nil
}

func HttpPost(req mrequest.Request) (*mrequest.RequestResult, error) {
	url, err := url.Parse(req.URI)
	if err != nil {
		return nil, err
	}

	query := url.Query()
	for k, v := range req.QueryParams {
		query.Add(k, v)
	}
	url.RawQuery = query.Encode()

	reader := bytes.NewReader(req.Data)
	rawReq, err := http.NewRequest("POST", url.String(), reader)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		rawReq.Header.Add(k, v)
	}

	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	resault := mrequest.RequestResult{
		ID:        uuid.String(),
		RequestID: req.RequestID,
	}

	start := time.Now()

	// TODO: change this to custom client
	resp, err := http.DefaultClient.Do(rawReq)
	if err != nil {
		return nil, err
	}

	resault.Duration = time.Since(start)
	resault.StatusCode = resp.StatusCode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resault.Body = body

	return &resault, nil
}

func HttpPut(req mrequest.Request) (*mrequest.RequestResult, error) {
	url, err := url.Parse(req.URI)
	if err != nil {
		return nil, err
	}

	query := url.Query()
	for k, v := range req.QueryParams {
		query.Add(k, v)
	}
	url.RawQuery = query.Encode()

	reader := bytes.NewReader(req.Data)
	rawReq, err := http.NewRequest("PUT", url.String(), reader)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		rawReq.Header.Add(k, v)
	}

	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	resault := mrequest.RequestResult{
		ID:        uuid.String(),
		RequestID: req.RequestID,
	}

	start := time.Now()

	// TODO: change this to custom client
	resp, err := http.DefaultClient.Do(rawReq)
	if err != nil {
		return nil, err
	}

	resault.Duration = time.Since(start)
	resault.StatusCode = resp.StatusCode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resault.Body = body

	return &resault, nil
}

func HttpDelete(req mrequest.Request) (*mrequest.RequestResult, error) {
	url, err := url.Parse(req.URI)
	if err != nil {
		return nil, err
	}

	query := url.Query()
	for k, v := range req.QueryParams {
		query.Add(k, v)
	}
	url.RawQuery = query.Encode()

	reader := bytes.NewReader(req.Data)
	rawReq, err := http.NewRequest("PUT", url.String(), reader)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		rawReq.Header.Add(k, v)
	}

	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	resault := mrequest.RequestResult{
		ID:        uuid.String(),
		RequestID: req.RequestID,
	}

	start := time.Now()

	// TODO: change this to custom client
	resp, err := http.DefaultClient.Do(rawReq)
	if err != nil {
		return nil, err
	}

	resault.Duration = time.Since(start)
	resault.StatusCode = resp.StatusCode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resault.Body = body

	return &resault, nil
}
