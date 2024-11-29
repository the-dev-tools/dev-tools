package mresultapi

import (
	"database/sql/driver"
	"dev-tools-backend/pkg/idwrap"
	"errors"
	"net/http"
	"time"

	"github.com/goccy/go-json"
)

type TriggerType int16

var (
	TRIGGER_TYPE_UNSPECIFIED TriggerType = 0
	TRIGGER_TYPE_COLLECTION  TriggerType = 1
	TRIGGER_TYPE_FLOW        TriggerType = 2
	TRIGGER_TYPE_CRON        TriggerType = 3
	TRIGGER_TYPE_WEBHOOK     TriggerType = 4
)

type MResultAPI struct {
	Time        time.Time     `json:"time"`
	Name        string        `json:"name"`
	HttpResp    HttpResp      `json:"httpResp"`
	Duration    time.Duration `json:"duration"`
	TriggerType TriggerType   `json:"triggerdBy"`
	ID          idwrap.IDWrap `json:"id"`
	TriggerBy   idwrap.IDWrap `json:"rootID"`
}

type HttpResp struct {
	Header     http.Header `json:"header"`
	Proto      string      `json:"proto"`
	Body       []byte      `json:"body"`
	StatusCode int         `json:"statusCode"`
	ProtoMajor int         `json:"protoMajor"`
	ProtoMinor int         `json:"protoMinor"`
}

func (h HttpResp) Value() (driver.Value, error) {
	return json.Marshal(h)
}

func (h *HttpResp) Scan(value interface{}) error {
	data, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(data, &h)
}
