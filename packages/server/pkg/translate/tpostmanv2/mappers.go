//nolint:revive // exported
package tpostmanv2

import (
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

// convertPostmanURLToSearchParams converts a Postman URL to base URL and search parameters
func convertPostmanURLToSearchParams(postmanURL PostmanURL, httpID idwrap.IDWrap) (string, []mhttp.HTTPSearchParam) {
	// Build the raw URL first
	rawURL := postmanURL.Raw
	if rawURL == "" {
		// Build raw URL from components if not provided
		var urlBuilder strings.Builder

		if postmanURL.Protocol != "" {
			urlBuilder.WriteString(postmanURL.Protocol)
			urlBuilder.WriteString("://")
		}

		if len(postmanURL.Host) > 0 {
			urlBuilder.WriteString(strings.Join(postmanURL.Host, "."))
		}

		if postmanURL.Port != "" {
			urlBuilder.WriteString(":")
			urlBuilder.WriteString(postmanURL.Port)
		}

		if len(postmanURL.Path) > 0 {
			urlBuilder.WriteString("/")
			urlBuilder.WriteString(strings.Join(postmanURL.Path, "/"))
		}

		rawURL = urlBuilder.String()
	}

	// Parse URL to extract existing query parameters
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, return raw URL without additional params
		return rawURL, nil
	}

	var searchParams []mhttp.HTTPSearchParam
	now := time.Now().UnixMilli()

	// First, add parameters from Postman's query array (only enabled ones)
	for _, param := range postmanURL.Query {
		if param.Disabled {
			continue // Skip disabled parameters
		}
		searchParam := mhttp.HTTPSearchParam{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         param.Key,
			Value:       param.Value,
			Description: param.Description,
			Enabled:     true, // All included parameters are enabled
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		searchParams = append(searchParams, searchParam)
	}

	// If Postman query array is provided, use it as the authoritative source
	// and ignore query parameters from the raw URL
	if len(postmanURL.Query) > 0 {
		// Parse URL to remove any existing query string
		if parsedURL != nil {
			parsedURL.RawQuery = ""
			rawURL = parsedURL.String()
		}
	} else {
		// If no Postman query array, extract parameters from raw URL
		if parsedURL != nil {
			rawQueryParams := parsedURL.Query()
			for key, values := range rawQueryParams {
				for _, value := range values {
					searchParam := mhttp.HTTPSearchParam{
						ID:          idwrap.NewNow(),
						HttpID:      httpID,
						Key:         key,
						Value:       value,
						Description: "",
						Enabled:     true,
						CreatedAt:   now,
						UpdatedAt:   now,
					}
					searchParams = append(searchParams, searchParam)
				}
			}

			// Return the base URL without query string
			parsedURL.RawQuery = ""
			rawURL = parsedURL.String()
		}
	}

	return rawURL, searchParams
}

// convertPostmanHeadersToHTTPHeaders converts Postman headers and auth to HTTP header models
func convertPostmanHeadersToHTTPHeaders(postmanHeaders []PostmanHeader, postmanAuth *PostmanAuth, httpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var headers []mhttp.HTTPHeader
	now := time.Now().UnixMilli()

	// Add regular headers first
	for _, header := range postmanHeaders {
		if header.Key == "" || header.Disabled {
			continue // Skip headers without keys or disabled headers
		}

		httpHeader := mhttp.HTTPHeader{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         header.Key,
			Value:       header.Value,
			Description: header.Description,
			Enabled:     true, // All included headers are enabled
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		headers = append(headers, httpHeader)
	}

	// Add authentication headers
	authHeaders := convertAuthToHeaders(postmanAuth, httpID)
	headers = append(headers, authHeaders...)

	return headers
}

// convertAuthToHeaders converts Postman authentication to HTTP headers
func convertAuthToHeaders(auth *PostmanAuth, httpID idwrap.IDWrap) []mhttp.HTTPHeader {
	if auth == nil {
		return nil
	}

	now := time.Now().UnixMilli()

	switch auth.Type {
	case "apikey":
		if len(auth.APIKey) > 0 {
			var key, value string
			for _, param := range auth.APIKey {
				switch param.Key {
				case "key":
					key = param.Value
				case "value":
					value = param.Value
				}
			}
			if key != "" && value != "" {
				return []mhttp.HTTPHeader{
					{
						ID:          idwrap.NewNow(),
						HttpID:      httpID,
						Key:         key,
						Value:       value,
						Description: "API Key authentication",
						Enabled:     true,
						CreatedAt:   now,
						UpdatedAt:   now,
					},
				}
			}
		}

	case "basic":
		if len(auth.Basic) > 0 {
			var username, password string
			for _, param := range auth.Basic {
				switch param.Key {
				case "username":
					username = param.Value
				case "password":
					password = param.Value
				}
			}
			if username != "" {
				// Convert to Base64
				credentials := username + ":" + password
				encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
				return []mhttp.HTTPHeader{
					{
						ID:          idwrap.NewNow(),
						HttpID:      httpID,
						Key:         "Authorization",
						Value:       "Basic " + encoded,
						Description: "Basic authentication",
						Enabled:     true,
						CreatedAt:   now,
						UpdatedAt:   now,
					},
				}
			}
		}

	case "bearer":
		if len(auth.Bearer) > 0 {
			var token string
			for _, param := range auth.Bearer {
				if param.Key == "token" {
					token = param.Value
					break
				}
			}
			if token != "" {
				return []mhttp.HTTPHeader{
					{
						ID:          idwrap.NewNow(),
						HttpID:      httpID,
						Key:         "Authorization",
						Value:       "Bearer " + token,
						Description: "Bearer token authentication",
						Enabled:     true,
						CreatedAt:   now,
						UpdatedAt:   now,
					},
				}
			}
		}
	}

	return nil
}

// convertPostmanBodyToHTTPModels converts Postman body to the various HTTP body model types
func convertPostmanBodyToHTTPModels(postmanBody *PostmanBody, httpID idwrap.IDWrap) (*mhttp.HTTPBodyRaw, []mhttp.HTTPBodyForm, []mhttp.HTTPBodyUrlencoded) {
	if postmanBody == nil {
		return nil, nil, nil
	}

	now := time.Now().UnixMilli()

	switch postmanBody.Mode {
	case "raw":
		bodyRaw := &mhttp.HTTPBodyRaw{
			ID:              idwrap.NewNow(),
			HttpID:          httpID,
			RawData:         []byte(postmanBody.Raw),
			CompressionType: compress.CompressTypeNone,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		// Apply compression if beneficial
		if len(postmanBody.Raw) > 1024 { // Threshold for compression
			compressed, err := compress.Compress([]byte(postmanBody.Raw), compress.CompressTypeZstd)
			if err == nil && len(compressed) < len(postmanBody.Raw) {
				bodyRaw.RawData = compressed
				bodyRaw.CompressionType = compress.CompressTypeZstd
			}
		}

		return bodyRaw, nil, nil

	case "formdata":
		var bodyForms []mhttp.HTTPBodyForm
		for _, formData := range postmanBody.FormData {
			if formData.Key == "" || formData.Disabled {
				continue // Skip empty keys or disabled form fields
			}

			bodyForm := mhttp.HTTPBodyForm{
				ID:          idwrap.NewNow(),
				HttpID:      httpID,
				Key:         formData.Key,
				Value:       formData.Value,
				Description: formData.Description,
				Enabled:     true, // All included form fields are enabled
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			bodyForms = append(bodyForms, bodyForm)
		}
		return nil, bodyForms, nil

	case "urlencoded":
		var bodyUrlencoded []mhttp.HTTPBodyUrlencoded
		for _, urlEncoded := range postmanBody.URLEncoded {
			if urlEncoded.Key == "" || urlEncoded.Disabled {
				continue // Skip empty keys or disabled URL encoded fields
			}

			bodyUrl := mhttp.HTTPBodyUrlencoded{
				ID:          idwrap.NewNow(),
				HttpID:      httpID,
				Key:         urlEncoded.Key,
				Value:       urlEncoded.Value,
				Description: urlEncoded.Description,
				Enabled:     true, // All included URL encoded fields are enabled
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			bodyUrlencoded = append(bodyUrlencoded, bodyUrl)
		}
		return nil, nil, bodyUrlencoded

	default:
		// For unknown body modes, treat as raw
		bodyRaw := &mhttp.HTTPBodyRaw{
			ID:              idwrap.NewNow(),
			HttpID:          httpID,
			RawData:         []byte{},
			CompressionType: compress.CompressTypeNone,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		return bodyRaw, nil, nil
	}
}

// buildPostmanURL builds a Postman URL from base URL and search parameters
func buildPostmanURL(baseURL string, searchParams []PostmanQueryParam) PostmanURL {
	// Parse the base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		// If parsing fails, return a simple PostmanURL structure
		return PostmanURL{
			Raw: baseURL,
		}
	}

	// Build the Postman URL structure
	postmanURL := PostmanURL{
		Raw:   baseURL,
		Query: searchParams,
	}

	// Fill in detailed URL components if available
	if parsedURL != nil {
		postmanURL.Protocol = parsedURL.Scheme
		postmanURL.Host = []string{parsedURL.Hostname()}
		postmanURL.Port = parsedURL.Port()
		if parsedURL.Port() != "" {
			postmanURL.Port = parsedURL.Port()
		}

		if parsedURL.Path != "" && parsedURL.Path != "/" {
			postmanURL.Path = strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
		}

		postmanURL.Hash = parsedURL.Fragment
	}

	return postmanURL
}

// extractSearchParamsForHTTP finds search parameters associated with a specific HTTP request
func extractSearchParamsForHTTP(httpID idwrap.IDWrap, searchParams []mhttp.HTTPSearchParam) []PostmanQueryParam {
	var postmanQuery []PostmanQueryParam

	for _, param := range searchParams {
		if param.HttpID.Compare(httpID) == 0 && param.Enabled {
			postmanQuery = append(postmanQuery, PostmanQueryParam{
				Key:         param.Key,
				Value:       param.Value,
				Description: param.Description,
				Disabled:    false,
			})
		}
	}

	return postmanQuery
}

// extractHeadersForHTTP finds headers associated with a specific HTTP request
func extractHeadersForHTTP(httpID idwrap.IDWrap, headers []mhttp.HTTPHeader) []PostmanHeader {
	var postmanHeaders []PostmanHeader

	for _, header := range headers {
		if header.HttpID.Compare(httpID) == 0 && header.Enabled {
			postmanHeaders = append(postmanHeaders, PostmanHeader{
				Key:         header.Key,
				Value:       header.Value,
				Description: header.Description,
				Disabled:    false,
			})
		}
	}

	return postmanHeaders
}

// extractBodyRawForHTTP finds the raw body associated with a specific HTTP request
func extractBodyRawForHTTP(httpID idwrap.IDWrap, bodyRaws []*mhttp.HTTPBodyRaw) *mhttp.HTTPBodyRaw {
	for _, bodyRaw := range bodyRaws {
		if bodyRaw.HttpID.Compare(httpID) == 0 {
			return bodyRaw
		}
	}
	return nil
}
