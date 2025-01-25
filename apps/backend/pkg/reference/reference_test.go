package reference_test

import (
	"testing"
	"the-dev-tools/backend/pkg/reference"
	"the-dev-tools/backend/pkg/testutil"
)

func TestConvertMapToReference_SimpleMap(t *testing.T) {
	input := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]interface{}{
			"subKey1": "subValue1",
		},
	}
	key := "testKey"

	expected := reference.Reference{
		Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
		Key: reference.ReferenceKey{
			Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  key,
		},
		Map: []reference.Reference{
			{
				Kind: reference.ReferenceKind_REFERENCE_KIND_VALUE,
				Key: reference.ReferenceKey{
					Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
					Key:  "key1",
				},
				Value: "value1",
			},
			{
				Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
				Key: reference.ReferenceKey{
					Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
					Key:  "key2",
				},
				Map: []reference.Reference{
					{
						Kind: reference.ReferenceKind_REFERENCE_KIND_VALUE,
						Key: reference.ReferenceKey{
							Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
							Key:  "subKey1",
						},
						Value: "subValue1",
					},
				},
			},
		},
	}

	got, err := reference.ConvertMapToReference(input, key)
	testutil.Assert(t, nil, err)
	testutil.Assert(t, expected.Kind, got.Kind)
	testutil.Assert(t, expected.Key, got.Key)
	testutil.Assert(t, len(expected.Map), len(got.Map))

	for i := range got.Map {
		testutil.Assert(t, expected.Map[i].Kind, got.Map[i].Kind)
		testutil.Assert(t, expected.Map[i].Key, got.Map[i].Key)
		testutil.Assert(t, expected.Map[i].Value, got.Map[i].Value)
		testutil.Assert(t, len(expected.Map[i].Map), len(got.Map[i].Map))
	}
}

func TestConvertMapToReference_NilMap(t *testing.T) {
	_, err := reference.ConvertMapToReference(nil, "testKey")
	testutil.Assert(t, reference.ErrNilMap, err)
}

func TestConvertMapToReference_EmptyMap(t *testing.T) {
	input := map[string]interface{}{}
	key := "testKey"

	expected := reference.Reference{
		Kind: reference.ReferenceKind_REFERENCE_KIND_MAP,
		Key: reference.ReferenceKey{
			Kind: reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  key,
		},
		Map: []reference.Reference{},
	}

	got, err := reference.ConvertMapToReference(input, key)
	testutil.Assert(t, nil, err)
	testutil.Assert(t, expected.Kind, got.Kind)
	testutil.Assert(t, expected.Key, got.Key)
	testutil.Assert(t, len(expected.Map), len(got.Map))
}
