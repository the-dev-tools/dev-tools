package errmap

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	neturl "net/url"
	"strings"
	"syscall"
)

// Code classifies high-level error categories for user-facing messages.
type Code string

const (
	CodeCanceled            Code = "canceled"
	CodeTimeout             Code = "timeout"
	CodeDNSError            Code = "dns_error"
	CodeInvalidURL          Code = "invalid_url"
	CodeUnsupportedScheme   Code = "unsupported_scheme"
	CodeConnectionRefused   Code = "connection_refused"
	CodeConnectionReset     Code = "connection_reset"
	CodeNetworkUnreachable  Code = "network_unreachable"
	CodeTLSUnknownAuthority Code = "tls_unknown_authority"
	CodeTLSHostnameMismatch Code = "tls_hostname_mismatch"
	CodeTLSHandshake        Code = "tls_handshake"
	CodeIO                  Code = "io_error"
	CodeUnexpected          Code = "unexpected"
	CodeExpressionSyntax    Code = "expression_syntax"
	CodeExpressionRuntime   Code = "expression_runtime"
)

// Error is a small, idiomatic wrapper that carries a code and context while
// preserving the original cause via Unwrap.
type Error struct {
	Code      Code
	Message   string
	Method    string
	URL       string
	Temporary bool
	Retryable bool
	cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	// Prefer explicit message
	msg := e.Message
	if msg == "" {
		msg = humanize(e.Code, e.cause)
	}
	// Add request context if available
	if e.Method != "" && e.URL != "" {
		return fmt.Sprintf("%s %s: %s", e.Method, e.URL, msg)
	}
	if e.URL != "" { // fallback
		return fmt.Sprintf("%s: %s", e.URL, msg)
	}
	return msg
}

func (e *Error) Unwrap() error { return e.cause }

// humanize produces a friendly message for a given code + cause.
func humanize(code Code, cause error) string {
	switch code {
	case CodeCanceled:
		return "request was canceled"
	case CodeTimeout:
		return "request timed out"
	case CodeDNSError:
		var dn *net.DNSError
		if errors.As(cause, &dn) {
			if dn.Name != "" {
				return fmt.Sprintf("DNS lookup failed for %q: %s", dn.Name, dn.Err)
			}
			return fmt.Sprintf("DNS error: %s", dn.Err)
		}
		return "DNS error"
	case CodeInvalidURL:
		return "invalid URL"
	case CodeUnsupportedScheme:
		return "unsupported protocol scheme"
	case CodeConnectionRefused:
		return "connection refused by remote host"
	case CodeConnectionReset:
		return "connection reset by peer"
	case CodeNetworkUnreachable:
		return "network unreachable"
	case CodeTLSUnknownAuthority:
		return "TLS: unknown certificate authority"
	case CodeTLSHostnameMismatch:
		return "TLS: certificate does not match host"
	case CodeTLSHandshake:
		return "TLS handshake failed"
	case CodeIO:
		return "I/O error"
	case CodeExpressionSyntax:
		if cause != nil {
			return fmt.Sprintf("expression syntax error: %s", cause.Error())
		}
		return "expression syntax error"
	case CodeExpressionRuntime:
		if cause != nil {
			return fmt.Sprintf("expression evaluation error: %s", cause.Error())
		}
		return "expression evaluation error"
	default:
		if cause != nil {
			return cause.Error()
		}
		return "unexpected error"
	}
}

