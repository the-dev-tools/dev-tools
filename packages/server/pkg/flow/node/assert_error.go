package node

import (
	"fmt"
	"strings"
)

// maxBodyExcerpt bounds the response excerpt in an assertion error, so a large page or
// stack trace doesn't flood the console.
const maxBodyExcerpt = 200

// AssertionError reports a failed assertion with what the server actually answered: the
// status always, and for a non-2xx response a one-line excerpt of the body ("Invalid
// credentials"). A 2xx body is left out because it can carry tokens and this error is
// printed to CI logs.
func AssertionError(expr string, status int, body []byte) error {
	if status >= 200 && status < 300 {
		return fmt.Errorf("assertion failed: %s (got %d)", expr, status)
	}
	excerpt := strings.Join(strings.Fields(string(body)), " ")
	if excerpt == "" {
		return fmt.Errorf("assertion failed: %s (got %d)", expr, status)
	}
	if runes := []rune(excerpt); len(runes) > maxBodyExcerpt {
		excerpt = string(runes[:maxBodyExcerpt]) + "…"
	}
	return fmt.Errorf("assertion failed: %s (got %d: %s)", expr, status, excerpt)
}
