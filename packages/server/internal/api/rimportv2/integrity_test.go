package rimportv2

import (
	"strings"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
)

func TestValidateTranslationResult(t *testing.T) {
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()
	fileID1 := idwrap.NewNow()
	fileID2 := idwrap.NewNow()
	nodeID1 := idwrap.NewNow()

	tests := []struct {
		name        string
		result      *TranslationResult
		wantErrors  int
		errContains string
	}{
		{
			name:       "nil result",
			result:     nil,
			wantErrors: 0,
		},
		{
			name: "valid result",
			result: &TranslationResult{
				HTTPRequests: []mhttp.HTTP{
					{ID: httpID1},
				},
				Files: []mfile.File{
					{ID: fileID1, ContentType: mfile.ContentTypeHTTP, ContentID: &httpID1},
				},
			},
			wantErrors: 0,
		},
		{
			name: "invalid file content reference",
			result: &TranslationResult{
				HTTPRequests: []mhttp.HTTP{
					{ID: httpID1},
				},
				Files: []mfile.File{
					{ID: fileID1, ContentType: mfile.ContentTypeHTTP, ContentID: &httpID2},
				},
			},
			wantErrors:  1,
			errContains: "file references HTTP not in translation result",
		},
		{
			name: "invalid file parent reference",
			result: &TranslationResult{
				Files: []mfile.File{
					{ID: fileID1, ParentID: &fileID2},
				},
			},
			wantErrors:  1,
			errContains: "file references parent not in translation result",
		},
		{
			name: "invalid request node reference",
			result: &TranslationResult{
				HTTPRequests: []mhttp.HTTP{
					{ID: httpID1},
				},
				RequestNodes: []mflow.NodeRequest{
					{FlowNodeID: nodeID1, HttpID: &httpID2},
				},
			},
			wantErrors:  1,
			errContains: "request node references HTTP not in translation result",
		},
		{
			name: "valid folder and flow",
			result: &TranslationResult{
				Files: []mfile.File{
					{ID: fileID1, ContentType: mfile.ContentTypeFolder},
					{ID: fileID2, ContentType: mfile.ContentTypeFlow},
				},
			},
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := ValidateTranslationResult(tt.result)
			if len(report.Errors) != tt.wantErrors {
				t.Errorf("ValidateTranslationResult() got %d errors, want %d", len(report.Errors), tt.wantErrors)
			}
			if tt.wantErrors > 0 && tt.errContains != "" {
				found := false
				for _, err := range report.Errors {
					if strings.Contains(err.Message, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateTranslationResult() errors did not contain %q", tt.errContains)
				}
			}
		})
	}
}