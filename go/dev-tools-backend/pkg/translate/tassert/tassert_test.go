package tassert_test

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/massert"
	"dev-tools-backend/pkg/translate/tassert"
	requestv1 "dev-tools-spec/dist/buf/go/collection/item/request/v1"
	"fmt"
	"strings"
	"testing"
)

func TestSerializeAssertModelToRPC(t *testing.T) {
	id := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	abc := []string{"body", "test", "test2", "test3"}
	path := strings.Join(abc, ".")

	assert := massert.Assert{
		ID:        id,
		ExampleID: exampleID,
		Path:      path,
		Value:     "test",
		Enable:    true,
	}

	rpcAssert, err := tassert.SerializeAssertModelToRPC(assert)
	if err != nil {
		t.Fatalf("Error while converting model to rpc: %v", err)
	}

	rpcID, err := idwrap.NewFromBytes(rpcAssert.AssertId)
	if err != nil {
		t.Fatalf("Error while converting bytes to ID: %v", err)
	}
	if id.Compare(rpcID) != 0 {
		t.Errorf("Expected ID to be %v, got %v", id, rpcID)
	}

	for i, pathKey := range rpcAssert.Path {
		if pathKey.Key != abc[i] {
			t.Errorf("Expected path key to be %v, got %v", abc[i], pathKey.Key)
		}
	}
}

func TestSerializeAssertModelToRPCAny(t *testing.T) {
	id := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	abc := []string{"body", "test", "any", "test3"}
	path := strings.Join(abc, ".")

	assert := massert.Assert{
		ID:        id,
		ExampleID: exampleID,
		Path:      path,
		Value:     "test",
		Enable:    true,
	}

	rpcAssert, err := tassert.SerializeAssertModelToRPC(assert)
	if err != nil {
		t.Fatalf("Error while converting model to rpc: %v", err)
	}

	rpcID, err := idwrap.NewFromBytes(rpcAssert.AssertId)
	if err != nil {
		t.Fatalf("Error while converting bytes to ID: %v", err)
	}
	if id.Compare(rpcID) != 0 {
		t.Errorf("Expected ID to be %v, got %v", id, rpcID)
	}

	for i, pathKey := range rpcAssert.Path {
		if pathKey.Key != abc[i] {
			t.Errorf("Expected path key to be %v, got %v", abc[i], pathKey.Key)
		}
	}
}

func TestSerializeAssertModelToRPCItem(t *testing.T) {
	id := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	abc := []string{"body", "test", "test2", "test3"}
	path := strings.Join(abc, ".")
	assert := massert.Assert{
		ID:        id,
		ExampleID: exampleID,
		Path:      path,
		Value:     "test",
		Enable:    true,
	}

	rpcAssert, err := tassert.SerializeAssertModelToRPC(assert)
	if err != nil {
		t.Fatalf("Error while converting model to rpc: %v", err)
	}

	rpcID, err := idwrap.NewFromBytes(rpcAssert.AssertId)
	if err != nil {
		t.Fatalf("Error while converting bytes to ID: %v", err)
	}
	if id.Compare(rpcID) != 0 {
		t.Errorf("Expected ID to be %v, got %v", id, rpcID)
	}

	for i, pathKey := range rpcAssert.Path {
		if pathKey.Key != abc[i] {
			t.Errorf("Expected path key to be %v, got %v", abc[i], pathKey.Key)
		}
	}
}

func TestSerializeAssertRPCToModel(t *testing.T) {
	id := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	keys := []string{"body", "", "any", "test3"}
	keysAnyBool := []bool{false, false, true, false}
	keyIndexBool := []bool{false, true, false, false}
	keyIndexInt := []int32{-1, 2, -1, -1}
	var pathKeys []*requestv1.PathKey
	for i, s := range keys {
		pathKey := requestv1.PathKey{
			Kind: requestv1.PathKind_PATH_KIND_UNSPECIFIED,
			Key:  s,
		}
		if keysAnyBool[i] {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX_ANY
		}
		if keyIndexBool[i] {
			pathKey.Kind = requestv1.PathKind_PATH_KIND_INDEX
			pathKey.Index = keyIndexInt[i]
			pathKey.Key = ""
		}
		pathKeys = append(pathKeys, &pathKey)
	}

	rpcAssert := &requestv1.Assert{
		AssertId: id.Bytes(),
		Path:     pathKeys,
		Value:    "test",
		Type:     requestv1.AssertKind_ASSERT_KIND_EQUAL,
	}

	assert, err := tassert.SerializeAssertRPCToModel(rpcAssert, exampleID)
	if err != nil {
		t.Fatalf("Error while converting bytes to ID: %v", err)
	}

	if id.Compare(assert.ID) != 0 {
		t.Errorf("Expected ID to be %v, got %v", id, assert.ID)
	}

	// TODO: refactor this
	paths := strings.Split(assert.Path, ".")
	for i, path := range paths {
		if path == "any" {
			if !keysAnyBool[i] {
				t.Errorf("Expected path key to be %v, got %v", keysAnyBool[i], path)
			}
			return
		}
		if path != keys[i] {
			if keyIndexBool[i] {
				if path != fmt.Sprintf("%s[%d]", keys[i-1], keyIndexInt[i]) {
					t.Errorf("Expected path key to be %v, got %v", fmt.Sprintf("%s[%d]", keys[i-1], keyIndexInt[i]), path)
				}
				return
			}

			if keyIndexBool[i+1] {
				if path != fmt.Sprintf("%s[%d]", keys[i], keyIndexInt[i+1]) {
					t.Errorf("Expected path key to be %v, got %v", fmt.Sprintf("%s[%d]", keys[i], keyIndexInt[i+1]), path)
				}
				return
			}
			t.Errorf("Expected path key to be %v, got %v", keys[i], path)
		}
	}
}
