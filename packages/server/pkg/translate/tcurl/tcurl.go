package tcurl

import (
	"fmt"
	"strings"
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

func ConvertCurl(curlStr string) (CurlResolved, error) {
	result := CurlResolved{
		Apis:             []mitemapi.ItemApi{},
		Examples:         []mitemapiexample.ItemApiExample{},
		Queries:          []mexamplequery.Query{},
		Headers:          []mexampleheader.Header{},
		RawBodies:        []mbodyraw.ExampleBodyRaw{},
		FormBodies:       []mbodyform.BodyForm{},
		UrlEncodedBodies: []mbodyurl.BodyURLEncoded{},
	}

	// Split the curl command into lines, handling possible line continuations
	lines := strings.Split(strings.ReplaceAll(curlStr, " \\\n", " "), "\n")
	var method, url string
	var hasDataFlag bool

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 {
			if !strings.HasPrefix(line, "curl") {
				return CurlResolved{}, fmt.Errorf("invalid curl command")
			}

			// Extract method and URL
			parts := splitCurlCommand(line)
			url = extractURL(parts)
			method = extractMethod(parts)
			if url == "" {
				return CurlResolved{}, fmt.Errorf("URL not found in curl command")
			}

			// Parse query parameters from URL
			baseURL, queries := parseURLAndQueries(url)
			url = baseURL
			result.Queries = queries

			// Create API item
			api := mitemapi.ItemApi{
				Method: method,
				Url:    url,
			}
			result.Apis = append(result.Apis, api)

			// Continue parsing other flags from the first line
			parseCurlFlags(parts, &result, &hasDataFlag)
			continue
		}

		// Process flags in subsequent lines or remaining flags in first line
		if i > 0 || strings.Contains(line, " -") {
			if strings.Contains(line, " -H ") || strings.Contains(line, " --header ") {
				fmt.Println("Header found", line)
				header, err := parseHeader(line)
				if err != nil {
					return CurlResolved{}, err
				}
				result.Headers = append(result.Headers, header)
			} else if strings.Contains(line, " --data ") || strings.Contains(line, " -d ") ||
				strings.Contains(line, " --data-raw ") {
				hasDataFlag = true
				body, err := parseRawBody(line)
				if err != nil {
					return CurlResolved{}, err
				}
				result.RawBodies = append(result.RawBodies, body)
			} else if strings.Contains(line, " --data-urlencode ") {
				hasDataFlag = true
				body, err := parseURLEncodedBody(line)
				if err != nil {
					return CurlResolved{}, err
				}
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, body)
			} else if strings.Contains(line, " --form ") || strings.Contains(line, " -F ") {
				hasDataFlag = true
				form, err := parseFormBody(line)
				if err != nil {
					return CurlResolved{}, err
				}
				result.FormBodies = append(result.FormBodies, form)
			}
		}
	}

	// If no explicit method was provided but we have data flags, assume POST
	if method == "GET" && hasDataFlag {
		if len(result.Apis) > 0 {
			result.Apis[0].Method = "POST"
		}
	}

	// Create example
	if len(result.Apis) > 0 {
		example := mitemapiexample.ItemApiExample{
			ItemApiID: result.Apis[0].ID,
			Name:      "Example from curl",
		}
		result.Examples = append(result.Examples, example)
	}

	return result, nil
}

