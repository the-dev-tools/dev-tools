package topencollection

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// convertBody converts an OpenCollection body into DevTools body models.
func convertBody(body *OCBody, httpID idwrap.IDWrap) (
	bodyKind mhttp.HttpBodyKind,
	bodyRaw *mhttp.HTTPBodyRaw,
	bodyForms []mhttp.HTTPBodyForm,
	bodyUrlencoded []mhttp.HTTPBodyUrlencoded,
) {
	if body == nil || body.Type == "none" || body.Type == "" {
		return mhttp.HttpBodyKindNone, nil, nil, nil
	}

	switch body.Type {
	case "json", "xml", "text":
		rawData := extractRawData(body.Data)
		return mhttp.HttpBodyKindRaw, &mhttp.HTTPBodyRaw{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			RawData: []byte(rawData),
		}, nil, nil

	case "form-urlencoded":
		fields := extractFormFields(body.Data)
		var items []mhttp.HTTPBodyUrlencoded
		for i, f := range fields {
			items = append(items, mhttp.HTTPBodyUrlencoded{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          f.Name,
				Value:        f.Value,
				Enabled:      !f.Disabled,
				DisplayOrder: float32(i + 1),
			})
		}
		return mhttp.HttpBodyKindUrlEncoded, nil, nil, items

	case "multipart-form":
		fields := extractFormFields(body.Data)
		var items []mhttp.HTTPBodyForm
		for i, f := range fields {
			items = append(items, mhttp.HTTPBodyForm{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          f.Name,
				Value:        f.Value,
				Description:  f.ContentType,
				Enabled:      !f.Disabled,
				DisplayOrder: float32(i + 1),
			})
		}
		return mhttp.HttpBodyKindFormData, nil, items, nil

	default:
		// Unknown body type — treat as raw
		rawData := extractRawData(body.Data)
		return mhttp.HttpBodyKindRaw, &mhttp.HTTPBodyRaw{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			RawData: []byte(rawData),
		}, nil, nil
	}
}

// extractRawData converts the body data field to a string.
func extractRawData(data interface{}) string {
	if data == nil {
		return ""
	}

	switch v := data.(type) {
	case string:
		return v
	case map[string]interface{}:
		// JSON object — marshal it
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractFormFields extracts form fields from the body data.
// Handles both []OCFormField (structured) and []interface{} (YAML decoded).
func extractFormFields(data interface{}) []OCFormField {
	if data == nil {
		return nil
	}

	// If it's already []OCFormField (unlikely from YAML), return as is
	if fields, ok := data.([]OCFormField); ok {
		return fields
	}

	// YAML decodes arrays as []interface{}
	rawList, ok := data.([]interface{})
	if !ok {
		return nil
	}

	var fields []OCFormField
	for _, item := range rawList {
		m, ok := item.(map[string]interface{})
		if !ok {
			// Try via YAML re-marshal
			b, err := yaml.Marshal(item)
			if err != nil {
				continue
			}
			var f OCFormField
			if err := yaml.Unmarshal(b, &f); err != nil {
				continue
			}
			fields = append(fields, f)
			continue
		}

		f := OCFormField{
			Name:  stringFromMap(m, "name"),
			Value: stringFromMap(m, "value"),
		}
		if v, ok := m["disabled"].(bool); ok {
			f.Disabled = v
		}
		if v, ok := m["contentType"].(string); ok {
			f.ContentType = v
		}
		fields = append(fields, f)
	}

	return fields
}

func stringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}
