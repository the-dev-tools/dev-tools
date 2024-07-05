package mrequest

import "time"

type MethodTypeID int

const (
	// HTTP methods
	HttpMethodGet MethodTypeID = iota
	HttpMethodPost
	HttpMethodPut
	HttpMethodDelete
	HttpMethodPatch
)

type Request struct {
	URI          string
	RequestID    string
	QueryParams  map[string]string
	Headers      map[string]string
	Data         []byte
	MethodTypeID MethodTypeID
}

type RequestResult struct {
	ID         string
	RequestID  string
	Duration   time.Duration
	StatusCode int
	Body       []byte
}
