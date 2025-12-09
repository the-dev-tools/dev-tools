//nolint:revive // exported
package tcurlv2

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
)

// CurlResolvedV2 contains the resolved HTTP request data using the new models
type CurlResolvedV2 struct {
	// Primary HTTP request
	HTTP mhttp.HTTP

	// Associated data structures
	SearchParams   []mhttp.HTTPSearchParam
	Headers        []mhttp.HTTPHeader
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        *mhttp.HTTPBodyRaw

	// File system integration
	File mfile.File
}

// ConvertCurlOptions contains options for the curl conversion
type ConvertCurlOptions struct {
	WorkspaceID  idwrap.IDWrap
	FolderID     *idwrap.IDWrap
	ParentHttpID *idwrap.IDWrap // For delta system support
	IsDelta      bool           // Whether this is a delta variation
	DeltaName    *string        // Optional delta name
	Filename     string         // Optional filename (defaults to URL)
}

// Regular expressions for parsing curl commands (same as original tcurl)
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

// ConvertCurl converts a curl command string to the new HTTP model structures
func ConvertCurl(curlStr string, opts ConvertCurlOptions) (*CurlResolvedV2, error) {
	// Normalize the curl command to handle multi-line input
	normalizedCurl := normalizeCurlCommand(curlStr)

	// Validate that it's a curl command
	if !strings.HasPrefix(strings.TrimSpace(normalizedCurl), "curl") {
		return nil, fmt.Errorf("invalid curl command")
	}

	// Generate new ID for the HTTP request
	httpID := idwrap.NewNow()

	// Extract URL
	fullURL := extractURL(normalizedCurl)
	if fullURL == "" {
		return nil, fmt.Errorf("URL not found in curl command")
	}

	// Parse query parameters from URL
	baseURL, searchParams := parseURLAndSearchQueries(fullURL, httpID)

	// Extract method
	method := extractMethod(normalizedCurl)

	// Extract headers
	headers := extractHeaders(normalizedCurl, httpID)

	// Extract cookies and add them as headers
	cookieHeaders := extractCookies(normalizedCurl, httpID)
	headers = append(headers, cookieHeaders...)

	// Extract data bodies
	hasDataFlag := false
	bodyRaw := extractRawBody(normalizedCurl, httpID, &hasDataFlag)
	bodyForms := extractBodyForms(normalizedCurl, httpID, &hasDataFlag)
	bodyUrlencoded := extractBodyUrlencoded(normalizedCurl, httpID, &hasDataFlag)

	// If no explicit method was provided but we have data flags, assume POST
	if method == "GET" && hasDataFlag {
		method = "POST"
	}

	// Generate filename from URL if not provided
	filename := opts.Filename
	if filename == "" {
		filename = generateFilenameFromURL(baseURL)
	}

	// Create the primary HTTP request
	httpRequest := mhttp.HTTP{
		ID:           httpID,
		WorkspaceID:  opts.WorkspaceID,
		FolderID:     opts.FolderID,
		Name:         filename,
		Url:          baseURL,
		Method:       method,
		Description:  "", // Could be populated from curl comments in the future
		ParentHttpID: opts.ParentHttpID,
		IsDelta:      opts.IsDelta,
		DeltaName:    opts.DeltaName,
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	}

	// Create the file record with proper HTTP content kind
	file := mfile.File{
		ID:          idwrap.NewNow(),
		WorkspaceID: opts.WorkspaceID,
		ParentID:    nil, // Root level
		ContentID:   &httpID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        filename,
		Order:       0, // Will be set by the caller
		UpdatedAt:   time.Now(),
	}

	// Create the resolved structure
	result := &CurlResolvedV2{
		HTTP:           httpRequest,
		SearchParams:   searchParams,
		Headers:        headers,
		BodyForms:      bodyForms,
		BodyUrlencoded: bodyUrlencoded,
		BodyRaw:        bodyRaw,
		File:           file,
	}

	return result, nil
}

// GetFileContent returns both the file and HTTP content for easy processing
func GetFileContent(resolved *CurlResolvedV2) (*mfile.File, *mhttp.HTTP) {
	return &resolved.File, &resolved.HTTP
}

