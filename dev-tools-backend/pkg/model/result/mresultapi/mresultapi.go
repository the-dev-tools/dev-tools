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
	ID          idwrap.IDWrap `json:"id"`
	TriggerType TriggerType   `json:"triggerdBy"`
	TriggerBy   idwrap.IDWrap `json:"rootID"`
	Name        string        `json:"name"`
	Time        time.Time     `json:"time"`
	Duration    time.Duration `json:"duration"`
	HttpResp    HttpResp      `json:"httpResp"`
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
	data, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(data, &h)
}
