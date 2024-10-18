package assertsys

import (
	"dev-tools-backend/pkg/model/massert"
	"errors"
	"strings"
)

type AssertSys struct{}

func New() AssertSys {
	return AssertSys{}
}

var (
	validFirstKeys     = map[string]massert.AssertTarget{"header": massert.AssertTargetHeader, "body": massert.AssertTargetBody}
	ErrInvalidFirstKey = errors.New("invalid first key")
)

type Resource struct {
	AssertType      int8
	AssertTargetKey int8
}

func GetResource(jsondothpath string) (Resource, error) {
	keys := strings.Split(jsondothpath, ".")
	if len(keys) < 2 {
		return Resource{}, ErrInvalidFirstKey
	}
	_, ok := validFirstKeys[keys[0]]
	if !ok {
		return Resource{}, ErrInvalidFirstKey
	}
}
