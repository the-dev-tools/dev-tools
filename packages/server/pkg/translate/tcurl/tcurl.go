package tcurl

import (
	"fmt"
	"regexp"
	"strings"
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
var (
	// URL pattern matches URLs in curl commands
	urlPattern = regexp.MustCompile(`(?:https?://|www\.)[^\s'"]+`)

	// Method pattern matches the -X or --request flag followed by the HTTP method
	methodPattern = regexp.MustCompile(`(?:-X|--request)\s+(?:'([A-Z]+)'|"([A-Z]+)"|([A-Z]+))`)

	// Header pattern matches -H or --header flags with their values
	headerPattern = regexp.MustCompile(`(?:-H|--header)\s+(?:'([^:]+):([^']+)'|"([^:]+):([^"]+)"|([^:]+):([^'"\s]+))`)

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
		Name:         fmt.Sprintf("CURL - %s", baseURL),
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
		Name:         "Example from curl",
		CollectionID: collectionID,
		IsDefault:    true,
		BodyType:     bodyType,
	}
	result.Examples = append(result.Examples, example)

	// Create empty raw body if there's no raw body but other body types exist
	// SQL depends on having raw body entries
	if len(result.RawBodies) == 0 && (len(result.FormBodies) > 0 || len(result.UrlEncodedBodies) > 0) {
		emptyRawBody := mbodyraw.ExampleBodyRaw{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Data:      []byte{}, // Empty data
		}
		result.RawBodies = append(result.RawBodies, emptyRawBody)
	}

	return result, nil
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
			fields[i-1] == "curl" || fields[i-1] == "-L" {
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
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Data:      []byte(content),
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
