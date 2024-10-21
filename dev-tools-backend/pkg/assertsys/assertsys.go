package assertsys

import (
	"context"
	"dev-tools-backend/pkg/model/massert"
	"dev-tools-nodes/pkg/httpclient"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/PaesslerAG/gval"
)

type AssertSys struct {
	assertMap map[massert.AssertType]string
}

func New() AssertSys {
	mapAssertType := massert.MapAssertType()
	return AssertSys{
		assertMap: mapAssertType,
	}
}

func (c AssertSys) Eval(respHttp httpclient.Response, at massert.AssertType, jsondothpath, val string) (bool, error) {
	bodyMap := make(map[string]interface{})
	// turn response body into map
	err := json.Unmarshal(respHttp.Body, &bodyMap)

	headerMap := make(map[string]interface{})
	// turn response header into map
	for _, v := range respHttp.Headers {
		val, ok := headerMap[v.HeaderKey]
		if ok {
			headerMap[v.HeaderKey] = []string{val.(string), v.Value}
		} else {
			headerMap[v.HeaderKey] = v.Value
		}
	}

	respMap := make(map[string]interface{})
	respMap["body"] = bodyMap
	respMap["header"] = headerMap
	respMap["status"] = respHttp.StatusCode

	rootMap := make(map[string]interface{})
	rootMap["response"] = respMap

	gvalFunc := gval.Function("contains", func(args ...interface{}) (bool, error) {
		if len(args) != 2 {
			return false, fmt.Errorf("contains function requires 2 arguments")
		}
		a, b := args[0], args[1]
		if a == nil || b == nil {
			return false, nil
		}
		aMap, ok := a.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("a invalid type %T", a)
		}
		jsonBytes, err := json.Marshal(aMap)
		if err != nil {
			return false, err
		}
		aStr := string(jsonBytes)

		bStr, ok := b.(string)
		if !ok {
			return false, fmt.Errorf("b invalid type %T", b)
		}

		return strings.Contains(aStr, bStr), nil
	})

	// INFO: need for dash in json path
	dashOption := gval.NewLanguage(
		gval.Init(func(ctx context.Context, p *gval.Parser) (gval.Evaluable, error) {
			p.SetIsIdentRuneFunc(func(r rune, pos int) bool {
				return unicode.IsLetter(r) || r == '_' || (pos > 0 && unicode.IsDigit(r)) || (pos > 0 && r == '-')
			})
			return p.ParseExpression(ctx)
		}),
	)

	options := []gval.Language{gvalFunc, dashOption}

	var evalOuputVal interface{}
	a, ok := c.assertMap[at]
	if at == massert.AssertTypeContains || at == massert.AssertTypeNotContains {
		if !ok {
			return false, fmt.Errorf("invalid assert type %d", at)
		}
		// TODO: fix this string manipulation
		evalQuery := fmt.Sprintf("%s(%s, \"%s\")", a, jsondothpath, val)
		evalOuputVal, err = gval.Evaluate(evalQuery, rootMap, options...)
		if err != nil {
			return false, err
		}

	} else {
		// TODO: fix this string manipulation
		evalQuery := fmt.Sprintf("%s %s \"%s\"", jsondothpath, a, val)
		evalOuputVal, err = gval.Evaluate(evalQuery, rootMap, options...)
	}
	if err != nil {
		fmt.Println(err)
	}

	valueBool, ok := evalOuputVal.(bool)
	if !ok {
		return false, fmt.Errorf("invalid type %T", evalOuputVal)
	}

	return valueBool, nil
}