// BuildCurl assembles a curl command string from the resolved HTTP structure.
// This creates a curl command that represents the HTTP request.
func BuildCurl(resolved *CurlResolvedV2) (string, error) {
	if resolved.HTTP.ID.Compare(idwrap.IDWrap{}) == 0 {
		return "", errors.New("tcurlv2: no HTTP request to build curl from")
	}

	method := strings.ToUpper(strings.TrimSpace(resolved.HTTP.Method))
	fullURL := buildURLWithSearchQueries(resolved.HTTP.Url, resolved.SearchParams)
	methodRequiresFlag := method != "" && method != "GET"

	// Sort headers for consistent output
	headers := make([]mhttp.HTTPHeader, len(resolved.Headers))
	copy(headers, resolved.Headers)
	sortHeaders(headers)

	// Sort body forms for consistent output
	bodyForms := make([]mhttp.HTTPBodyForm, len(resolved.BodyForms))
	copy(bodyForms, resolved.BodyForms)
	sortBodyForms(bodyForms)

	// Sort body urlencoded for consistent output
	bodyUrlencoded := make([]mhttp.HTTPBodyUrlencoded, len(resolved.BodyUrlencoded))
	copy(bodyUrlencoded, resolved.BodyUrlencoded)
	sortBodyUrlencoded(bodyUrlencoded)

	// Get raw body data
	var rawBodyData []byte
	if resolved.BodyRaw != nil {
		rawBodyData = resolved.BodyRaw.RawData
		if resolved.BodyRaw.CompressionType != compress.CompressTypeNone && len(resolved.BodyRaw.RawData) > 0 {
			decompressed, err := compress.Decompress(resolved.BodyRaw.RawData, resolved.BodyRaw.CompressionType)
			if err != nil {
				return "", fmt.Errorf("tcurlv2: decompress raw body: %w", err)
			}
			rawBodyData = decompressed
		}
	}

	args := []string{singleQuote(fullURL)}
	if methodRequiresFlag {
		args = append(args, "-X "+method)
	}

	for _, header := range headers {
		if header.Enabled {
			args = append(args, "-H "+singleQuote(fmt.Sprintf("%s: %s", header.Key, header.Value)))
		}
	}

	if len(rawBodyData) > 0 {
		args = append(args, "--data-raw "+singleQuote(string(rawBodyData)))
	}

	for _, form := range bodyForms {
		if form.Enabled {
			args = append(args, "-F "+singleQuote(fmt.Sprintf("%s=%s", form.Key, form.Value)))
		}
	}

	for _, urlBody := range bodyUrlencoded {
		if urlBody.Enabled {
			args = append(args, "--data-urlencode "+singleQuote(fmt.Sprintf("%s=%s", urlBody.Key, urlBody.Value)))
		}
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

// Helper functions (adapted from original tcurl)

// normalizeCurlCommand handles both single-line and multi-line formats
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

func extractHeaders(curlStr string, httpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var headers []mhttp.HTTPHeader

	matches := headerPattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		// Check single quotes pattern (groups 1,2), double quotes pattern (groups 3,4), or no quotes pattern (groups 5,6)
		var key, value string
		switch {
		case match[1] != "":
			key, value = match[1], match[2] // Single quotes
		case match[3] != "":
			key, value = match[3], match[4] // Double quotes
		default:
			key, value = match[5], match[6] // No quotes
		}

		header := mhttp.HTTPHeader{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         strings.TrimSpace(key),
			Value:       strings.TrimSpace(value),
			Description: "",
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		headers = append(headers, header)
	}

	return headers
}

func extractCookies(curlStr string, httpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var cookieHeaders []mhttp.HTTPHeader

	matches := cookiePattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		// Check each capture group (single quotes, double quotes, or no quotes)
		var cookieContent string
		switch {
		case match[1] != "":
			cookieContent = match[1] // Single quotes
		case match[2] != "":
			cookieContent = match[2] // Double quotes
		default:
			cookieContent = match[3] // No quotes
		}

		cookieHeader := mhttp.HTTPHeader{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         "Cookie",
			Value:       strings.TrimSpace(cookieContent),
			Description: "",
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		cookieHeaders = append(cookieHeaders, cookieHeader)
	}

	return cookieHeaders
}

func extractRawBody(curlStr string, httpID idwrap.IDWrap, hasDataFlag *bool) *mhttp.HTTPBodyRaw {
	matches := dataPattern.FindAllStringSubmatch(curlStr, -1)
	if len(matches) == 0 {
		return nil
	}

	*hasDataFlag = true

	// Use the first match for raw body
	var content string
	switch {
	case matches[0][1] != "":
		content = matches[0][1] // Single quotes
	case matches[0][2] != "":
		content = matches[0][2] // Double quotes
	default:
		content = matches[0][3] // No quotes
	}

	body := &mhttp.HTTPBodyRaw{
		ID:              idwrap.NewNow(),
		HttpID:          httpID,
		RawData:         []byte(content),
		CompressionType: compress.CompressTypeNone,
		CreatedAt:       time.Now().UnixMilli(),
		UpdatedAt:       time.Now().UnixMilli(),
	}

	return body
}

func extractBodyUrlencoded(curlStr string, httpID idwrap.IDWrap, hasDataFlag *bool) []mhttp.HTTPBodyUrlencoded {
	var bodies []mhttp.HTTPBodyUrlencoded

	matches := dataUrlEncodePattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		*hasDataFlag = true

		// Check each capture group (single quotes, double quotes, or no quotes)
		var key, value string
		switch {
		case match[1] != "":
			key, value = match[1], match[2] // Single quotes
		case match[3] != "":
			key, value = match[3], match[4] // Double quotes
		default:
			key, value = match[5], match[6] // No quotes
		}

		body := mhttp.HTTPBodyUrlencoded{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         key,
			Value:       value,
			Description: "",
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		bodies = append(bodies, body)
	}

	return bodies
}

func extractBodyForms(curlStr string, httpID idwrap.IDWrap, hasDataFlag *bool) []mhttp.HTTPBodyForm {
	var forms []mhttp.HTTPBodyForm

	matches := formDataPattern.FindAllStringSubmatch(curlStr, -1)
	for _, match := range matches {
		*hasDataFlag = true

		// Check each capture group (single quotes, double quotes, or no quotes)
		var key, value string
		switch {
		case match[1] != "":
			key, value = match[1], match[2] // Single quotes
		case match[3] != "":
			key, value = match[3], match[4] // Double quotes
		default:
			key, value = match[5], match[6] // No quotes
		}

		form := mhttp.HTTPBodyForm{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         key,
			Value:       value,
			Description: "",
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		forms = append(forms, form)
	}

	return forms
}

func parseURLAndSearchQueries(urlStr string, httpID idwrap.IDWrap) (string, []mhttp.HTTPSearchParam) {
	parts := strings.SplitN(urlStr, "?", 2)
	if len(parts) == 1 {
		return urlStr, nil
	}

	baseURL := parts[0]
	queryStr := parts[1]
	var searchParams []mhttp.HTTPSearchParam

	matches := queryParamPattern.FindAllStringSubmatch(queryStr, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			param := mhttp.HTTPSearchParam{
				ID:        idwrap.NewNow(),
				HttpID:    httpID,
				Key:       match[1],
				Value:     match[2],
				Enabled:   true,
				CreatedAt: time.Now().UnixMilli(),
				UpdatedAt: time.Now().UnixMilli(),
			}
			searchParams = append(searchParams, param)
		}
	}

	return baseURL, searchParams
}

func removeQuotes(s string) string {
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
		return s[1 : len(s)-1]
	}
	return s
}

func generateFilenameFromURL(urlStr string) string {
	// Extract the path part of the URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return "untitled"
	}

	// Use the path, or hostname if path is empty
	path := u.Path
	if path == "" || path == "/" {
		path = u.Hostname()
	}

	// Clean up the path to make it a valid filename
	path = strings.Trim(path, "/")
	if path == "" {
		path = "untitled"
	}

	// Replace problematic characters
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, " ", "_")

	return path
}

