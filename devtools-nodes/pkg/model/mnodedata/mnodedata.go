package mnodedata

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
	Url         string            `json:"url"`
	QueryParams map[string]string `json:"queryParams"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
}

// Driver value
func (d NodeApiRestData) Driver() string {
	return "rest"
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