// Helper functions for parsing curl commands
func splitCurlCommand(cmd string) []string {
	var parts []string
	inQuote := false
	var quoteChar rune
	var current strings.Builder

	for _, r := range cmd {
		if (r == '\'' || r == '"') && (!inQuote || r == quoteChar) {
			inQuote = !inQuote
			quoteChar = r
			continue
		}

		if r == ' ' && !inQuote {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func extractURL(parts []string) string {
	for i, part := range parts {
		if !strings.HasPrefix(part, "-") && i > 0 && !strings.HasPrefix(parts[i-1], "-") {
			return part
		}
	}
	return ""
}

func extractMethod(parts []string) string {
	for i, part := range parts {
		if (part == "-X" || part == "--request") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "GET" // Default to GET if method not specified
}

func parseURLAndQueries(urlStr string) (string, []mexamplequery.Query) {
	parts := strings.SplitN(urlStr, "?", 2)
	if len(parts) == 1 {
		return urlStr, nil
	}

	baseURL := parts[0]
	queryStr := parts[1]
	var queries []mexamplequery.Query

	for _, qp := range strings.Split(queryStr, "&") {
		qparts := strings.SplitN(qp, "=", 2)
		if len(qparts) == 2 {
			query := mexamplequery.Query{
				QueryKey: qparts[0],
				Value:    qparts[1],
			}
			queries = append(queries, query)
		}
	}

	return baseURL, queries
}

func parseHeader(line string) (mexampleheader.Header, error) {
	headerStr := extractQuotedContent(line)
	headerParts := strings.SplitN(headerStr, ":", 2)
	if len(headerParts) != 2 {
		return mexampleheader.Header{}, fmt.Errorf("invalid header format: %s", headerStr)
	}

	return mexampleheader.Header{
		HeaderKey: strings.TrimSpace(headerParts[0]),
		Value:     strings.TrimSpace(headerParts[1]),
	}, nil
}

func parseRawBody(line string) (mbodyraw.ExampleBodyRaw, error) {
	content := extractQuotedContent(line)
	return mbodyraw.ExampleBodyRaw{
		Data: []byte(content),
	}, nil
}

func parseURLEncodedBody(line string) (mbodyurl.BodyURLEncoded, error) {
	content := extractQuotedContent(line)
	parts := strings.SplitN(content, "=", 2)
	if len(parts) != 2 {
		return mbodyurl.BodyURLEncoded{}, fmt.Errorf("invalid url-encoded parameter: %s", content)
	}

	return mbodyurl.BodyURLEncoded{
		BodyKey: parts[0],
		Value:   parts[1],
	}, nil
}

func parseFormBody(line string) (mbodyform.BodyForm, error) {
	content := extractQuotedContent(line)
	parts := strings.SplitN(content, "=", 2)
	if len(parts) != 2 {
		return mbodyform.BodyForm{}, fmt.Errorf("invalid form parameter: %s", content)
	}

	return mbodyform.BodyForm{
		BodyKey: parts[0],
		Value:   parts[1],
	}, nil
}

func extractQuotedContent(line string) string {
	if strings.Contains(line, "'") {
		parts := strings.Split(line, "'")
		if len(parts) >= 3 {
			return parts[1]
		}
	} else if strings.Contains(line, "\"") {
		parts := strings.Split(line, "\"")
		if len(parts) >= 3 {
			return parts[1]
		}
	}
	return ""
}

func parseCurlFlags(parts []string, result *CurlResolved, hasDataFlag *bool) {
	for i := 1; i < len(parts); i++ {
		// Skip the URL and method parts which were already handled
		if !strings.HasPrefix(parts[i], "-") {
			continue
		}

		switch parts[i] {
		case "-H", "--header":
			if i+1 < len(parts) {
				header, err := parseHeader("--header '" + parts[i+1] + "'")
				if err == nil {
					result.Headers = append(result.Headers, header)
				}
				i++
			}
		case "-d", "--data", "--data-raw":
			*hasDataFlag = true
			if i+1 < len(parts) {
				body, err := parseRawBody("--data-raw '" + parts[i+1] + "'")
				if err == nil {
					result.RawBodies = append(result.RawBodies, body)
				}
				i++
			}
		case "--data-urlencode":
			*hasDataFlag = true
			if i+1 < len(parts) {
				body, err := parseURLEncodedBody("--data-urlencode '" + parts[i+1] + "'")
				if err == nil {
					result.UrlEncodedBodies = append(result.UrlEncodedBodies, body)
				}
				i++
			}
		case "-F", "--form":
			*hasDataFlag = true
			if i+1 < len(parts) {
				form, err := parseFormBody("--form '" + parts[i+1] + "'")
				if err == nil {
					result.FormBodies = append(result.FormBodies, form)
				}
				i++
			}
		}
	}
}
