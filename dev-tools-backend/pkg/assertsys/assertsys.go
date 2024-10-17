package assertsys

import (
	"dev-tools-backend/pkg/model/massert"
	"net/http"
)

func AssertTarget(assertType massert.AssertType, resp http.Response, assertValue, assertKey string) bool {
	switch assertType {
	case massert.AssertTypeEqual:
		return target == massert.AssertTargetBody
	case massert.AssertTypeNotEqual:
		return target == massert.AssertTargetBody
	case massert.AssertTypeContains:
		return target == massert.AssertTargetBody
	case massert.AssertTypeNotContains:
		return target == massert.AssertTargetBody
	case massert.AssertTypeGreater:
		return target == massert.AssertTargetBody
	case massert.AssertTypeLess:
		return target == massert.AssertTargetBody
	case massert.AssertTypeGreaterOrEqual:
		return target == massert.AssertTargetBody
	case massert.AssertTypeLessOrEqual:
		return target == massert.AssertTargetBody
	default:
		return false
	}
}

func RespToMap(resp http.Response) map[string]interface{} {
	return map[string]interface{}{
		"status": resp.Status,
		"header": resp.Header,
		"body":   resp.Body,
	}
}
