package tbreadcrumbs

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexamplebreadcrumb"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
)

func TestModelBreadcrumbKindToProto(t *testing.T) {
	t.Helper()

	for model, expected := range modelToProtoBreadcrumbKind {
		model := model
		expected := expected
		t.Run(expected.String(), func(t *testing.T) {
			got, err := modelBreadcrumbKindToProto(model)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != expected {
				t.Fatalf("unexpected proto kind: got %v want %v", got, expected)
			}
		})
	}

	t.Run("Unknown", func(t *testing.T) {
		unknown := mexamplebreadcrumb.ExampleBreadcrumbKind(255)
		got, err := modelBreadcrumbKindToProto(unknown)
		if err == nil {
			t.Fatalf("expected error for unknown kind")
		}
		if got != fallbackExampleBreadcrumbKind() {
			t.Fatalf("expected fallback kind, got %v", got)
		}
	})
}

func TestProtoBreadcrumbKindToModel(t *testing.T) {
	t.Helper()

	for proto, expected := range protoToModelBreadcrumbKind {
		proto := proto
		expected := expected
		t.Run(proto.String(), func(t *testing.T) {
			got, err := protoBreadcrumbKindToModel(proto)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != expected {
				t.Fatalf("unexpected model kind: got %v want %v", got, expected)
			}
		})
	}

	t.Run("Unspecified", func(t *testing.T) {
		if _, err := protoBreadcrumbKindToModel(fallbackExampleBreadcrumbKind()); err == nil {
			t.Fatalf("expected error for unspecified kind")
		}
	})

	t.Run("Unknown", func(t *testing.T) {
		unknown := examplev1.ExampleBreadcrumbKind(255)
		if _, err := protoBreadcrumbKindToModel(unknown); err == nil {
			t.Fatalf("expected error for unknown proto kind")
		}
	})
}

