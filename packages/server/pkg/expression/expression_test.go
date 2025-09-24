package expression

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/expr-lang/expr/file"
	"the-dev-tools/server/pkg/errmap"
)

type sampleNested struct {
	ID   int               `json:"id"`
	Tags []string          `json:"tags"`
	Meta map[string]uint32 `json:"meta"`
}

type sampleStruct struct {
	Name      string        `json:"name"`
	Count     int           `json:"count"`
	Nested    sampleNested  `json:"nested"`
	Optional  string        `json:"optional,omitempty"`
	Ignored   string        `json:"-"`
	Raw       []byte        `json:"raw"`
	Timestamp time.Time     `json:"timestamp"`
	Ptr       *sampleNested `json:"ptr,omitempty"`
}

func legacyConvert(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func TestConvertStructToMapWithJSONTagsMatchesLegacy(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
	s := sampleStruct{
		Name:  "example",
		Count: 42,
		Nested: sampleNested{
			ID:   7,
			Tags: []string{"a", "b"},
			Meta: map[string]uint32{"views": 5},
		},
		Optional:  "",
		Ignored:   "should be skipped",
		Raw:       []byte("hello"),
		Timestamp: now,
		Ptr: &sampleNested{
			ID:   9,
			Tags: []string{"c"},
		},
	}

	expected, err := legacyConvert(s)
	if err != nil {
		t.Fatalf("legacy convert failed: %v", err)
	}

	got, err := convertStructToMapWithJSONTags(s)
	if err != nil {
		t.Fatalf("convertStructToMapWithJSONTags returned error: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("conversion mismatch\nexpected: %#v\n     got: %#v", expected, got)
	}
}

func TestConvertHandlesMapAndSlice(t *testing.T) {
	input := map[string]any{
		"numbers": []int{1, 2, 3},
		"mixed":   []any{"a", 5},
	}

	expected, err := legacyConvert(input)
	if err != nil {
		t.Fatalf("legacy convert failed: %v", err)
	}

	got, err := convertStructToMapWithJSONTags(input)
	if err != nil {
		t.Fatalf("convertStructToMapWithJSONTags returned error: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("conversion mismatch\nexpected: %#v\n     got: %#v", expected, got)
	}
}

func TestExpressionEvaluteAsBool_SyntaxErrorFriendly(t *testing.T) {
	env := NewEnv(map[string]any{
		"flag": true,
	})

	_, err := ExpressionEvaluteAsBool(context.Background(), env, "flag &&")
	if err == nil {
		t.Fatal("expected syntax error, got nil")
	}

	var friendly *errmap.Error
	if !errors.As(err, &friendly) {
		t.Fatalf("expected errmap.Error, got %T", err)
	}

	if friendly.Code != errmap.CodeExpressionSyntax {
		t.Fatalf("expected CodeExpressionSyntax, got %s", friendly.Code)
	}

	if !strings.Contains(friendly.Message, "line 1") {
		t.Fatalf("expected line information in message, got %q", friendly.Message)
	}

	if !strings.Contains(friendly.Message, "^") {
		t.Fatalf("expected caret indicator in message, got %q", friendly.Message)
	}

	var fileErr *file.Error
	if !errors.As(err, &fileErr) {
		t.Fatalf("expected underlying file.Error, got %T", err)
	}
}

func TestExpressionEvaluteAsBool_RuntimeErrorFriendly(t *testing.T) {
	env := NewEnv(map[string]any{
		"boom": func() bool { panic("boom panic") },
	})

	_, err := ExpressionEvaluteAsBool(context.Background(), env, "boom()")
	if err == nil {
		t.Fatal("expected runtime error, got nil")
	}

	var friendly *errmap.Error
	if !errors.As(err, &friendly) {
		t.Fatalf("expected errmap.Error, got %T", err)
	}

	if friendly.Code != errmap.CodeExpressionRuntime {
		t.Fatalf("expected CodeExpressionRuntime, got %s", friendly.Code)
	}

	if !strings.Contains(friendly.Message, "boom") {
		t.Fatalf("expected panic description in message, got %q", friendly.Message)
	}

	if !strings.Contains(friendly.Message, "line 1") {
		t.Fatalf("expected line information in message, got %q", friendly.Message)
	}

	var fileErr *file.Error
	if !errors.As(err, &fileErr) {
		t.Fatalf("expected underlying file.Error, got %T", err)
	}
}

func BenchmarkLegacyConvertStruct(b *testing.B) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
	s := sampleStruct{
		Name:  "example",
		Count: 42,
		Nested: sampleNested{
			ID:   7,
			Tags: []string{"a", "b", "c"},
			Meta: map[string]uint32{"views": 5, "likes": 3},
		},
		Raw:       []byte("hello world"),
		Timestamp: now,
		Ptr: &sampleNested{
			ID:   9,
			Tags: []string{"c", "d"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := legacyConvert(s); err != nil {
			b.Fatalf("legacy convert failed: %v", err)
		}
	}
}

func BenchmarkConvertStructWithJSONTags(b *testing.B) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
	s := sampleStruct{
		Name:  "example",
		Count: 42,
		Nested: sampleNested{
			ID:   7,
			Tags: []string{"a", "b", "c"},
			Meta: map[string]uint32{"views": 5, "likes": 3},
		},
		Raw:       []byte("hello world"),
		Timestamp: now,
		Ptr: &sampleNested{
			ID:   9,
			Tags: []string{"c", "d"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := convertStructToMapWithJSONTags(s); err != nil {
			b.Fatalf("convertStructToMapWithJSONTags failed: %v", err)
		}
	}
}
