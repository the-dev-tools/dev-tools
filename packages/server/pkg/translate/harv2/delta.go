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

// CreateDeltaHeaders creates delta headers when HAR headers differ from base request
func CreateDeltaHeaders(originalHeaders []mhttp.HTTPHeader, newHeaders []mhttp.HTTPHeader, deltaHttpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var deltaHeaders []mhttp.HTTPHeader

	// Create map of original headers by key for comparison
	originalMap := make(map[string]mhttp.HTTPHeader)
	for _, header := range originalHeaders {
		originalMap[header.Key] = header
	}

	// Find changed or new headers
	for _, newHeader := range newHeaders {
		original, exists := originalMap[newHeader.Key]

		// Create delta if header doesn't exist or has different value
		if !exists || original.Value != newHeader.Value {
			deltaKey := newHeader.Key
			deltaValue := newHeader.Value
			deltaDesc := "Imported from HAR"
			deltaEnabled := true

			// Prepare parent header ID - use zero value if no original
			var parentHeaderID *idwrap.IDWrap
			if exists {
				parentHeaderID = &original.ID
			}

			deltaHeader := mhttp.HTTPHeader{
				ID:               idwrap.NewNow(),
				HttpID:           deltaHttpID,
				Key:        deltaKey,
				Value:      deltaValue,
				Description:      deltaDesc,
				Enabled:          true, // Delta headers are always enabled
				ParentHttpHeaderID:   parentHeaderID,
				IsDelta:          true,
				DeltaKey:   &deltaKey,
				DeltaValue: &deltaValue,
				DeltaDescription: &deltaDesc,
				DeltaEnabled:     &deltaEnabled,
				CreatedAt:        newHeader.CreatedAt + 1,
				UpdatedAt:        newHeader.UpdatedAt + 1,
			}
			deltaHeaders = append(deltaHeaders, deltaHeader)
		}
	}

	return deltaHeaders
}

// CreateDeltaSearchParams creates delta search params when HAR params differ from base request
func CreateDeltaSearchParams(originalParams []mhttp.HTTPSearchParam, newParams []mhttp.HTTPSearchParam, deltaHttpID idwrap.IDWrap) []mhttp.HTTPSearchParam {
	var deltaParams []mhttp.HTTPSearchParam

	// Create map of original params by key for comparison
	originalMap := make(map[string]mhttp.HTTPSearchParam)
	for _, param := range originalParams {
		originalMap[param.Key] = param
	}

	// Find changed or new params
	for _, newParam := range newParams {
		original, exists := originalMap[newParam.Key]

		// Create delta if param doesn't exist or has different value
		if !exists || original.Value != newParam.Value {
			deltaKey := newParam.Key
			deltaValue := newParam.Value
			deltaDesc := "Imported from HAR"
			deltaEnabled := true

			// Prepare parent param ID - use zero value if no original
			var parentSearchParamID *idwrap.IDWrap
			if exists {
				parentSearchParamID = &original.ID
			}

			deltaParam := mhttp.HTTPSearchParam{
				ID:              idwrap.NewNow(),
				HttpID:          deltaHttpID,
				Key:        deltaKey,
				Value:      deltaValue,
				Description:     deltaDesc,
				Enabled:         true,
				ParentHttpSearchParamID: parentSearchParamID,
				IsDelta:         true,
				DeltaKey:   &deltaKey,
				DeltaValue: &deltaValue,
				DeltaDescription: &deltaDesc,
				DeltaEnabled:    &deltaEnabled,
				CreatedAt:       newParam.CreatedAt + 1,
				UpdatedAt:       newParam.UpdatedAt + 1,
			}
			deltaParams = append(deltaParams, deltaParam)
		}
	}

	return deltaParams
}

// CreateDeltaBodyForms creates delta body forms when HAR forms differ from base request
func CreateDeltaBodyForms(originalForms []mhttp.HTTPBodyForm, newForms []mhttp.HTTPBodyForm, deltaHttpID idwrap.IDWrap) []mhttp.HTTPBodyForm {
	var deltaForms []mhttp.HTTPBodyForm

	// Create map of original forms by key for comparison
	originalMap := make(map[string]mhttp.HTTPBodyForm)
	for _, form := range originalForms {
		originalMap[form.FormKey] = form
	}

	// Find changed or new forms
	for _, newForm := range newForms {
		original, exists := originalMap[newForm.FormKey]

		// Create delta if form doesn't exist or has different value
		if !exists || original.FormValue != newForm.FormValue {
			deltaKey := newForm.FormKey
			deltaValue := newForm.FormValue
			deltaDesc := "Imported from HAR"
			deltaEnabled := true

			// Prepare parent form ID - use zero value if no original
			var parentBodyFormID *idwrap.IDWrap
			if exists {
				parentBodyFormID = &original.ID
			}

			deltaForm := mhttp.HTTPBodyForm{
				ID:               idwrap.NewNow(),
				HttpID:           deltaHttpID,
				FormKey:          deltaKey,
				FormValue:        deltaValue,
				Description:      deltaDesc,
				Enabled:          true,
				ParentBodyFormID: parentBodyFormID,
				IsDelta:          true,
				DeltaFormKey:     &deltaKey,
				DeltaFormValue:   &deltaValue,
				DeltaDescription: &deltaDesc,
				DeltaEnabled:     &deltaEnabled,
				CreatedAt:        newForm.CreatedAt + 1,
				UpdatedAt:        newForm.UpdatedAt + 1,
			}
			deltaForms = append(deltaForms, deltaForm)
		}
	}

	return deltaForms
}

