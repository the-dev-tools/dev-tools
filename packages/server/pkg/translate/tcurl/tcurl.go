package tcurl

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
)

type CurlResolved struct {
	Apis             []mitemapi.ItemApi
	Examples         []mitemapiexample.ItemApiExample
	Queries          []mexamplequery.Query
	Headers          []mexampleheader.Header
	RawBodies        []mbodyraw.ExampleBodyRaw
	FormBodies       []mbodyform.BodyForm
	UrlEncodedBodies []mbodyurl.BodyURLEncoded
}

// Regular expressions for parsing curl commands
// Regular expressions for parsing curl commands
var (
	// URL pattern matches URLs in curl commands
	urlPattern = regexp.MustCompile(`(?:https?://|www\.)[^\s'"]+`)

	// Method pattern matches the -X or --request flag followed by the HTTP method
	methodPattern = regexp.MustCompile(`(?:-X|--request)\s+(?:'([A-Z]+)'|"([A-Z]+)"|([A-Z]+))`)

	// Header pattern matches -H or --header flags with their values
	headerPattern = regexp.MustCompile(`(?:-H|--header)\s+(?:'([^:]+):([^']+)'|"([^:]+):([^"]+)"|([^:]+):([^'"\s]+))`)

	// Cookie pattern matches -b or --cookie flags with their values
	cookiePattern = regexp.MustCompile(`(?:-b|--cookie)\s+(?:'([^']*)'|"([^"]*)"|([^\s'"][^\s]*))`)

	// Data patterns for different types of data
	dataPattern          = regexp.MustCompile(`(?:-d|--data|--data-raw|--data-binary)\s+(?:'([^']*)'|"([^"]*)"|([^\s'"][^\s]*))`)
	dataUrlEncodePattern = regexp.MustCompile(`--data-urlencode\s+(?:'([^=]+)=([^']*)'|"([^=]+)=([^"]*)"|([^=\s]+)=([^\s'"][^\s]*))`)
	formDataPattern      = regexp.MustCompile(`(?:-F|--form)\s+(?:'([^=]+)=([^']*)'|"([^=]+)=([^"]*)"|([^=\s]+)=([^\s'"][^\s]*))`)

	// Query parameter pattern to extract from URL
	queryParamPattern = regexp.MustCompile(`([^&=]+)=([^&]*)`)
)

func ConvertCurl(curlStr string, collectionID idwrap.IDWrap) (CurlResolved, error) {
	result := CurlResolved{
		Apis:             []mitemapi.ItemApi{},
		Examples:         []mitemapiexample.ItemApiExample{},
		Queries:          []mexamplequery.Query{},
		Headers:          []mexampleheader.Header{},
		RawBodies:        []mbodyraw.ExampleBodyRaw{},
		FormBodies:       []mbodyform.BodyForm{},
		UrlEncodedBodies: []mbodyurl.BodyURLEncoded{},
	}

	// Normalize the curl command to handle multi-line input
	normalizedCurl := normalizeCurlCommand(curlStr)

	// Validate that it's a curl command
	if !strings.HasPrefix(strings.TrimSpace(normalizedCurl), "curl") {
		return CurlResolved{}, fmt.Errorf("invalid curl command")
	}

	// Generate IDs for the new API and example
	exampleID := idwrap.NewNow()
	apiID := idwrap.NewNow()

	// Extract URL
	url := extractURL(normalizedCurl)
	if url == "" {
		return CurlResolved{}, fmt.Errorf("URL not found in curl command")
	}

	// Parse query parameters from URL
	baseURL, queries := parseURLAndQueries(url, exampleID)
	result.Queries = queries

	// Extract method
	method := extractMethod(normalizedCurl)

	// Extract headers
	headers := extractHeaders(normalizedCurl, exampleID)

	// Extract cookies and add them as headers
	cookieHeaders := extractCookies(normalizedCurl, exampleID)
	headers = append(headers, cookieHeaders...)

	result.Headers = headers

	// Extract data bodies
	hasDataFlag := false
	rawBodies := extractRawBodies(normalizedCurl, exampleID, &hasDataFlag)
	result.RawBodies = rawBodies

	// Extract URL-encoded bodies
	urlEncodedBodies := extractURLEncodedBodies(normalizedCurl, exampleID, &hasDataFlag)
	result.UrlEncodedBodies = urlEncodedBodies

	// Extract form bodies
	formBodies := extractFormBodies(normalizedCurl, exampleID, &hasDataFlag)
	result.FormBodies = formBodies

	// Create API item
	api := mitemapi.ItemApi{
		ID:           apiID,
		CollectionID: collectionID,
		Method:       method,
		Url:          baseURL,
		Name:         baseURL,
	}

	// If no explicit method was provided but we have data flags, assume POST
	if method == "GET" && hasDataFlag {
		api.Method = "POST"
	}

	result.Apis = append(result.Apis, api)

	// Create example and determine the BodyType based on what data is present
	bodyType := mitemapiexample.BodyTypeNone
	if len(result.RawBodies) > 0 {
		bodyType = mitemapiexample.BodyTypeRaw
	} else if len(result.FormBodies) > 0 {
		bodyType = mitemapiexample.BodyTypeForm
	} else if len(result.UrlEncodedBodies) > 0 {
		bodyType = mitemapiexample.BodyTypeUrlencoded
	}

	example := mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    apiID,
		Name:         baseURL,
		CollectionID: collectionID,
		IsDefault:    true,
		BodyType:     bodyType,
	}
	result.Examples = append(result.Examples, example)

	// Create empty raw body if there's no raw body but other body types exist
	// SQL depends on having raw body entries
	if len(result.RawBodies) == 0 && (len(result.FormBodies) > 0 || len(result.UrlEncodedBodies) > 0) {
		emptyRawBody := mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte{}, // Empty data
			CompressType:  compress.CompressTypeNone,
			VisualizeMode: mbodyraw.VisualizeModeText,
		}
		result.RawBodies = append(result.RawBodies, emptyRawBody)
	}

	return result, nil
}

