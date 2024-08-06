package mitemapi

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/oklog/ulid/v2"
)

type ItemApi struct {
	ID           ulid.ULID
	CollectionID ulid.ULID
	ParentID     *ulid.ULID
	Name         string
	Url          string
	Method       string
	Headers      Headers
	Body         []byte
	QueryParams  QueryParams
}

type Headers struct {
	HeaderMap map[string]string
}

type QueryParams struct {
	QueryMap map[string]string
}

// Headers
func (h Headers) Value() (driver.Value, error) {
	return json.Marshal(h.HeaderMap)
}

func (h *Headers) Scan(value interface{}) error {
	// TODO: check if value is not byte array
	return json.Unmarshal(value.([]byte), &h.HeaderMap)
}

func (i *ItemApi) SetHeaders(headers map[string]string) {
	i.Headers.HeaderMap = headers
}

func (i *ItemApi) GetHeaders() map[string]string {
	return i.Headers.HeaderMap
}

// QueryParams
func (q QueryParams) Value() (driver.Value, error) {
	return json.Marshal(q.QueryMap)
}

func (q *QueryParams) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), &q.QueryMap)
}

func (i *ItemApi) SetQueryParams(queryParams map[string]string) {
	i.QueryParams.QueryMap = queryParams
}

func (i *ItemApi) GetQueryParams() map[string]string {
	return i.QueryParams.QueryMap
}
