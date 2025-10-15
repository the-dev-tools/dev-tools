package ttag

import (
	"testing"

	"the-dev-tools/server/pkg/model/mtag"
	tagv1 "the-dev-tools/spec/dist/buf/go/tag/v1"
)

func TestModelTagColorToProto(t *testing.T) {
	t.Helper()

	for model, expected := range modelToProtoColor {
		model := model
		t.Run(expected.String(), func(t *testing.T) {
			got, err := modelTagColorToProto(model)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != expected {
				t.Fatalf("unexpected proto colour: got %v want %v", got, expected)
			}
		})
	}

	t.Run("Unknown", func(t *testing.T) {
		unknown := mtag.Color(255)
		got, err := modelTagColorToProto(unknown)
		if err == nil {
			t.Fatalf("expected error for unknown colour")
		}
		if got != fallbackTagColor() {
			t.Fatalf("expected fallback colour, got %v", got)
		}
	})
}

func TestProtoTagColorToModel(t *testing.T) {
	t.Helper()

	for proto, expected := range protoToModelColor {
		proto := proto
		t.Run(proto.String(), func(t *testing.T) {
			got, err := protoTagColorToModel(proto)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != expected {
				t.Fatalf("unexpected model colour: got %v want %v", got, expected)
			}
		})
	}

	t.Run("Unspecified", func(t *testing.T) {
		if _, err := protoTagColorToModel(fallbackTagColor()); err == nil {
			t.Fatalf("expected error for unspecified colour")
		}
	})

	t.Run("Unknown", func(t *testing.T) {
		unknown := tagv1.TagColor(255)
		if _, err := protoTagColorToModel(unknown); err == nil {
			t.Fatalf("expected error for unknown colour")
		}
	})
}

func TestTagColorRoundTrip(t *testing.T) {
	for modelColour := range modelToProtoColor {
		protoColour, err := modelTagColorToProto(modelColour)
		if err != nil {
			t.Fatalf("modelTagColorToProto(%v) error = %v", modelColour, err)
		}

		back, err := protoTagColorToModel(protoColour)
		if err != nil {
			t.Fatalf("protoTagColorToModel(%v) error = %v", protoColour, err)
		}

		if back != modelColour {
			t.Fatalf("round-trip mismatch: start %v end %v", modelColour, back)
		}
	}
}