// BuildCurl assembles a curl command string from the resolved structures that
// ConvertCurl produces. Only the first API/example pair is considered – callers
// should pass a CurlResolved that represents a single request.
func BuildCurl(resolved CurlResolved) (string, error) {
	if len(resolved.Apis) == 0 {
		return "", errors.New("tcurl: no APIs to build curl from")
	}

	api := resolved.Apis[0]
	example, ok := findExampleForAPI(api.ID, resolved.Examples)
	if !ok {
		return "", fmt.Errorf("tcurl: no example associated with api %s", api.ID.String())
	}

	method := strings.ToUpper(strings.TrimSpace(api.Method))
	fullURL := buildURLWithQueries(api.Url, resolved.Queries, example.ID)
	methodRequiresFlag := method != "" && method != "GET"

	headers := collectEnabledHeaders(resolved.Headers, example.ID)
	sortHeaders(headers)

	formBodies := collectEnabledFormBodies(resolved.FormBodies, example.ID)
	sortBodyForms(formBodies)

	urlBodies := collectEnabledURLEncodedBodies(resolved.UrlEncodedBodies, example.ID)
	sortBodyURLEncoded(urlBodies)

	rawBody, err := selectRawBody(resolved.RawBodies, example.ID)
	if err != nil {
		return "", err
	}

	args := []string{singleQuote(fullURL)}
	if methodRequiresFlag {
		args = append(args, "-X "+method)
	}

	for _, header := range headers {
		args = append(args, "-H "+singleQuote(fmt.Sprintf("%s: %s", header.HeaderKey, header.Value)))
	}

	if len(rawBody) > 0 {
		args = append(args, "--data-raw "+singleQuote(string(rawBody)))
	}

	for _, form := range formBodies {
		args = append(args, "-F "+singleQuote(fmt.Sprintf("%s=%s", form.BodyKey, form.Value)))
	}

	for _, urlBody := range urlBodies {
		args = append(args, "--data-urlencode "+singleQuote(fmt.Sprintf("%s=%s", urlBody.BodyKey, urlBody.Value)))
	}

	var builder strings.Builder
	builder.WriteString("curl ")
	builder.WriteString(args[0])
	for i := 1; i < len(args); i++ {
		builder.WriteString(" \\")
		builder.WriteString("\n  ")
		builder.WriteString(args[i])
	}

	return builder.String(), nil
}

func findExampleForAPI(apiID idwrap.IDWrap, examples []mitemapiexample.ItemApiExample) (*mitemapiexample.ItemApiExample, bool) {
	var fallback *mitemapiexample.ItemApiExample
	for i := range examples {
		example := &examples[i]
		if example.ItemApiID != apiID {
			continue
		}
		if example.IsDefault {
			return example, true
		}
		if fallback == nil {
			fallback = example
		}
	}
	if fallback != nil {
		return fallback, true
	}
	return nil, false
}

func buildURLWithQueries(baseURL string, queries []mexamplequery.Query, exampleID idwrap.IDWrap) string {
	values := url.Values{}
	for _, query := range queries {
		if query.ExampleID != exampleID {
			continue
		}
		if !query.IsEnabled() {
			continue
		}
		values.Add(query.QueryKey, query.Value)
	}

	encoded := values.Encode()
	if encoded == "" {
		return baseURL
	}

	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	return baseURL + separator + encoded
}

