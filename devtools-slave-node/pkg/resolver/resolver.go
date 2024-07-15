package resolver

import (
	"devtools-slave-node/internal/stress"
	"devtools-tasks/pkg/model/mrequest"
)

type StressMethodFunc func(req mrequest.Request) (*mrequest.RequestResult, error)

// resolve the method type to the corresponding function
// to sending stress request
func Resolve(methodType mrequest.MethodTypeID) StressMethodFunc {
	switch methodType {
	case mrequest.HttpMethodGet:
		return stress.HttpGet
	case mrequest.HttpMethodPost:
		return stress.HttpPost
	case mrequest.HttpMethodPut:
		return stress.HttpPut
	case mrequest.HttpMethodDelete:
		return stress.HttpDelete
	default:
		return stress.HttpGet
	}
}
