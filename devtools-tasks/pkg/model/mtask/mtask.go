package mtask

import (
	"devtools-tasks/pkg/model/mrequest"
	"devtools-tasks/pkg/model/mtaskresult"
	"time"
)

type Task struct {
	ID     string
	Name   string
	Amount int
	Status string

	Waits []int // task IDS

	RequestData mrequest.Request

	TestMethod string
	Done       chan bool
	Next       []int // task IDs

	PassSuccessRate float32

	Result  mtaskresult.TaskResult // can be struct with more details
	Timeout time.Duration
}

type TaskRequest struct {
	ID              string
	Amount          int
	RequestData     mrequest.Request
	PassSuccessRate float32
	Timeout         time.Duration
}

type TaskResponse struct {
	ID     string
	Amount int

	Resault     mtaskresult.TaskResult
	ReqResaults []mrequest.RequestResult
}
