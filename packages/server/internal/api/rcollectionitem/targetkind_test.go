package rcollectionitem

import (
	"testing"

	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
)

func TestValidateMoveKindCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		sourceKind itemv1.ItemKind
		targetKind itemv1.ItemKind
		wantError  bool
		errorText  string
	}{
		{
			name:       "Unspecified target kind should be valid",
			sourceKind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			targetKind: itemv1.ItemKind_ITEM_KIND_UNSPECIFIED,
			wantError:  false,
		},
		{
			name:       "Folder to folder move should be valid",
			sourceKind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			targetKind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			wantError:  false,
		},
		{
			name:       "Folder to endpoint move should be invalid",
			sourceKind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			targetKind: itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			wantError:  true,
			errorText:  "invalid move: cannot move folder into an endpoint",
		},
		{
			name:       "Endpoint to folder move should be valid",
			sourceKind: itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			targetKind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			wantError:  false,
		},
		{
			name:       "Endpoint to endpoint move should be valid",
			sourceKind: itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			targetKind: itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			wantError:  false,
		},
		{
			name:       "Unspecified source kind should be invalid",
			sourceKind: itemv1.ItemKind_ITEM_KIND_UNSPECIFIED,
			targetKind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			wantError:  true,
			errorText:  "source kind must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMoveKindCompatibility(tt.sourceKind, tt.targetKind)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorText != "" && err.Error() != tt.errorText {
					t.Errorf("expected error text %q, got %q", tt.errorText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}