// CreateDeltaBodyUrlEncoded creates delta URL-encoded body when HAR differs from base request
func CreateDeltaBodyUrlEncoded(originalEncoded []mhttp.HTTPBodyUrlencoded, newEncoded []mhttp.HTTPBodyUrlencoded, deltaHttpID idwrap.IDWrap) []mhttp.HTTPBodyUrlencoded {
	var deltaEncoded []mhttp.HTTPBodyUrlencoded

	// Create map of original encoded params by key for comparison
	originalMap := make(map[string]mhttp.HTTPBodyUrlencoded)
	for _, encoded := range originalEncoded {
		originalMap[encoded.UrlencodedKey] = encoded
	}

	// Find changed or new encoded params
	for _, newEncoded := range newEncoded {
		original, exists := originalMap[newEncoded.UrlencodedKey]

		// Create delta if param doesn't exist or has different value
		if !exists || original.UrlencodedValue != newEncoded.UrlencodedValue {
			deltaKey := newEncoded.UrlencodedKey
			deltaValue := newEncoded.UrlencodedValue
			deltaDesc := "Imported from HAR"
			deltaEnabled := true

			// Prepare parent encoded param ID - use zero value if no original
			var parentBodyUrlencodedID *idwrap.IDWrap
			if exists {
				parentBodyUrlencodedID = &original.ID
			}

			deltaEncodedParam := mhttp.HTTPBodyUrlencoded{
				ID:                     idwrap.NewNow(),
				HttpID:                 deltaHttpID,
				UrlencodedKey:          deltaKey,
				UrlencodedValue:        deltaValue,
				Description:            deltaDesc,
				Enabled:                true,
				ParentBodyUrlencodedID: parentBodyUrlencodedID,
				IsDelta:                true,
				DeltaUrlencodedKey:     &deltaKey,
				DeltaUrlencodedValue:   &deltaValue,
				DeltaDescription:       &deltaDesc,
				DeltaEnabled:           &deltaEnabled,
				CreatedAt:              newEncoded.CreatedAt + 1,
				UpdatedAt:              newEncoded.UpdatedAt + 1,
			}
			deltaEncoded = append(deltaEncoded, deltaEncodedParam)
		}
	}

	return deltaEncoded
}

// CreateDeltaBodyRaw creates delta raw body when HAR differs from base request
func CreateDeltaBodyRaw(originalRaw *mhttp.HTTPBodyRaw, newRaw *mhttp.HTTPBodyRaw, deltaHttpID idwrap.IDWrap) *mhttp.HTTPBodyRaw {
	// If no new raw data, no delta needed
	if newRaw == nil {
		return nil
	}

	// If no original, create new raw body instead of delta
	if originalRaw == nil {
		return &mhttp.HTTPBodyRaw{
			ID:                   idwrap.NewNow(),
			HttpID:               deltaHttpID,
			RawData:              newRaw.RawData,
			ContentType:          newRaw.ContentType,
			CompressionType:      newRaw.CompressionType,
			ParentBodyRawID:      nil,
			IsDelta:              false,
			DeltaRawData:         nil,
			DeltaContentType:     nil,
			DeltaCompressionType: nil,
			CreatedAt:            newRaw.CreatedAt,
			UpdatedAt:            newRaw.UpdatedAt,
		}
	}

	// Compare raw data and content type
	if string(originalRaw.RawData) == string(newRaw.RawData) &&
		originalRaw.ContentType == newRaw.ContentType &&
		originalRaw.CompressionType == newRaw.CompressionType {
		return nil
	}

	deltaRawData := newRaw.RawData
	deltaContentType := newRaw.ContentType
	deltaCompressionType := newRaw.CompressionType

	deltaRaw := &mhttp.HTTPBodyRaw{
		ID:                   idwrap.NewNow(),
		HttpID:               deltaHttpID,
		RawData:              newRaw.RawData,
		ContentType:          newRaw.ContentType,
		CompressionType:      newRaw.CompressionType,
		ParentBodyRawID:      &originalRaw.ID,
		IsDelta:              true,
		DeltaRawData:         deltaRawData,
		DeltaContentType:     &deltaContentType,
		DeltaCompressionType: &deltaCompressionType,
		CreatedAt:            newRaw.CreatedAt + 1,
		UpdatedAt:            newRaw.UpdatedAt + 1,
	}

	return deltaRaw
}
