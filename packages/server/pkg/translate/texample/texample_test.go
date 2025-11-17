package texample

import (
	"bytes"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	bodyv1 "the-dev-tools/spec/dist/buf/go/api/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/api/collection/item/example/v1"
)

func TestModelBodyTypeToProto(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		input mitemapiexample.BodyType
		want  bodyv1.BodyKind
	}{
		{
			name:  "None",
			input: mitemapiexample.BodyTypeNone,
			want:  bodyv1.BodyKind_BODY_KIND_UNSPECIFIED,
		},
		{
			name:  "Form",
			input: mitemapiexample.BodyTypeForm,
			want:  bodyv1.BodyKind_BODY_KIND_FORM_ARRAY,
		},
		{
			name:  "Urlencoded",
			input: mitemapiexample.BodyTypeUrlencoded,
			want:  bodyv1.BodyKind_BODY_KIND_URL_ENCODED_ARRAY,
		},
		{
			name:  "Raw",
			input: mitemapiexample.BodyTypeRaw,
			want:  bodyv1.BodyKind_BODY_KIND_RAW,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := modelBodyTypeToProto(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("modelBodyTypeToProto(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestModelBodyTypeToProtoUnknown(t *testing.T) {
	unknown := mitemapiexample.BodyType(127)

	got, err := modelBodyTypeToProto(unknown)
	if err == nil {
		t.Fatalf("expected error for unknown body type")
	}
	if got != fallbackBodyKind() {
		t.Fatalf("expected fallback body kind, got %v", got)
	}
}

func TestProtoBodyKindToModel(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		input bodyv1.BodyKind
		want  mitemapiexample.BodyType
	}{
		{
			name:  "Unspecified",
			input: fallbackBodyKind(),
			want:  mitemapiexample.BodyTypeNone,
		},
		{
			name:  "Form",
			input: bodyv1.BodyKind_BODY_KIND_FORM_ARRAY,
			want:  mitemapiexample.BodyTypeForm,
		},
		{
			name:  "Urlencoded",
			input: bodyv1.BodyKind_BODY_KIND_URL_ENCODED_ARRAY,
			want:  mitemapiexample.BodyTypeUrlencoded,
		},
		{
			name:  "Raw",
			input: bodyv1.BodyKind_BODY_KIND_RAW,
			want:  mitemapiexample.BodyTypeRaw,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := protoBodyKindToModel(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("protoBodyKindToModel(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestProtoBodyKindToModelUnknown(t *testing.T) {
	unknown := bodyv1.BodyKind(255)
	if _, err := protoBodyKindToModel(unknown); err == nil {
		t.Fatalf("expected error for unknown body kind")
	}
}

func TestBodyTypeRoundTrip(t *testing.T) {
	t.Helper()

	bodyTypes := []mitemapiexample.BodyType{
		mitemapiexample.BodyTypeNone,
		mitemapiexample.BodyTypeForm,
		mitemapiexample.BodyTypeUrlencoded,
		mitemapiexample.BodyTypeRaw,
	}

	for _, bt := range bodyTypes {
		protoKind, err := modelBodyTypeToProto(bt)
		if err != nil {
			t.Fatalf("modelBodyTypeToProto(%v) error = %v", bt, err)
		}

		back, err := protoBodyKindToModel(protoKind)
		if err != nil {
			t.Fatalf("protoBodyKindToModel(%v) error = %v", protoKind, err)
		}

		if back != bt {
			t.Fatalf("round-trip mismatch: start %v end %v", bt, back)
		}
	}
}

func TestSerializeModelToRPC(t *testing.T) {
	id := idwrap.NewNow()

	tests := []struct {
		name     string
		bodyType mitemapiexample.BodyType
		want     bodyv1.BodyKind
	}{
		{
			name:     "MapsKnownBodyType",
			bodyType: mitemapiexample.BodyTypeRaw,
			want:     bodyv1.BodyKind_BODY_KIND_RAW,
		},
		{
			name:     "FallsBackOnUnknownBodyType",
			bodyType: mitemapiexample.BodyType(127),
			want:     fallbackBodyKind(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			model := mitemapiexample.ItemApiExample{
				ID:       id,
				Name:     "example",
				BodyType: tc.bodyType,
			}

			rpc := SerializeModelToRPC(model, nil, nil)
			if rpc == nil {
				t.Fatalf("expected non-nil RPC example")
			}

			if !bytes.Equal(rpc.GetExampleId(), id.Bytes()) {
				t.Fatalf("unexpected example id: got %v want %v", rpc.GetExampleId(), id.Bytes())
			}

			if rpc.GetName() != model.Name {
				t.Fatalf("unexpected name: got %q want %q", rpc.GetName(), model.Name)
			}

			if rpc.GetBodyKind() != tc.want {
				t.Fatalf("unexpected body kind: got %v want %v", rpc.GetBodyKind(), tc.want)
			}
		})
	}
}

func TestDeserializeRPCToModel(t *testing.T) {
	id := idwrap.NewNow()

	tests := []struct {
		name     string
		bodyKind bodyv1.BodyKind
		want     mitemapiexample.BodyType
	}{
		{
			name:     "MapsKnownBodyKind",
			bodyKind: bodyv1.BodyKind_BODY_KIND_FORM_ARRAY,
			want:     mitemapiexample.BodyTypeForm,
		},
		{
			name:     "MapsFallbackBodyKind",
			bodyKind: fallbackBodyKind(),
			want:     mitemapiexample.BodyTypeNone,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rpc := &examplev1.Example{
				ExampleId: id.Bytes(),
				Name:      "example",
				BodyKind:  tc.bodyKind,
			}

			model, err := DeserializeRPCToModel(rpc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if model.ID.Compare(id) != 0 {
				t.Fatalf("unexpected id: got %v want %v", model.ID, id)
			}

			if model.Name != rpc.GetName() {
				t.Fatalf("unexpected name: got %q want %q", model.Name, rpc.GetName())
			}

			if model.BodyType != tc.want {
				t.Fatalf("unexpected body type: got %v want %v", model.BodyType, tc.want)
			}
		})
	}
}

func TestDeserializeRPCToModelNil(t *testing.T) {
	model, err := DeserializeRPCToModel(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != (mitemapiexample.ItemApiExample{}) {
		t.Fatalf("expected zero value model, got %+v", model)
	}
}

func TestDeserializeRPCToModelUnknownBodyKind(t *testing.T) {
	id := idwrap.NewNow()

	rpc := &examplev1.Example{
		ExampleId: id.Bytes(),
		BodyKind:  bodyv1.BodyKind(255),
	}

	if _, err := DeserializeRPCToModel(rpc); err == nil {
		t.Fatalf("expected error for unknown body kind")
	}
}
