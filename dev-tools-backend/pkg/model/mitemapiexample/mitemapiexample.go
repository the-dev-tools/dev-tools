package mitemapiexample

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/oklog/ulid/v2"
)

type ItemApiExampleMeta struct {
	ID   ulid.ULID
	Name string
}

type ItemApiExample struct {
	ID              ulid.ULID
	ItemApiID       ulid.ULID
	ParentExampleID *ulid.ULID
	CollectionID    ulid.ULID
	IsDefault       bool
	Name            string
	Headers         Headers
	Cookies         Cookies
	Compressed      bool
	Body            []byte
	Query           Query
	Updated         time.Time
}

func NewItemApiExample(id ulid.ULID, itemApiID, collectionID ulid.ULID, parentExampleId *ulid.ULID, isDefault bool, name string, headers Headers, query Query, compressed bool, body []byte) *ItemApiExample {
	return &ItemApiExample{
		ID:              id,
		ItemApiID:       itemApiID,
		ParentExampleID: parentExampleId,
		CollectionID:    collectionID,
		IsDefault:       isDefault,
		Name:            name,
		Headers:         headers,
		Query:           query,
		Compressed:      compressed,
		Body:            body,
		Updated:         time.Now(),
	}
}

func (i ItemApiExample) GetCreatedTime() time.Time {
	return time.UnixMilli(int64(i.ID.Time()))
}

type Cookies struct {
	CookieMap map[string]string
}

func NewCookies(cookies map[string]string) *Cookies {
	return &Cookies{
		CookieMap: cookies,
	}
}

func NewCookiesDefault() *Cookies {
	return &Cookies{
		CookieMap: make(map[string]string),
	}
}

func (c Cookies) Value() (driver.Value, error) {
	return json.Marshal(c.CookieMap)
}

func (c Cookies) Bytes() []byte {
	b, _ := json.Marshal(c.CookieMap)
	return b
}

func (c *Cookies) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), &c.CookieMap)
}

func (i *ItemApiExample) SetCookies(cookies map[string]string) {
	i.Cookies.CookieMap = cookies
}

func (i *ItemApiExample) GetCookies() map[string]string {
	return i.Cookies.CookieMap
}

type Headers struct {
	HeaderMap map[string]string
}

func NewHeaders(headers map[string]string) *Headers {
	return &Headers{
		HeaderMap: headers,
	}
}

func NewHeadersDefault() *Headers {
	return &Headers{
		HeaderMap: make(map[string]string),
	}
}

type Query struct {
	QueryMap map[string]string
}

func NewQuery(query map[string]string) *Query {
	return &Query{
		QueryMap: query,
	}
}

func NewQueryDefault() *Query {
	return &Query{
		QueryMap: make(map[string]string),
	}
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

func (i *ItemApiExample) SetHeaders(headers map[string]string) {
	i.Headers.HeaderMap = headers
}

func (i *ItemApiExample) GetHeaders() map[string]string {
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

func (i *ItemApiExample) SetQueryParams(queryParams map[string]string) {
	i.Query.QueryMap = queryParams
}

func (i *ItemApiExample) GetQueryParams() map[string]string {
	return i.Query.QueryMap
}
