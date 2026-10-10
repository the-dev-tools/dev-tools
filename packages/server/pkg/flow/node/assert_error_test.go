package node

import (
	"strings"
	"testing"
)

func TestAssertionErrorShowsWhatTheServerAnswered(t *testing.T) {
	err := AssertionError("response.status == 200", []string{"response.status = 401"}, 401, []byte("{\n  \"error\": \"Invalid credentials\"\n}"))
	want := `assertion failed: response.status == 200 (got 401: { "error": "Invalid credentials" })`
	if err.Error() != want {
		t.Fatalf("got  %q\nwant %q", err.Error(), want)
	}
}

func TestAssertionErrorKeepsSuccessfulBodiesOutOfLogs(t *testing.T) {
	// A 2xx body can carry a token; the error ends up in CI logs. Only the checked value shows.
	err := AssertionError("response.body.role == \"admin\"", []string{`response.body.role = "user"`}, 200, []byte(`{"token":"eyJhbGciOi.secret.sig","role":"user"}`))
	want := `assertion failed: response.body.role == "admin" (got response.body.role = "user")`
	if err.Error() != want {
		t.Fatalf("got  %q\nwant %q", err.Error(), want)
	}
}

func TestAssertionErrorShowsCheckedValues(t *testing.T) {
	err := AssertionError("response.duration < 50", []string{"response.duration = 5120"}, 200, nil)
	want := `assertion failed: response.duration < 50 (got response.duration = 5120)`
	if err.Error() != want {
		t.Fatalf("got  %q\nwant %q", err.Error(), want)
	}
}

func TestAssertionErrorNon2xxAddsValuesAfterStatus(t *testing.T) {
	err := AssertionError(`response.body.name == "x"`, []string{`response.body.name = <missing>`}, 404, []byte("Not found"))
	want := `assertion failed: response.body.name == "x" (got 404: Not found; response.body.name = <missing>)`
	if err.Error() != want {
		t.Fatalf("got  %q\nwant %q", err.Error(), want)
	}
}

func TestAssertionErrorWithoutValuesFallsBackToStatus(t *testing.T) {
	err := AssertionError("1 == 2", nil, 200, nil)
	want := `assertion failed: 1 == 2 (got 200)`
	if err.Error() != want {
		t.Fatalf("got  %q\nwant %q", err.Error(), want)
	}
}

func TestAssertionErrorTruncatesLongBodies(t *testing.T) {
	err := AssertionError("response.status == 200", nil, 500, []byte(strings.Repeat("x", 1000)))
	msg := err.Error()
	if !strings.HasSuffix(msg, "…)") || len(msg) > 300 {
		t.Fatalf("expected a short, truncated excerpt, got %d bytes: %q", len(msg), msg)
	}
}