// Map converts an arbitrary error into an *Error with a best-effort code.
// It keeps the original error as the cause.
func Map(err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return err // already mapped
	}
	// Context cancellation / timeout
	if errors.Is(err, context.Canceled) {
		return &Error{Code: CodeCanceled, Retryable: true, cause: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: CodeTimeout, Retryable: true, cause: err}
	}

	// url.Error often wraps timeouts, invalid URLs, etc.
	var uerr *url.Error
	if errors.As(err, &uerr) {
		// Timeout via url.Error implements Timeout through net.Error
		var t net.Error
		if errors.As(uerr.Err, &t) && t.Timeout() {
			return &Error{Code: CodeTimeout, Temporary: t.Temporary(), Retryable: true, cause: err}
		}
		// Invalid URL
		lower := strings.ToLower(uerr.Error())
		if strings.Contains(lower, "unsupported protocol scheme") {
			return &Error{Code: CodeUnsupportedScheme, cause: err}
		}
		if isInvalidURLMessage(lower, uerr.Err) {
			return &Error{Code: CodeInvalidURL, cause: err}
		}
		// Fallthrough: analyze underlying
		err = uerr.Err
	}

	// DNS
	var dnserr *net.DNSError
	if errors.As(err, &dnserr) {
		return &Error{Code: CodeDNSError, Temporary: dnserr.IsTemporary, Retryable: dnserr.IsTemporary, cause: dnserr}
	}

	// net.Error general timeouts/temporary
	var nerr net.Error
	if errors.As(err, &nerr) {
		if nerr.Timeout() {
			return &Error{Code: CodeTimeout, Temporary: nerr.Temporary(), Retryable: true, cause: nerr}
		}
	}

	// net.OpError with syscall specifics
	var operr *net.OpError
	if errors.As(err, &operr) {
		switch {
		case errors.Is(operr.Err, syscall.ECONNREFUSED):
			return &Error{Code: CodeConnectionRefused, Temporary: false, Retryable: true, cause: err}
		case errors.Is(operr.Err, syscall.ECONNRESET):
			return &Error{Code: CodeConnectionReset, Temporary: true, Retryable: true, cause: err}
		case errors.Is(operr.Err, syscall.ENETUNREACH), errors.Is(operr.Err, syscall.EHOSTUNREACH):
			return &Error{Code: CodeNetworkUnreachable, Temporary: true, Retryable: true, cause: err}
		}
	}

	// TLS/X.509
	var ua *x509.UnknownAuthorityError
	if errors.As(err, &ua) {
		return &Error{Code: CodeTLSUnknownAuthority, cause: err}
	}
	var hn *x509.HostnameError
	if errors.As(err, &hn) {
		return &Error{Code: CodeTLSHostnameMismatch, cause: err}
	}
	// Generic handshake phrase match (best effort)
	if strings.Contains(strings.ToLower(err.Error()), "handshake failure") || strings.Contains(strings.ToLower(err.Error()), "tls") {
		return &Error{Code: CodeTLSHandshake, cause: err}
	}

	// Fallbacks for common textual hints
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "timeout"):
		return &Error{Code: CodeTimeout, cause: err}
	case strings.Contains(lower, "unsupported protocol scheme"):
		return &Error{Code: CodeUnsupportedScheme, cause: err}
	case strings.Contains(lower, "refused"):
		return &Error{Code: CodeConnectionRefused, cause: err}
	case strings.Contains(lower, "reset"):
		return &Error{Code: CodeConnectionReset, cause: err}
	}

	return &Error{Code: CodeUnexpected, cause: err}
}

func isInvalidURLMessage(message string, cause error) bool {
	if strings.Contains(message, "invalid url") {
		return true
	}
	if strings.Contains(message, "invalid uri") {
		return true
	}
	if strings.Contains(message, "malformed url") {
		return true
	}

	var parseErr *neturl.Error
	if errors.As(cause, &parseErr) {
		inner := strings.ToLower(parseErr.Error())
		if strings.Contains(inner, "invalid url") || strings.Contains(inner, "invalid uri") || strings.Contains(inner, "malformed url") {
			return true
		}
	}

	return false
}