func collectEnabledHeaders(headers []mexampleheader.Header, exampleID idwrap.IDWrap) []mexampleheader.Header {
	result := make([]mexampleheader.Header, 0, len(headers))
	for _, header := range headers {
		if header.ExampleID != exampleID {
			continue
		}
		if !header.IsEnabled() {
			continue
		}
		result = append(result, header)
	}
	return result
}

func sortHeaders(headers []mexampleheader.Header) {
	sort.SliceStable(headers, func(i, j int) bool {
		ki := strings.ToLower(headers[i].HeaderKey)
		kj := strings.ToLower(headers[j].HeaderKey)
		if ki == kj {
			return headers[i].HeaderKey < headers[j].HeaderKey
		}
		return ki < kj
	})
}

func collectEnabledFormBodies(bodies []mbodyform.BodyForm, exampleID idwrap.IDWrap) []mbodyform.BodyForm {
	result := make([]mbodyform.BodyForm, 0, len(bodies))
	for _, body := range bodies {
		if body.ExampleID != exampleID {
			continue
		}
		if !body.IsEnabled() {
			continue
		}
		result = append(result, body)
	}
	return result
}

func sortBodyForms(bodies []mbodyform.BodyForm) {
	sort.SliceStable(bodies, func(i, j int) bool {
		if bodies[i].BodyKey == bodies[j].BodyKey {
			return bodies[i].Value < bodies[j].Value
		}
		return bodies[i].BodyKey < bodies[j].BodyKey
	})
}

func collectEnabledURLEncodedBodies(bodies []mbodyurl.BodyURLEncoded, exampleID idwrap.IDWrap) []mbodyurl.BodyURLEncoded {
	result := make([]mbodyurl.BodyURLEncoded, 0, len(bodies))
	for _, body := range bodies {
		if body.ExampleID != exampleID {
			continue
		}
		if !body.IsEnabled() {
			continue
		}
		result = append(result, body)
	}
	return result
}

func sortBodyURLEncoded(bodies []mbodyurl.BodyURLEncoded) {
	sort.SliceStable(bodies, func(i, j int) bool {
		if bodies[i].BodyKey == bodies[j].BodyKey {
			return bodies[i].Value < bodies[j].Value
		}
		return bodies[i].BodyKey < bodies[j].BodyKey
	})
}

func selectRawBody(bodies []mbodyraw.ExampleBodyRaw, exampleID idwrap.IDWrap) ([]byte, error) {
	for _, body := range bodies {
		if body.ExampleID != exampleID {
			continue
		}
		data := body.Data
		if body.CompressType != compress.CompressTypeNone && len(body.Data) > 0 {
			decompressed, err := compress.Decompress(body.Data, body.CompressType)
			if err != nil {
				return nil, fmt.Errorf("tcurl: decompress raw body: %w", err)
			}
			data = decompressed
		}
		return data, nil
	}
	return nil, nil
}

func singleQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// Normalize a curl command to handle both single-line and multi-line formats
func normalizeCurlCommand(curlStr string) string {
	// Handle line continuations (\ at end of line)
	curlStr = strings.ReplaceAll(curlStr, " \\\n", " ")
	curlStr = strings.ReplaceAll(curlStr, "\\\n", " ")

	// Remove newlines inside quoted strings
	var normalized strings.Builder
	inQuote := false
	quoteChar := rune(0)

	lines := strings.Split(curlStr, "\n")
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines
		if trimmedLine == "" {
			continue
		}

		// If this is a new curl command and we already have content, stop here
		if i > 0 && !inQuote && strings.HasPrefix(trimmedLine, "curl") && normalized.Len() > 0 {
			break
		}

		// Add space between lines if needed
		if normalized.Len() > 0 && !inQuote {
			normalized.WriteRune(' ')
		}

		// Process each character
		for _, char := range trimmedLine {
			if char == '\'' || char == '"' {
				if !inQuote {
					inQuote = true
					quoteChar = char
				} else if char == quoteChar {
					inQuote = false
				}
			}
			normalized.WriteRune(char)
		}
	}

	return normalized.String()
}

func extractURL(curlStr string) string {
	// Check for URLs in the curl command
	urls := urlPattern.FindAllString(curlStr, -1)
	if len(urls) > 0 {
		// Return the first URL found
		url := urls[0]
		// Remove any trailing quotes or spaces
		url = strings.TrimRight(url, "'\" ")
		return url
	}

	// If no URL was found using the regex, try to extract it after the curl command
	fields := strings.Fields(curlStr)
	for i, field := range fields {
		if i > 0 && field != "curl" && !strings.HasPrefix(field, "-") &&
			(fields[i-1] == "curl" || fields[i-1] == "-L") {
			// Remove quotes if present
			return removeQuotes(field)
		}
	}

	return ""
}

func extractMethod(curlStr string) string {
	matches := methodPattern.FindStringSubmatch(curlStr)
	if len(matches) >= 2 {
		// Check each capture group (single quotes, double quotes, or no quotes)
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" {
				return matches[i]
			}
		}
	}
	return "GET" // Default to GET if no method specified
}

