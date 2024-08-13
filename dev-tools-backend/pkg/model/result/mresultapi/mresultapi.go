package mresultapi

import (
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

type TriggerType int32

var (
	TriggerTypeUnknown TriggerType = 0
	TriggerTypeManuel  TriggerType = 1
	TriggerTypeFlow    TriggerType = 2
	TriggerTypeCron    TriggerType = 3
	TriggerTypeWebhook TriggerType = 4
)

type MResultAPI struct {
	ID        ulid.ULID     `json:"id"`
	ReqID     ulid.ULID     `json:"rootID"`
	TriggerBy TriggerType   `json:"triggerdBy"`
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Time      time.Time     `json:"time"`
	Duration  time.Duration `json:"duration"`
	HttpResp  HttpResp      `json:"httpResp"`
}

type HttpResp struct {
	StatusCode int         `json:"statusCode"`
	Proto      string      `json:"proto"`
	ProtoMajor int         `json:"protoMajor"`
	ProtoMinor int         `json:"protoMinor"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"body"`
}

func (h HttpResp) Value() (driver.Value, error) {
	return json.Marshal(h)
}

func (h *HttpResp) Scan(value interface{}) error {
	// TODO: check if value is not byte array
	return json.Unmarshal(value.([]byte), &h)
}