// New constructs an Error with the supplied code, message, and underlying cause.
func New(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

// MapRequestError annotates the mapped error with request context.
func MapRequestError(method, urlStr string, err error) error {
	if err == nil {
		return nil
	}
	m := Map(err)
	var me *Error
	if errors.As(m, &me) {
		me.Method = method
		me.URL = urlStr
		return me
	}
	return m
}

// --- Structured helpers ---

// ToJSON marshals an error into {"code":"...","message":"..."}.
// If err is not an *Error, code defaults to "unknown".
func ToJSON(err error) string {
	type payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err == nil {
		return `{"code":"unknown","message":""}`
	}
	var me *Error
	if errors.As(err, &me) {
		p := payload{Code: string(me.Code), Message: me.Error()}
		// Inline JSON without bringing in extra deps
		// Avoiding json.Marshal to keep ToJSON fast and allocation-light.
		// But we still need to escape quotes in message.
		escMsg := strings.ReplaceAll(p.Message, "\"", "\\\"")
		return fmt.Sprintf(`{"code":"%s","message":"%s"}`, p.Code, escMsg)
	}
	escMsg := strings.ReplaceAll(err.Error(), "\"", "\\\"")
	return fmt.Sprintf(`{"code":"unknown","message":"%s"}`, escMsg)
}

// Friendly returns a user-friendly, action-oriented message string.
// It uses request context (method/URL) when available, and produces
// clearer phrasing than the raw error text.
func Friendly(err error) string {
	if err == nil {
		return ""
	}
	var me *Error
	if !errors.As(err, &me) {
		return err.Error()
	}

	method := me.Method
	urlStr := me.URL
	ctx := ""
	if method != "" && urlStr != "" {
		ctx = fmt.Sprintf(" (%s %s)", method, urlStr)
	} else if urlStr != "" {
		ctx = fmt.Sprintf(" (%s)", urlStr)
	}

	switch me.Code {
	case CodeUnsupportedScheme:
		scheme := ""
		if u, perr := neturl.Parse(urlStr); perr == nil {
			scheme = u.Scheme
		} else if i := strings.Index(urlStr, "://"); i > 0 {
			scheme = urlStr[:i]
		}
		suggest := ""
		if scheme == "htps" { // common typo for https
			suggest = "https"
		} else if strings.HasPrefix(scheme, "htt") {
			if strings.Contains(scheme, "s") {
				suggest = "https"
			} else {
				suggest = "http"
			}
		}
		if scheme == "" {
			scheme = "<none>"
		}
		if suggest != "" {
			return fmt.Sprintf("Unsupported URL scheme '%s'%s. Did you mean '%s'?", scheme, ctx, suggest)
		}
		return fmt.Sprintf("Unsupported URL scheme '%s'%s.", scheme, ctx)
	case CodeInvalidURL:
		return fmt.Sprintf("The URL is invalid%s.", ctx)
	case CodeTimeout:
		return fmt.Sprintf("Request timed out%s.", ctx)
	case CodeCanceled:
		return "Request was canceled."
	case CodeDNSError:
		// Try to extract hostname for a clearer message
		host := ""
		if u, perr := neturl.Parse(urlStr); perr == nil {
			host = u.Hostname()
		}
		if host != "" {
			return fmt.Sprintf("Could not resolve host '%s'%s.", host, ctx)
		}
		return fmt.Sprintf("Could not resolve hostname%s.", ctx)
	case CodeConnectionRefused:
		return fmt.Sprintf("Could not connect — connection refused%s.", ctx)
	case CodeConnectionReset:
		return fmt.Sprintf("Connection reset by peer%s.", ctx)
	case CodeNetworkUnreachable:
		return fmt.Sprintf("Network unreachable%s.", ctx)
	case CodeTLSUnknownAuthority:
		return fmt.Sprintf("TLS certificate is not trusted by your system%s.", ctx)
	case CodeTLSHostnameMismatch:
		return fmt.Sprintf("TLS certificate does not match the requested host%s.", ctx)
	case CodeTLSHandshake:
		return fmt.Sprintf("TLS handshake failed%s.", ctx)
	case CodeIO:
		return fmt.Sprintf("I/O error%s.", ctx)
	default:
		// Fall back to the wrapped error text
		if s := me.Error(); s != "" {
			return s
		}
		return "Unexpected error."
	}
}
