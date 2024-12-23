package tassert

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massert"
	assertv1 "the-dev-tools/spec/dist/buf/go/assert/v1"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) (*requestv1.Assert, error) {
	var pathKeys []*assertv1.PathKey
	str := strings.Split(a.Path, ".")
	arrayRegex := regexp.MustCompile(`\[(\d+)\]`)
	for _, s := range str {
		pathKey := assertv1.PathKey{
			Key: s,
		}
		arr := arrayRegex.MatchString(s)
		if arr {
			pathKey.Kind = assertv1.PathKind_PATH_KIND_INDEX
			path := arrayRegex.FindStringSubmatch(s)[1]
			pathInt, err := strconv.Atoi(path)
			if err != nil {
				return nil, err
			}
			pathKey.Index = int32(pathInt)
		}
		if s != "any" {
			pathKey.Kind = assertv1.PathKind_PATH_KIND_UNSPECIFIED
		} else {
			pathKey.Kind = assertv1.PathKind_PATH_KIND_INDEX_ANY
		}
		pathKeys = append(pathKeys, &pathKey)
	}

	return &requestv1.Assert{
		AssertId: a.ID.Bytes(),
		Path:     pathKeys,
		Value:    a.Value,
		Type:     assertv1.AssertKind(a.Type),
	}, nil
}

func SerializeAssertModelToRPCItem(a massert.Assert) (*requestv1.AssertListItem, error) {
	var pathKeys []*assertv1.PathKey
	str := strings.Split(a.Path, ".")
	arrayRegex := regexp.MustCompile(`\[(\d+)\]`)
	for _, s := range str {
		pathKey := assertv1.PathKey{
			Key: s,
		}
		arr := arrayRegex.MatchString(s)
		if arr {
			pathKey.Kind = assertv1.PathKind_PATH_KIND_INDEX
			path := arrayRegex.FindStringSubmatch(s)[1]
			pathInt, err := strconv.Atoi(path)
			if err != nil {
				return nil, err
			}
			pathKey.Index = int32(pathInt)
		}
		if s != "any" {
			pathKey.Kind = assertv1.PathKind_PATH_KIND_UNSPECIFIED
		} else {
			pathKey.Kind = assertv1.PathKind_PATH_KIND_INDEX_ANY
		}
		pathKeys = append(pathKeys, &pathKey)
	}

	return &requestv1.AssertListItem{
		AssertId: a.ID.Bytes(),
		Path:     pathKeys,
		Value:    a.Value,
		Type:     assertv1.AssertKind(a.Type),
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
	path := ""
	for _, p := range a.Path {
		switch p.Kind {
		case assertv1.PathKind_PATH_KIND_UNSPECIFIED:
			path += "." + p.Key
		case assertv1.PathKind_PATH_KIND_INDEX:
			path += fmt.Sprintf("[%d]", p.Index)
		case assertv1.PathKind_PATH_KIND_INDEX_ANY:
			path += ".any"
		}
	}
	path = strings.TrimLeft(path, ".")

	return massert.Assert{
		ExampleID: exampleID,
		Path:      path,
		Value:     a.Value,
		Type:      massert.AssertType(a.Type),
	}
}
