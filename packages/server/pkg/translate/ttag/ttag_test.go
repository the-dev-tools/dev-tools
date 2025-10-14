package ttag

import (
	"testing"

	"the-dev-tools/server/pkg/model/mtag"
	tagv1 "the-dev-tools/spec/dist/buf/go/tag/v1"
)

func TestModelTagColorToProto(t *testing.T) {
	cases := []struct {
		name    string
		input   mtag.Color
		want    tagv1.TagColor
		wantErr bool
	}{
		{name: "Slate", input: mtag.ColorSlate, want: tagv1.TagColor_TAG_COLOR_SLATE},
		{name: "Green", input: mtag.ColorGreen, want: tagv1.TagColor_TAG_COLOR_GREEN},
		{name: "Amber", input: mtag.ColorAmber, want: tagv1.TagColor_TAG_COLOR_AMBER},
		{name: "Sky", input: mtag.ColorSky, want: tagv1.TagColor_TAG_COLOR_SKY},
		{name: "Purple", input: mtag.ColorPurple, want: tagv1.TagColor_TAG_COLOR_PURPLE},
		{name: "Rose", input: mtag.ColorRose, want: tagv1.TagColor_TAG_COLOR_ROSE},
		{name: "Blue", input: mtag.ColorBlue, want: tagv1.TagColor_TAG_COLOR_BLUE},
		{name: "Fuchsia", input: mtag.ColorFuchsia, want: tagv1.TagColor_TAG_COLOR_FUCHSIA},
		{name: "Unknown", input: mtag.Color(99), want: tagv1.TagColor_TAG_COLOR_UNSPECIFIED, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := modelTagColorToProto(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if got != tc.want {
					t.Fatalf("unexpected fallback colour: got %v want %v", got, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected colour: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestProtoTagColorToModel(t *testing.T) {
	cases := []struct {
		name    string
		input   tagv1.TagColor
		want    mtag.Color
		wantErr bool
	}{
		{name: "Slate", input: tagv1.TagColor_TAG_COLOR_SLATE, want: mtag.ColorSlate},
		{name: "Green", input: tagv1.TagColor_TAG_COLOR_GREEN, want: mtag.ColorGreen},
		{name: "Amber", input: tagv1.TagColor_TAG_COLOR_AMBER, want: mtag.ColorAmber},
		{name: "Sky", input: tagv1.TagColor_TAG_COLOR_SKY, want: mtag.ColorSky},
		{name: "Purple", input: tagv1.TagColor_TAG_COLOR_PURPLE, want: mtag.ColorPurple},
		{name: "Rose", input: tagv1.TagColor_TAG_COLOR_ROSE, want: mtag.ColorRose},
		{name: "Blue", input: tagv1.TagColor_TAG_COLOR_BLUE, want: mtag.ColorBlue},
		{name: "Fuchsia", input: tagv1.TagColor_TAG_COLOR_FUCHSIA, want: mtag.ColorFuchsia},
		{name: "Unspecified", input: tagv1.TagColor_TAG_COLOR_UNSPECIFIED, wantErr: true},
		{name: "Unknown", input: tagv1.TagColor(99), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := protoTagColorToModel(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected colour: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestTagColorRoundTrip(t *testing.T) {
	values := []mtag.Color{
		mtag.ColorSlate,
		mtag.ColorGreen,
		mtag.ColorAmber,
		mtag.ColorSky,
		mtag.ColorPurple,
		mtag.ColorRose,
		mtag.ColorBlue,
		mtag.ColorFuchsia,
	}

	for _, value := range values {
		protoColor, err := modelTagColorToProto(value)
		if err != nil {
			t.Fatalf("modelTagColorToProto(%v) error = %v", value, err)
		}

		roundTrip, err := protoTagColorToModel(protoColor)
		if err != nil {
			t.Fatalf("protoTagColorToModel(%v) error = %v", protoColor, err)
		}

		if roundTrip != value {
			t.Fatalf("round-trip mismatch: start %v end %v", value, roundTrip)
		}
	}
}