func TestSerializeDeserializeRoundTrip(t *testing.T) {
	collectionID := idwrap.NewTextMust("01H7Z4P7X5TB7ZXN6FYA2R70VF")
	folderID := idwrap.NewTextMust("01H7Z4P7X5TB7ZXN6FYA2R70VG")
	folderParentID := idwrap.NewTextMust("01H7Z4P7X5TB7ZXN6FYA2R70VH")
	endpointID := idwrap.NewTextMust("01H7Z4P7X5TB7ZXN6FYA2R70VI")
	endpointParentID := idwrap.NewTextMust("01H7Z4P7X5TB7ZXN6FYA2R70VJ")

	tests := []struct {
		name       string
		breadcrumb mexamplebreadcrumb.ExampleBreadcrumb
		assert     func(*testing.T, mexamplebreadcrumb.ExampleBreadcrumb)
	}{
		{
			name: "Collection",
			breadcrumb: mexamplebreadcrumb.ExampleBreadcrumb{
				Kind: mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_COLLECTION,
				Collection: &mcollection.Collection{
					ID:   collectionID,
					Name: "collection",
				},
			},
			assert: func(t *testing.T, got mexamplebreadcrumb.ExampleBreadcrumb) {
				if got.Collection == nil {
					t.Fatalf("expected collection to be populated")
				}
				if got.Collection.ID != collectionID {
					t.Fatalf("collection id mismatch: got %v want %v", got.Collection.ID, collectionID)
				}
				if got.Collection.Name != "collection" {
					t.Fatalf("collection name mismatch: got %s", got.Collection.Name)
				}
			},
		},
		{
			name: "Folder",
			breadcrumb: mexamplebreadcrumb.ExampleBreadcrumb{
				Kind: mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_FOLDER,
				Folder: &mitemfolder.ItemFolder{
					ID:       folderID,
					ParentID: &folderParentID,
					Name:     "folder",
				},
			},
			assert: func(t *testing.T, got mexamplebreadcrumb.ExampleBreadcrumb) {
				if got.Folder == nil {
					t.Fatalf("expected folder to be populated")
				}
				if got.Folder.ID != folderID {
					t.Fatalf("folder id mismatch: got %v want %v", got.Folder.ID, folderID)
				}
				if got.Folder.ParentID == nil || *got.Folder.ParentID != folderParentID {
					t.Fatalf("folder parent mismatch: got %v want %v", got.Folder.ParentID, folderParentID)
				}
				if got.Folder.Name != "folder" {
					t.Fatalf("folder name mismatch: got %s", got.Folder.Name)
				}
			},
		},
		{
			name: "Endpoint",
			breadcrumb: mexamplebreadcrumb.ExampleBreadcrumb{
				Kind: mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_ENDPOINT,
				Endpoint: &mitemapi.ItemApi{
					ID:       endpointID,
					FolderID: &endpointParentID,
					Name:     "endpoint",
					Method:   "GET",
					Hidden:   true,
				},
			},
			assert: func(t *testing.T, got mexamplebreadcrumb.ExampleBreadcrumb) {
				if got.Endpoint == nil {
					t.Fatalf("expected endpoint to be populated")
				}
				if got.Endpoint.ID != endpointID {
					t.Fatalf("endpoint id mismatch: got %v want %v", got.Endpoint.ID, endpointID)
				}
				if got.Endpoint.FolderID == nil || *got.Endpoint.FolderID != endpointParentID {
					t.Fatalf("endpoint parent mismatch: got %v want %v", got.Endpoint.FolderID, endpointParentID)
				}
				if got.Endpoint.Name != "endpoint" {
					t.Fatalf("endpoint name mismatch: got %s", got.Endpoint.Name)
				}
				if got.Endpoint.Method != "GET" {
					t.Fatalf("endpoint method mismatch: got %s", got.Endpoint.Method)
				}
				if got.Endpoint.Hidden != true {
					t.Fatalf("endpoint hidden mismatch: got %v", got.Endpoint.Hidden)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			proto := SerializeModelToRPC(tt.breadcrumb)
			expectedKind, err := modelBreadcrumbKindToProto(tt.breadcrumb.Kind)
			if err != nil {
				t.Fatalf("unexpected conversion error: %v", err)
			}
			if proto.GetKind() != expectedKind {
				t.Fatalf("proto kind mismatch: got %v want %v", proto.GetKind(), expectedKind)
			}

			roundTrip, err := DeserializeRPCToModel(proto)
			if err != nil {
				t.Fatalf("DeserializeRPCToModel() error = %v", err)
			}
			if roundTrip.Kind != tt.breadcrumb.Kind {
				t.Fatalf("kind mismatch after round-trip: got %v want %v", roundTrip.Kind, tt.breadcrumb.Kind)
			}

			if tt.assert != nil {
				tt.assert(t, roundTrip)
			}
		})
	}
}

func TestSerializeModelToRPCFallback(t *testing.T) {
	breadcrumb := mexamplebreadcrumb.ExampleBreadcrumb{Kind: mexamplebreadcrumb.ExampleBreadcrumbKind(200)}

	proto := SerializeModelToRPC(breadcrumb)
	if proto.GetKind() != fallbackExampleBreadcrumbKind() {
		t.Fatalf("expected fallback kind, got %v", proto.GetKind())
	}
}

func TestDeserializeRPCToModelErrors(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		got, err := DeserializeRPCToModel(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != (mexamplebreadcrumb.ExampleBreadcrumb{}) {
			t.Fatalf("expected zero value breadcrumb, got %+v", got)
		}
	})

	t.Run("Unspecified", func(t *testing.T) {
		proto := &examplev1.ExampleBreadcrumb{Kind: fallbackExampleBreadcrumbKind()}
		if _, err := DeserializeRPCToModel(proto); err == nil {
			t.Fatalf("expected error for unspecified kind")
		}
	})

	t.Run("UnknownKind", func(t *testing.T) {
		proto := &examplev1.ExampleBreadcrumb{Kind: examplev1.ExampleBreadcrumbKind(255)}
		if _, err := DeserializeRPCToModel(proto); err == nil {
			t.Fatalf("expected error for unknown enum value")
		}
	})

	t.Run("MissingCollection", func(t *testing.T) {
		proto := &examplev1.ExampleBreadcrumb{Kind: examplev1.ExampleBreadcrumbKind_EXAMPLE_BREADCRUMB_KIND_COLLECTION}
		if _, err := DeserializeRPCToModel(proto); err == nil {
			t.Fatalf("expected error when collection payload missing")
		}
	})
}