func extractHeaders(curlStr string, exampleID idwrap.IDWrap) []mexampleheader.Header {
	var headers []mexampleheader.Header

	matches := headerPattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		// Check single quotes pattern (groups 1,2), double quotes pattern (groups 3,4), or no quotes pattern (groups 5,6)
		var key, value string
		if match[1] != "" {
			key, value = match[1], match[2] // Single quotes
		} else if match[3] != "" {
			key, value = match[3], match[4] // Double quotes
		} else {
			key, value = match[5], match[6] // No quotes
		}

		header := mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: strings.TrimSpace(key),
			Value:     strings.TrimSpace(value),
			Enable:    true,
		}
		headers = append(headers, header)
	}

	return headers
}

func extractRawBodies(curlStr string, exampleID idwrap.IDWrap, hasDataFlag *bool) []mbodyraw.ExampleBodyRaw {
	var bodies []mbodyraw.ExampleBodyRaw

	matches := dataPattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		*hasDataFlag = true

		// Check each capture group (single quotes, double quotes, or no quotes)
		var content string
		if match[1] != "" {
			content = match[1] // Single quotes
		} else if match[2] != "" {
			content = match[2] // Double quotes
		} else {
			content = match[3] // No quotes
		}

		body := mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte(content),
			CompressType:  compress.CompressTypeNone,
			VisualizeMode: mbodyraw.VisualizeModeText,
		}
		bodies = append(bodies, body)
	}

	return bodies
}

func extractURLEncodedBodies(curlStr string, exampleID idwrap.IDWrap, hasDataFlag *bool) []mbodyurl.BodyURLEncoded {
	var bodies []mbodyurl.BodyURLEncoded

	matches := dataUrlEncodePattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		*hasDataFlag = true

		// Check each capture group (single quotes, double quotes, or no quotes)
		var key, value string
		if match[1] != "" {
			key, value = match[1], match[2] // Single quotes
		} else if match[3] != "" {
			key, value = match[3], match[4] // Double quotes
		} else {
			key, value = match[5], match[6] // No quotes
		}

		body := mbodyurl.BodyURLEncoded{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			BodyKey:   key,
			Value:     value,
			Enable:    true,
		}
		bodies = append(bodies, body)
	}

	return bodies
}

func extractFormBodies(curlStr string, exampleID idwrap.IDWrap, hasDataFlag *bool) []mbodyform.BodyForm {
	var forms []mbodyform.BodyForm

	matches := formDataPattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		*hasDataFlag = true

		// Check each capture group (single quotes, double quotes, or no quotes)
		var key, value string
		if match[1] != "" {
			key, value = match[1], match[2] // Single quotes
		} else if match[3] != "" {
			key, value = match[3], match[4] // Double quotes
		} else {
			key, value = match[5], match[6] // No quotes
		}

		form := mbodyform.BodyForm{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			BodyKey:   key,
			Value:     value,
			Enable:    true,
		}
		forms = append(forms, form)
	}

	return forms
}

func parseURLAndQueries(urlStr string, exampleID idwrap.IDWrap) (string, []mexamplequery.Query) {
	parts := strings.SplitN(urlStr, "?", 2)
	if len(parts) == 1 {
		return urlStr, nil
	}

	baseURL := parts[0]
	queryStr := parts[1]
	var queries []mexamplequery.Query

	matches := queryParamPattern.FindAllStringSubmatch(queryStr, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			query := mexamplequery.Query{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				QueryKey:  match[1],
				Value:     match[2],
				Enable:    true,
			}
			queries = append(queries, query)
		}
	}

	return baseURL, queries
}

func removeQuotes(s string) string {
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
		return s[1 : len(s)-1]
	}
	return s
}

// extractCookies extracts cookies from a curl command and converts them to Cookie headers
func extractCookies(curlStr string, exampleID idwrap.IDWrap) []mexampleheader.Header {
	var cookieHeaders []mexampleheader.Header

	matches := cookiePattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		// Check each capture group (single quotes, double quotes, or no quotes)
		var cookieContent string
		if match[1] != "" {
			cookieContent = match[1] // Single quotes
		} else if match[2] != "" {
			cookieContent = match[2] // Double quotes
		} else {
			cookieContent = match[3] // No quotes
		}

		cookieHeader := mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: "Cookie",
			Value:     strings.TrimSpace(cookieContent),
			Enable:    true,
		}
		cookieHeaders = append(cookieHeaders, cookieHeader)
	}

	return cookieHeaders
}

// ExtractURLForTesting exposes extractURL for testing purposes
func ExtractURLForTesting(curlStr string) string {
	return extractURL(curlStr)
}
