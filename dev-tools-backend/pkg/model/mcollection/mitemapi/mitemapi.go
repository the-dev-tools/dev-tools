package mitemapi

import (
	"database/sql/driver"

	"github.com/goccy/go-json"

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
	Query        Query
}

type Headers struct {
	HeaderMap map[string]string
}

type Query struct {
	QueryMap map[string]string
}

// Headers
func (h Headers) Value() (driver.Value, error) {
	return json.Marshal(h.HeaderMap)
}

func (h Headers) Bytes() []byte {
	b, _ := json.Marshal(h.HeaderMap)
	return b
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
func (q Query) Value() (driver.Value, error) {
	return json.Marshal(q.QueryMap)
}

func (q Query) Bytes() []byte {
	b, _ := json.Marshal(q.QueryMap)
	return b
}

func (q *Query) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), &q.QueryMap)
}

func (i *ItemApi) SetQueryParams(queryParams map[string]string) {
	i.Query.QueryMap = queryParams
}

func (i *ItemApi) GetQueryParams() map[string]string {
	return i.Query.QueryMap
}
