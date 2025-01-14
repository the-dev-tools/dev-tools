package tassert

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massert"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) (*requestv1.Assert, error) {
	var pathKeys []*referencev1.ReferenceKey
	str := strings.Split(a.Path, ".")
	arrayRegex := regexp.MustCompile(`\[(\d+)\]`)
	for _, s := range str {
		pathKey := referencev1.ReferenceKey{
			Key: s,
		}
		arr := arrayRegex.MatchString(s)
		if arr {
			pathKey.Kind = referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX
			path := arrayRegex.FindStringSubmatch(s)[1]
			pathInt, err := strconv.Atoi(path)
			if err != nil {
				return nil, err
			}
			pathKey.Index = int32(pathInt)
		}
		if s != "any" {
			pathKey.Kind = referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY
		}
		pathKeys = append(pathKeys, &pathKey)
	}

	return &requestv1.Assert{
		AssertId:       a.ID.Bytes(),
		ParentAssertId: nil,
		Condition: &conditionv1.Condition{
			Comparison: &conditionv1.Comparison{
				Kind:  conditionv1.ComparisonKind(a.Type),
				Path:  pathKeys,
				Value: a.Value,
			},
		},
	}, nil
}

func SerializeAssertModelToRPCItem(a massert.Assert) (*requestv1.AssertListItem, error) {
	assertRpc, err := SerializeAssertModelToRPC(a)
	if err != nil {
		return nil, err
	}

	return &requestv1.AssertListItem{
		AssertId:       assertRpc.AssertId,
		ParentAssertId: assertRpc.ParentAssertId,
		Condition:      assertRpc.Condition,
	}, nil
}

func SerializeAssertRPCToModel(assert *requestv1.Assert, exampleID idwrap.IDWrap) (massert.Assert, error) {
	id, err := idwrap.NewFromBytes(assert.GetAssertId())
	if err != nil {
		return massert.Assert{}, err
	}
	a := SerializeAssertRPCToModelWithoutID(assert, exampleID)
	a.ID = id
	return a, nil
}

func SerializeAssertRPCToModelWithoutID(a *requestv1.Assert, exampleID idwrap.IDWrap) massert.Assert {
	var path, value string
	massertType := massert.AssertTypeUndefined
	if a.Condition != nil {
		if a.Condition.Comparison != nil {
			comp := a.Condition.Comparison

			for _, p := range comp.Path {
				switch p.Kind {
				case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY:
					path += "." + p.Key
				case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX:
					path += fmt.Sprintf("[%d]", p.Index)
				case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_ANY:
					path += ".any"
				}
			}
			path = strings.TrimLeft(path, ".")
			value = comp.Value
			massertType = massert.AssertType(comp.Kind)
		}
	}

	return massert.Assert{
		ExampleID: exampleID,
		Path:      path,
		Value:     value,
		Type:      massertType,
	}
}
