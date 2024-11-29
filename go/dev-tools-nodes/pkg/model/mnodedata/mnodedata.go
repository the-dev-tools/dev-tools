package mnodedata

import (
	"database/sql/driver"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/model/mexamplequery"
	"encoding/json"
	"errors"
)

//
// Loop data struct
//

type NodeLoopData struct {
	Count         int
	LoopStartNode string
}

type NodeLoopRemoteData struct {
	Count             uint64
	LoopStartNode     string
	MachinesAmount    uint64
	SlaveHttpEndpoint string
}

//
// API data struct
//

type NodeApiRestData struct {
	Url     string                  `json:"url"`
	Query   []mexamplequery.Query   `json:"queryParams"`
	Method  string                  `json:"method"`
	Headers []mexampleheader.Header `json:"headers"`
	Body    []byte                  `json:"body"`
}

// Driver value
func (Social NodeApiRestData) Value() (driver.Value, error) {
	byteArr, err := json.Marshal(Social)
	if err != nil {
		return nil, err
	}
	return byteArr, nil
}

func (Socal *NodeApiRestData) Scan(value interface{}) error {
	byteArr, ok := value.([]byte)
	if !ok {
		return errors.New("nodeapirestdata is not []byte")
	}
	return json.Unmarshal(byteArr, Socal)
}

//
// Condition data struct
//

type NodeConditionRestStatusData struct {
	StatusCodeExits map[string]string
}

type NodeConditionJsonMatchData struct {
	Data       []byte
	Path       string
	MatchExits map[string]string
}

type NodeConditionExpressionData struct {
	Expression string
	MatchExits map[string]string
}

//
// Email data struct
//

type NodeEmailData struct {
	To string `json:"to"`
}