func buildURLWithSearchQueries(baseURL string, searchParams []mhttp.HTTPSearchParam) string {
	values := url.Values{}
	for _, param := range searchParams {
		if param.Enabled {
			values.Add(param.Key, param.Value)
		}
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

func sortHeaders(headers []mhttp.HTTPHeader) {
	sort.SliceStable(headers, func(i, j int) bool {
		ki := strings.ToLower(headers[i].Key)
		kj := strings.ToLower(headers[j].Key)
		if ki == kj {
			return headers[i].Key < headers[j].Key
		}
		return ki < kj
	})
}

func sortBodyForms(forms []mhttp.HTTPBodyForm) {
	sort.SliceStable(forms, func(i, j int) bool {
		if forms[i].Key == forms[j].Key {
			return forms[i].Value < forms[j].Value
		}
		return forms[i].Key < forms[j].Key
	})
}

func sortBodyUrlencoded(bodies []mhttp.HTTPBodyUrlencoded) {
	sort.SliceStable(bodies, func(i, j int) bool {
		if bodies[i].Key == bodies[j].Key {
			return bodies[i].Value < bodies[j].Value
		}
		return bodies[i].Key < bodies[j].Key
	})
}

func singleQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// ExtractURLForTesting exposes extractURL for testing purposes
func ExtractURLForTesting(curlStr string) string {
	return extractURL(curlStr)
}
