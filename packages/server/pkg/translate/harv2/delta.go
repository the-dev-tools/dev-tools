package harv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

// createDeltaVersion creates a delta version of an HTTP request
func createDeltaVersion(original mhttp.HTTP) mhttp.HTTP {
	deltaName := original.Name + " (Delta)"
	deltaURL := original.Url
	deltaMethod := original.Method
	deltaDesc := original.Description + " [Delta Version]"

	delta := mhttp.HTTP{
		ID:               idwrap.NewNow(),
		WorkspaceID:      original.WorkspaceID,
		ParentHttpID:     &original.ID,
		Name:             deltaName,
		Url:              original.Url,
		Method:           original.Method,
		Description:      deltaDesc,
		IsDelta:          true,
		DeltaName:        &deltaName,
		DeltaUrl:         &deltaURL,
		DeltaMethod:      &deltaMethod,
		DeltaDescription: &deltaDesc,
		CreatedAt:        original.CreatedAt + 1, // Ensure slightly later timestamp
		UpdatedAt:        original.UpdatedAt + 1,
	}

	return delta
}
