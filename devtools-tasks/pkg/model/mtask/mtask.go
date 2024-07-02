package mtask

import (
	"time"

	"github.com/DevToolsGit/devtools-tasks/pkg/model/mtaskresult"
)

type Task struct {
	ID     string
	Amount int
	Status string

	Waits []int // task IDS

	HttpMethod string
	Done       chan bool
	Next       []int // task IDs

	PassSuccessRate float32

	Result  mtaskresult.TaskResult // can be struct with more details
	Timeout time.Duration
}

type TaskSender struct {
	ID         string
	Amount     int
	HttpMethod string
	Timeout    time.Duration
}
