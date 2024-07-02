package mtaskresult

import "time"

type TaskResult struct {
	ID       int
	MethodID int
	Name     string

	PassedTest   bool
	SuccessCount int
	FailedCount  int

	requestMap map[string]string

	SentCount  int
	StartedAt  time.Time
	FinishedAt time.Time
}
