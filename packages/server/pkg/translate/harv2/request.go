package harv2

import (
	"fmt"
	"net/url"
	"strings"

	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

// createHTTPRequestFromEntryWithDeps creates HTTP entity and checks for data dependencies
func createHTTPRequestFromEntryWithDeps(entry Entry, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder) (
	*mhttp.HTTP,
	[]mhttp.HTTPHeader,
	[]mhttp.HTTPSearchParam,
	[]mhttp.HTTPBodyForm,
	[]mhttp.HTTPBodyUrlencoded,
	[]mhttp.HTTPBodyRaw,
	[]depfinder.VarCouple,
	error,
) {
	// Use the original function logic but inject dependency checks
	// Since we can't easily call the original function and then modify, we duplicate the logic here
	// but integrated with DepFinder.

	var allCouples []depfinder.VarCouple

	parsedURL, err := url.Parse(entry.Request.URL)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to parse URL %s: %w", entry.Request.URL, err)
	}

	// Determine body kind
	bodyKind := mhttp.HttpBodyKindNone
	if entry.Request.PostData != nil {
		mimeType := strings.ToLower(entry.Request.PostData.MimeType)
		if strings.Contains(mimeType, FormBodyCheck) {
			bodyKind = mhttp.HttpBodyKindFormData
		} else if strings.Contains(mimeType, UrlEncodedBodyCheck) {
			bodyKind = mhttp.HttpBodyKindUrlEncoded
		} else {
			bodyKind = mhttp.HttpBodyKindRaw
		}
	}

	name := generateRequestName(entry.Request.Method, parsedURL)
	now := entry.StartedDateTime.UnixMilli()
	httpID := idwrap.NewNow()

	httpReq := &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        name,
		Url:         entry.Request.URL,
		Method:      entry.Request.Method,
		Description: fmt.Sprintf("Imported from HAR - %s %s", entry.Request.Method, entry.Request.URL),
		BodyKind:    bodyKind,
		IsDelta:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if depFinder != nil {
		// Check URL Path Params for dependencies
		newURL, found, couples := depFinder.ReplaceURLPathParams(parsedURL.String())
		if found {
			httpReq.Url = newURL
			allCouples = append(allCouples, couples...)
		}
	}

	// Check URL for full replacements (query params are handled separately)
	// (Simplification: we won't modify the base URL string for path params here to avoid breaking
	// valid URLs unless we are sure. The old thar did simple string replacement.)

	// Extract headers
	headers := make([]mhttp.HTTPHeader, 0, len(entry.Request.Headers))
	for _, h := range entry.Request.Headers {
		if strings.HasPrefix(h.Name, ":") {
			continue
		}

		// Dependency Check
		val := h.Value
		if depFinder != nil {
			// Use ReplaceWithPathsSubstring for headers (often tokens are substrings like "Bearer ...")
			if newVal, found, couples := depFinder.ReplaceWithPathsSubstring(val); found {
				if strVal, ok := newVal.(string); ok {
					val = strVal
					allCouples = append(allCouples, couples...)
				}
			}
		}

		headers = append(headers, mhttp.HTTPHeader{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:   h.Name,
			Value: val,
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// Extract Query Parameters
	params := make([]mhttp.HTTPSearchParam, 0, len(entry.Request.QueryString))
	for _, q := range entry.Request.QueryString {
		val := q.Value
		if depFinder != nil {
			if newVal, found, couples := depFinder.ReplaceWithPaths(val); found {
				if strVal, ok := newVal.(string); ok {
					val = strVal
					allCouples = append(allCouples, couples...)
				}
			}
		}
		params = append(params, mhttp.HTTPSearchParam{
			ID:         idwrap.NewNow(),
			HttpID:     httpID,
			ParamKey:   q.Name,
			ParamValue: val,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// Extract Body
	var bodyForms []mhttp.HTTPBodyForm
	var bodyUrlEncoded []mhttp.HTTPBodyUrlencoded
	var bodyRaws []mhttp.HTTPBodyRaw

	if entry.Request.PostData != nil {
		if bodyKind == mhttp.HttpBodyKindFormData {
			for _, p := range entry.Request.PostData.Params {
				val := p.Value
				if depFinder != nil {
					if newVal, found, couples := depFinder.ReplaceWithPaths(val); found {
						if strVal, ok := newVal.(string); ok {
							val = strVal
							allCouples = append(allCouples, couples...)
						}
					}
				}
				bodyForms = append(bodyForms, mhttp.HTTPBodyForm{
					ID:        idwrap.NewNow(),
					HttpID:    httpID,
					FormKey:   p.Name,
					FormValue: val,
					Enabled:   true,
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
		} else if bodyKind == mhttp.HttpBodyKindUrlEncoded {
			for _, p := range entry.Request.PostData.Params {
				val := p.Value
				if depFinder != nil {
					if newVal, found, couples := depFinder.ReplaceWithPaths(val); found {
						if strVal, ok := newVal.(string); ok {
							val = strVal
							allCouples = append(allCouples, couples...)
						}
					}
				}
				bodyUrlEncoded = append(bodyUrlEncoded, mhttp.HTTPBodyUrlencoded{
					ID:              idwrap.NewNow(),
					HttpID:          httpID,
					UrlencodedKey:   p.Name,
					UrlencodedValue: val,
					Enabled:         true,
					CreatedAt:       now,
					UpdatedAt:       now,
				})
			}
		} else if bodyKind == mhttp.HttpBodyKindRaw {
			text := entry.Request.PostData.Text
			// Template JSON body
			if depFinder != nil && strings.Contains(strings.ToLower(entry.Request.PostData.MimeType), "json") {
				res := depFinder.TemplateJSON([]byte(text))
				if res.Err == nil {
					text = string(res.NewJson)
					allCouples = append(allCouples, res.Couples...)
				}
			}

			bodyRaws = append(bodyRaws, mhttp.HTTPBodyRaw{
				ID:              idwrap.NewNow(),
				HttpID:          httpID,
				RawData:         []byte(text),
				ContentType:     entry.Request.PostData.MimeType,
				CompressionType: 0, // Default to no compression
				CreatedAt:       now,
				UpdatedAt:       now,
			})
		}
	}

	return httpReq, headers, params, bodyForms, bodyUrlEncoded, bodyRaws, allCouples, nil
}

// generateRequestName creates a descriptive name from HTTP method and URL
func generateRequestName(method string, parsedURL *url.URL) string {
	// Extract meaningful path segments
	pathSegments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")

	// Take last 2-3 meaningful segments
	var meaningfulSegments []string
	for i := len(pathSegments) - 1; i >= 0 && len(meaningfulSegments) < 3; i-- {
		segment := pathSegments[i]
		if segment != "" && !strings.HasPrefix(segment, "{") && !isNumericSegment(segment) {
			meaningfulSegments = append([]string{segment}, meaningfulSegments...)
		}
	}

	// Include hostname if no meaningful path segments
	if len(meaningfulSegments) == 0 {
		host := strings.Replace(parsedURL.Hostname(), "www.", "", 1)
		host = strings.Replace(host, ".", " ", -1)
		return fmt.Sprintf("%s %s", method, strings.Title(host))
	}

	// Build final name
	pathName := strings.Join(meaningfulSegments, " ")
	return fmt.Sprintf("%s %s", method, strings.Title(strings.Replace(pathName, "-", " ", -1)))
}

// isNumericSegment checks if a URL segment is purely numeric (likely an ID)
func isNumericSegment(segment string) bool {
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(segment) > 0
}
