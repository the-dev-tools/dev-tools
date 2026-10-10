package node

import (
	"fmt"
	"strings"
)

// maxBodyExcerpt bounds the response excerpt in an assertion error, so a large page or
// stack trace doesn't flood the console.
const maxBodyExcerpt = 200

// AssertionError reports a failed assertion with what it checked and what the server
// answered. values are the variable paths the expression read with their actual values
// (`response.duration = 5120`). For a 2xx response only the values are shown: the body is
// left out because it can carry tokens and this error is printed to CI logs. For any other
// status the status and a one-line excerpt of the body come first ("got 401: Invalid
// credentials"), then the values, minus response.status, which the status already says.
func AssertionError(expr string, values []string, status int, body []byte) error {
	if status >= 200 && status < 300 {
		if len(values) == 0 {
			return fmt.Errorf("assertion failed: %s (got %d)", expr, status)
		}
		return fmt.Errorf("assertion failed: %s (got %s)", expr, strings.Join(values, ", "))
	}

	got := fmt.Sprint(status)
	excerpt := strings.Join(strings.Fields(string(body)), " ")
	if excerpt != "" {
		if runes := []rune(excerpt); len(runes) > maxBodyExcerpt {
			excerpt = string(runes[:maxBodyExcerpt]) + "…"
		}
		got += ": " + excerpt
	}
	extra := make([]string, 0, len(values))
	for _, v := range values {
		if !strings.HasPrefix(v, "response.status = ") {
			extra = append(extra, v)
		}
	}
	if len(extra) > 0 {
		got += "; " + strings.Join(extra, ", ")
	}
	return fmt.Errorf("assertion failed: %s (got %s)", expr, got)
}
