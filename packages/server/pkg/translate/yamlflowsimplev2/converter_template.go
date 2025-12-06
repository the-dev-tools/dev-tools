package yamlflowsimplev2

import (
	"encoding/json"
	"strings"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

func mergeHTTPRequestDataStruct(base, override YamlRequestDefV2, usingTemplate bool) YamlRequestDefV2 {
	if !usingTemplate {
		return override
	}
	merged := base
	if override.Method != "" {
		merged.Method = override.Method
	}
	if override.URL != "" {
		merged.URL = override.URL
	}
	if override.Description != "" {
		merged.Description = override.Description
	}
	if override.Body != nil {
		merged.Body = override.Body
	}

	if len(override.Headers) > 0 {
		merged.Headers = append(merged.Headers, override.Headers...)
	}
	if len(override.QueryParams) > 0 {
		merged.QueryParams = append(merged.QueryParams, override.QueryParams...)
	}
	if len(override.Assertions) > 0 {
		merged.Assertions = append(merged.Assertions, override.Assertions...)
	}
	return merged
}

func convertToHTTPHeaders(yamlHeaders []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var headers []mhttp.HTTPHeader
	for _, h := range yamlHeaders {
		headers = append(headers, mhttp.HTTPHeader{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Key:     h.Name,
			Value:   h.Value,
			Enabled: h.Enabled,
		})
	}
	return headers
}

func convertToHTTPSearchParams(yamlParams []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPSearchParam {
	var params []mhttp.HTTPSearchParam
	for _, p := range yamlParams {
		params = append(params, mhttp.HTTPSearchParam{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Key:     p.Name,
			Value:   p.Value,
			Enabled: p.Enabled,
		})
	}
	return params
}

func convertBodyStruct(body *YamlBodyUnion, httpID idwrap.IDWrap, opts ConvertOptionsV2) (mhttp.HTTPBodyRaw, []mhttp.HTTPBodyForm, []mhttp.HTTPBodyUrlencoded, mhttp.HttpBodyKind) {
	bodyRaw := mhttp.HTTPBodyRaw{
		ID:     idwrap.NewNow(),
		HttpID: httpID,
	}
	var bodyForms []mhttp.HTTPBodyForm
	var bodyUrlencoded []mhttp.HTTPBodyUrlencoded
	bodyKind := mhttp.HttpBodyKindRaw

	if body == nil {
		return bodyRaw, nil, nil, bodyKind
	}

	switch strings.ToLower(body.Type) {
	case "form-data":
		bodyKind = mhttp.HttpBodyKindFormData
		for _, form := range body.Form {
			bodyForms = append(bodyForms, mhttp.HTTPBodyForm{
				ID:      idwrap.NewNow(),
				HttpID:  httpID,
				Key:     form.Name,
				Value:   form.Value,
				Enabled: form.Enabled,
			})
		}
	case "urlencoded":
		bodyKind = mhttp.HttpBodyKindUrlEncoded
		for _, urlEncoded := range body.UrlEncoded {
			bodyUrlencoded = append(bodyUrlencoded, mhttp.HTTPBodyUrlencoded{
				ID:      idwrap.NewNow(),
				HttpID:  httpID,
				Key:     urlEncoded.Name,
				Value:   urlEncoded.Value,
				Enabled: urlEncoded.Enabled,
			})
		}
	case "json":
		bodyKind = mhttp.HttpBodyKindRaw
		if body.JSON != nil {
			jb, _ := json.Marshal(body.JSON)
			bodyRaw.RawData = jb
			bodyRaw.ContentType = "application/json"
		}
	case "raw":
		bodyKind = mhttp.HttpBodyKindRaw
		bodyRaw.RawData = []byte(body.Raw)
	default:
		bodyKind = mhttp.HttpBodyKindRaw
		bodyRaw.RawData = []byte(body.Raw)
	}

	if body.Compression != "" {
		if ct, ok := compress.CompressLockupMap[body.Compression]; ok {
			bodyRaw.CompressionType = ct
		}
	} else if opts.EnableCompression && len(bodyRaw.RawData) > 1024 {
		// Auto-compress only if larger than threshold
		compressed, err := compress.Compress(bodyRaw.RawData, opts.CompressionType)
		if err == nil {
			bodyRaw.RawData = compressed
			bodyRaw.CompressionType = opts.CompressionType
		}
	}

	return bodyRaw, bodyForms, bodyUrlencoded, bodyKind
}
