package reference_test

import (
	"reflect"
	"sort"
	"testing"
	"the-dev-tools/server/pkg/reference"
)

func sortReferences(refs []reference.ReferenceTreeItem) {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Key.Key < refs[j].Key.Key
	})
}

func TestNewReferenceFromMap(t *testing.T) {
	input := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	expected := reference.ReferenceTreeItem{
		Key:  reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""},
		Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
		Map: []reference.ReferenceTreeItem{
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "key1"},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "value1",
			},
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "key2"},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "42",
			},
		},
	}

	result := reference.NewReferenceFromInterface(input, reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""})
	sortReferences(result.Map)
	sortReferences(expected.Map)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestNewReferenceFromSlice(t *testing.T) {
	mapA := map[string]any{"a": 1, "b": 2}
	input := []any{"value1", 42, mapA}
	expected := reference.ReferenceTreeItem{
		Key:  reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""},
		Kind: reference.ReferenceKind_REFERENCE_KIND_ARRAY,
		Array: []reference.ReferenceTreeItem{
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: 0},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "value1",
			},
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: 1},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "42",
			},
			{
				Key:  reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: 2},
				Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
				Map: []reference.ReferenceTreeItem{
					{
						Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "a"},
						Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
						Value: "1",
					},
					{
						Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "b"},
						Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
						Value: "2",
					},
				},
			},
		},
	}

	result := reference.NewReferenceFromInterface(input, reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""})
	sortReferences(result.Array)
	sortReferences(expected.Array)

	// Sort nested maps
	for _, item := range result.Array {
		if item.Kind == reference.ReferenceKind_REFERENCE_KIND_MAP {
			sortReferences(item.Map)
		}
	}
	for _, item := range expected.Array {
		if item.Kind == reference.ReferenceKind_REFERENCE_KIND_MAP {
			sortReferences(item.Map)
		}
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestNewReferenceFromStruct(t *testing.T) {
	type TestStruct struct {
		Field1 string
		Field2 []int
	}
	input := TestStruct{
		Field1: "value1",
		Field2: []int{1, 2, 3},
	}
	expected := reference.ReferenceTreeItem{
		Key:  reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""},
		Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
		Map: []reference.ReferenceTreeItem{
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "Field1"},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "value1",
			},
			{
				Key:  reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "Field2"},
				Kind: reference.ReferenceKind_REFERENCE_KIND_ARRAY,
				Array: []reference.ReferenceTreeItem{
					{Key: reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: 0}, Kind: reference.ReferenceKind_REFERENCE_KIND_VALUE, Value: "1"},
					{Key: reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: 1}, Kind: reference.ReferenceKind_REFERENCE_KIND_VALUE, Value: "2"},
					{Key: reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX, Index: 2}, Kind: reference.ReferenceKind_REFERENCE_KIND_VALUE, Value: "3"},
				},
			},
		},
	}

	result := reference.NewReferenceFromInterface(input, reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""})
	sortReferences(result.Map)
	sortReferences(expected.Map)
	for _, ref := range result.Map {
		if ref.Kind == reference.ReferenceKind_REFERENCE_KIND_ARRAY {
			sortReferences(ref.Array)
		}
	}
	for _, ref := range expected.Map {
		if ref.Kind == reference.ReferenceKind_REFERENCE_KIND_ARRAY {
			sortReferences(ref.Array)
		}
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestNewReferenceFromMapWithStruct(t *testing.T) {
	type TestStruct struct {
		Field1 string
		Field2 int
	}
	input := map[string]interface{}{
		"key1": TestStruct{
			Field1: "value1",
			Field2: 42,
		},
		"key2": "value2",
	}
	expected := reference.ReferenceTreeItem{
		Key:  reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""},
		Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
		Map: []reference.ReferenceTreeItem{
			{
				Key:  reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "key1"},
				Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
				Map: []reference.ReferenceTreeItem{
					{
						Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "Field1"},
						Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
						Value: "value1",
					},
					{
						Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "Field2"},
						Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
						Value: "42",
					},
				},
			},
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "key2"},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "value2",
			},
		},
	}

	result := reference.NewReferenceFromInterface(input, reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""})
	sortReferences(result.Map)
	sortReferences(expected.Map)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

// Benchmarks

func BenchmarkNewReferenceFromInterfaceMap(b *testing.B) {
	input := map[string]interface{}{
		"key1": map[string]interface{}{
			"subkey1": "value1",
			"subkey2": 42,
		},
		"key2": "value2",
	}
	key := reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""}

	for i := 0; i < b.N; i++ {
		_ = reference.NewReferenceFromInterface(input, key)
	}
}

func BenchmarkNewReferenceFromInterfaceArray(b *testing.B) {
	input := []interface{}{"value1", 42}
	key := reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""}

	for i := 0; i < b.N; i++ {
		_ = reference.NewReferenceFromInterface(input, key)
	}
}

func BenchmarkNewReferenceFromInterfacePrimitive(b *testing.B) {
	input := 42
	key := reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""}

	for i := 0; i < b.N; i++ {
		_ = reference.NewReferenceFromInterface(input, key)
	}
}

func BenchmarkNewReferenceFromInterfaceStruct(b *testing.B) {
	type TestStruct struct {
		Field1 string
		Field2 []int
	}
	input := TestStruct{
		Field1: "value1",
		Field2: []int{1, 2, 3},
	}
	key := reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: ""}

	for i := 0; i < b.N; i++ {
		_ = reference.NewReferenceFromInterface(input, key)
	}
}

func TestNewReferenceFromInterface(t *testing.T) {
	input := map[string]any{
		"a": 1,
		"b": 2,
	}

	expected := reference.ReferenceTreeItem{
		Key:  reference.ReferenceKey{},
		Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
		Map: []reference.ReferenceTreeItem{
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "a"},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "1",
			},
			{
				Key:   reference.ReferenceKey{Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "b"},
				Kind:  reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: "2",
			},
		},
	}

	result := reference.NewReferenceFromInterface(input, reference.ReferenceKey{})
	sortReferences(result.Map)
	sortReferences(expected.Map)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
