package tassert

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/massert"
	requestv1 "dev-tools-spec/dist/buf/go/collection/item/request/v1"
	"strings"
)

func SerializeAssertModelToRPC(a massert.Assert) *requestv1.Assert {
	var pathKeys []*requestv1.PathKey
	str := strings.Split(a.Path, ",")
	for i, s := range str {
		pathKey := requestv1.PathKey{
			Key:   s,
			Index: int32(i),
		}
		if s != "any" {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX
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
	}
}

func SerializeAssertModelToRPCItem(a massert.Assert) *requestv1.AssertListItem {
	var pathKeys []*requestv1.PathKey
	str := strings.Split(a.Path, ",")
	for i, s := range str {
		pathKey := requestv1.PathKey{
			Key:   s,
			Index: int32(i),
		}
		if s != "any" {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX
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
	}
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
		if p.Kind == requestv1.PathKind_PATH_KIND_INDEX {
			path += p.Key + ","
		} else {
			path += "*,"
		}
	}

	return massert.Assert{
		ExampleID: exampleID,
		Path:      path,
		Value:     a.Value,
		Type:      massert.AssertType(a.Type),
	}
}
