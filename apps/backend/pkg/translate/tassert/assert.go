package tassert

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massert"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) (*requestv1.Assert, error) {
	var pathKeys []*requestv1.PathKey
	str := strings.Split(a.Path, ".")
	arrayRegex := regexp.MustCompile(`\[(\d+)\]`)
	for _, s := range str {
		pathKey := requestv1.PathKey{
			Key: s,
		}
		arr := arrayRegex.MatchString(s)
		if arr {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX
			path := arrayRegex.FindStringSubmatch(s)[1]
			pathInt, err := strconv.Atoi(path)
			if err != nil {
				return nil, err
			}
			pathKey.Index = int32(pathInt)
		}
		if s != "any" {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_UNSPECIFIED
		} else {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX_ANY
		}
		pathKeys = append(pathKeys, &pathKey)
	}

	return &requestv1.Assert{
		AssertId: a.ID.Bytes(),
		Path:     pathKeys,
		Value:    a.Value,
		Type:     requestv1.AssertKind(a.Type),
	}, nil
}

func SerializeAssertModelToRPCItem(a massert.Assert) (*requestv1.AssertListItem, error) {
	var pathKeys []*requestv1.PathKey
	str := strings.Split(a.Path, ".")
	arrayRegex := regexp.MustCompile(`\[(\d+)\]`)
	for _, s := range str {
		pathKey := requestv1.PathKey{
			Key: s,
		}
		arr := arrayRegex.MatchString(s)
		if arr {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX
			path := arrayRegex.FindStringSubmatch(s)[1]
			pathInt, err := strconv.Atoi(path)
			if err != nil {
				return nil, err
			}
			pathKey.Index = int32(pathInt)
		}
		if s != "any" {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_UNSPECIFIED
		} else {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX_ANY
		}
		pathKeys = append(pathKeys, &pathKey)
	}

	return &requestv1.AssertListItem{
		AssertId: a.ID.Bytes(),
		Path:     pathKeys,
		Value:    a.Value,
		Type:     requestv1.AssertKind(a.Type),
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
		case requestv1.PathKind_PATH_KIND_UNSPECIFIED:
			path += "." + p.Key
		case requestv1.PathKind_PATH_KIND_INDEX:
			path += fmt.Sprintf("[%d]", p.Index)
		case requestv1.PathKind_PATH_KIND_INDEX_ANY:
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
