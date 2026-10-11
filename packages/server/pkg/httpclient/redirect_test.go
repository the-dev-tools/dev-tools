package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func redirectServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dashboard":
			http.Redirect(w, r, "/auth", http.StatusFound)
		case "/auth":
			http.Redirect(w, r, "/login?next=/dashboard", http.StatusSeeOther)
		default:
			_, _ = w.Write([]byte("login page"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFollowedRedirectsReportTheFinalURLAndCount(t *testing.T) {
	srv := redirectServer(t)
	resp, err := SendRequestAndConvertStream(context.Background(), New(), &Request{Method: "GET", URL: srv.URL + "/dashboard"}, idwrap.NewNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.URL != srv.URL+"/login?next=/dashboard" || resp.Redirects != 2 {
		t.Fatalf("got status %d, url %q, redirects %d", resp.StatusCode, resp.URL, resp.Redirects)
	}
	v := ConvertResponseToVar(resp)
	if v.URL != resp.URL || v.Redirects != 2 {
		t.Fatalf("response var: url %q, redirects %d", v.URL, v.Redirects)
	}
}

func TestWithoutRedirectsReturnsThe30x(t *testing.T) {
	srv := redirectServer(t)
	client := New()
	ctx := WithoutRedirects(context.Background())
	resp, err := SendRequestAndConvertStream(ctx, client, &Request{Method: "GET", URL: srv.URL + "/dashboard"}, idwrap.NewNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || resp.URL != srv.URL+"/dashboard" || resp.Redirects != 0 {
		t.Fatalf("got status %d, url %q, redirects %d", resp.StatusCode, resp.URL, resp.Redirects)
	}
	location := ""
	for _, h := range resp.Headers {
		if h.HeaderKey == "Location" {
			location = h.Value
		}
	}
	if location != "/auth" {
		t.Fatalf("Location = %q", location)
	}

	// The same (shared) client still follows redirects for other requests.
	resp, err = SendRequestAndConvertStream(context.Background(), client, &Request{Method: "GET", URL: srv.URL + "/dashboard"}, idwrap.NewNow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Redirects != 2 {
		t.Fatalf("shared client: status %d, redirects %d", resp.StatusCode, resp.Redirects)
	}
}
