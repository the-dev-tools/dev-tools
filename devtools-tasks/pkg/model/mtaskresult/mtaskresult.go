package mtaskresult

import "time"

const (
	TaskStatusSuccess = "Success"
	TaskStatusFailed  = "Failed"
)

type TaskResult struct {
	ID         string
	RequestIDs []string
	Name       string

	Status       string
	SuccessCount int
	FailedCount  int

	StatusCodeMap  map[int]int
	RequestBodyMap map[string][]byte

	SentCount  int
	StartedAt  time.Time
	FinishedAt time.Time
	TimeTaken  time.Duration
}
