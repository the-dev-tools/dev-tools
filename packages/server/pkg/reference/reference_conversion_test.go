package reference

import (
	"testing"

	referencev1 "the-dev-tools/spec/dist/buf/go/api/reference/v1"

	"github.com/stretchr/testify/require")

func stringPtr(v string) *string {
	return &v
}

func TestConvertPkgToRpcTreeSuccess(t *testing.T) {
	ref := ReferenceTreeItem{
		Kind:  ReferenceKind_REFERENCE_KIND_VALUE,
		Key:   ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "foo"},
		Value: "bar",
	}

	got, err := ConvertPkgToRpcTree(ref)
	require.NoError(t, err, "ConvertPkgToRpcTree returned unexpected error")

	if got.Kind != referencev1.ReferenceKind_REFERENCE_KIND_VALUE {
		t.Fatalf("unexpected proto kind: %v", got.Kind)
	}
	if got.Key == nil {
		t.Fatalf("expected proto key to be populated")
	}
	if got.Key.Kind != referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY {
		t.Fatalf("unexpected proto key kind: %v", got.Key.Kind)
	}
	if got.Key.Key == nil || *got.Key.Key != "foo" {
		t.Fatalf("unexpected proto key value: %v", got.Key.Key)
	}
	if got.Value == nil || *got.Value != "bar" {
		t.Fatalf("unexpected proto value: %v", got.Value)
	}
}

func TestConvertPkgToRpcTreeInvalidKind(t *testing.T) {
	ref := ReferenceTreeItem{
		Kind: ReferenceKind(99),
		Key:  ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "foo"},
	}

	got, err := ConvertPkgToRpcTree(ref)
	if err == nil {
		t.Fatalf("expected error for invalid kind")
	}
	if got != nil {
		t.Fatalf("expected nil proto result, got: %v", got)
	}
}

func TestConvertPkgToRpcTreeChildError(t *testing.T) {
	child := ReferenceTreeItem{
		Kind: ReferenceKind(101),
		Key:  ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "child"},
	}
	ref := ReferenceTreeItem{
		Kind: ReferenceKind_REFERENCE_KIND_MAP,
		Key:  ReferenceKey{Kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY, Key: "parent"},
		Map:  []ReferenceTreeItem{child},
	}

	got, err := ConvertPkgToRpcTree(ref)
	if err == nil {
		t.Fatalf("expected error for invalid child kind")
	}
	if got != nil {
		t.Fatalf("expected nil proto result, got: %v", got)
	}
}

func TestConvertRpcToPkgSuccess(t *testing.T) {
	proto := &referencev1.ReferenceTreeItem{
		Kind: referencev1.ReferenceKind_REFERENCE_KIND_VALUE,
		Key: &referencev1.ReferenceKey{
			Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  stringPtr("foo"),
		},
		Value: stringPtr("bar"),
	}

	got, err := ConvertRpcToPkg(proto)
	require.NoError(t, err, "ConvertRpcToPkg returned unexpected error")
	if got.Kind != ReferenceKind_REFERENCE_KIND_VALUE {
		t.Fatalf("unexpected package kind: %v", got.Kind)
	}
	if got.Key.Kind != ReferenceKeyKind_REFERENCE_KEY_KIND_KEY {
		t.Fatalf("unexpected package key kind: %v", got.Key.Kind)
	}
	if got.Key.Key != "foo" {
		t.Fatalf("unexpected package key value: %v", got.Key.Key)
	}
	if got.Value != "bar" {
		t.Fatalf("unexpected package value: %v", got.Value)
	}
}

func TestConvertRpcToPkgInvalidKind(t *testing.T) {
	proto := &referencev1.ReferenceTreeItem{
		Kind: referencev1.ReferenceKind(222),
		Key: &referencev1.ReferenceKey{
			Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			Key:  stringPtr("foo"),
		},
	}

	got, err := ConvertRpcToPkg(proto)
	if err == nil {
		t.Fatalf("expected error for invalid proto kind")
	}
	if got.Kind != ReferenceKind_REFERENCE_KIND_UNSPECIFIED {
		t.Fatalf("expected fallback kind, got: %v", got.Kind)
	}
}

func TestConvertRpcToPkgNil(t *testing.T) {
	got, err := ConvertRpcToPkg(nil)
	require.NoError(t, err, "expected no error for nil input, got")
	if got.Kind != ReferenceKind_REFERENCE_KIND_UNSPECIFIED {
		t.Fatalf("expected zero value kind, got: %v", got.Kind)
	}
}

func TestConvertPkgKeyToRpcInvalid(t *testing.T) {
	ref := ReferenceKey{Kind: ReferenceKeyKind(77)}

	got, err := ConvertPkgKeyToRpc(ref)
	if err == nil {
		t.Fatalf("expected error for invalid key kind")
	}
	if got != nil {
		t.Fatalf("expected nil proto key, got: %v", got)
	}
}

func TestConvertRpcKeyToPkgKeyInvalid(t *testing.T) {
	proto := &referencev1.ReferenceKey{Kind: referencev1.ReferenceKeyKind(88)}

	got, err := ConvertRpcKeyToPkgKey(proto)
	if err == nil {
		t.Fatalf("expected error for invalid proto key kind")
	}
	if got.Kind != ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED {
		t.Fatalf("expected fallback key kind, got: %v", got.Kind)
	}
}

func TestConvertRpcKeyToPkgKeyNil(t *testing.T) {
	got, err := ConvertRpcKeyToPkgKey(nil)
	require.NoError(t, err, "expected no error for nil key, got")
	if got.Kind != ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED {
		t.Fatalf("expected zero value key kind, got: %v", got.Kind)
	}
}
