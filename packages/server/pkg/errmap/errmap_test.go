package errmap

import (
    "context"
    "crypto/x509"
    "errors"
    "net"
    "net/url"
    "syscall"
    "testing"
    "strings"
)

// timedErr is a test helper implementing net.Error with Timeout=true.
type timedErr struct{}

func (timedErr) Error() string   { return "timeout" }
func (timedErr) Timeout() bool   { return true }
func (timedErr) Temporary() bool { return true }

func TestMap_ContextDeadline(t *testing.T) {
    err := context.DeadlineExceeded
    got := Map(err)
    e, ok := got.(*Error)
    if !ok {
        t.Fatalf("expected *Error, got %T", got)
    }
    if e.Code != CodeTimeout {
        t.Fatalf("expected code %s, got %s", CodeTimeout, e.Code)
    }
}

func TestMap_ContextCanceled(t *testing.T) {
    err := context.Canceled
    got := Map(err)
    e := got.(*Error)
    if e.Code != CodeCanceled {
        t.Fatalf("expected code %s, got %s", CodeCanceled, e.Code)
    }
}

func TestMap_NetTimeout(t *testing.T) {
    var e net.Error = timedErr{}
    got := Map(e)
    if got.(*Error).Code != CodeTimeout {
        t.Fatalf("expected timeout mapping, got %v", got)
    }
}

func TestMap_DNSError(t *testing.T) {
    dn := &net.DNSError{Name: "example.invalid", Err: "no such host"}
    got := Map(dn)
    if got.(*Error).Code != CodeDNSError {
        t.Fatalf("expected dns mapping, got %v", got)
    }
    if msg := got.Error(); msg == "" {
        t.Fatalf("expected user message, got empty")
    }
}

func TestMap_ConnRefused(t *testing.T) {
    op := &net.OpError{Err: syscall.ECONNREFUSED}
    got := Map(op)
    if got.(*Error).Code != CodeConnectionRefused {
        t.Fatalf("expected connection_refused, got %v", got)
    }
}

func TestMap_TLSUnknownAuthority(t *testing.T) {
    got := Map(&x509.UnknownAuthorityError{})
    if got.(*Error).Code != CodeTLSUnknownAuthority {
        t.Fatalf("expected tls_unknown_authority, got %v", got)
    }
}

func TestMapRequestError_Annotates(t *testing.T) {
    base := errors.New("some error")
    got := MapRequestError("GET", "https://api.example.com", base)
    e := got.(*Error)
    if e.Method != "GET" || e.URL != "https://api.example.com" {
        t.Fatalf("expected request annotation, got %+v", e)
    }
}

func TestFriendly_UnsupportedSchemeSuggestion(t *testing.T) {
    // Simulate unsupported scheme error from url.Error
    badURL := "htps://google.com"
    uerr := &url.Error{Op: "Get", URL: badURL, Err: errors.New("unsupported protocol scheme \"htps\"")}
    mapped := MapRequestError("GET", badURL, uerr)
    msg := Friendly(mapped)
    if !strings.Contains(msg, "Unsupported URL scheme 'htps'") {
        t.Fatalf("expected unsupported scheme message, got: %s", msg)
    }
    if !strings.Contains(msg, "Did you mean 'https'") {
        t.Fatalf("expected suggestion for https, got: %s", msg)
    }
}
