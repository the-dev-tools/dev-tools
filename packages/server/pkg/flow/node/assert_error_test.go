package node

import (
	"strings"
	"testing"
)

func TestAssertionErrorShowsWhatTheServerAnswered(t *testing.T) {
	err := AssertionError("response.status == 200", 401, []byte("{\n  \"error\": \"Invalid credentials\"\n}"))
	want := `assertion failed: response.status == 200 (got 401: { "error": "Invalid credentials" })`
	if err.Error() != want {
		t.Fatalf("got  %q\nwant %q", err.Error(), want)
	}
}

func TestAssertionErrorKeepsSuccessfulBodiesOutOfLogs(t *testing.T) {
	// A 2xx body can carry a token; the error ends up in CI logs.
	err := AssertionError("response.body.role == \"admin\"", 200, []byte(`{"token":"eyJhbGciOi.secret.sig","role":"user"}`))
	want := `assertion failed: response.body.role == "admin" (got 200)`
	if err.Error() != want {
		t.Fatalf("got  %q\nwant %q", err.Error(), want)
	}
}

func TestAssertionErrorTruncatesLongBodies(t *testing.T) {
	err := AssertionError("response.status == 200", 500, []byte(strings.Repeat("x", 1000)))
	msg := err.Error()
	if !strings.HasSuffix(msg, "…)") || len(msg) > 300 {
		t.Fatalf("expected a short, truncated excerpt, got %d bytes: %q", len(msg), msg)
	}
}
