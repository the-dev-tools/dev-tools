package node

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
)

func clientWithCookie(t *testing.T) httpclient.HttpClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "tok1", Path: "/"})
	}))
	t.Cleanup(srv.Close)
	client := httpclient.New()
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return client
}

func TestWithCookiesExposesReceivedCookies(t *testing.T) {
	vars := map[string]any{"Login": map[string]any{}}
	WithCookies(vars, clientWithCookie(t))
	cookies, ok := vars["cookies"].(map[string]any)
	if !ok || cookies["csrftoken"] != "tok1" {
		t.Fatalf("got %#v", vars["cookies"])
	}
}

func TestWithCookiesNeverOverridesAFlowVariable(t *testing.T) {
	vars := map[string]any{"cookies": "mine"}
	WithCookies(vars, clientWithCookie(t))
	if vars["cookies"] != "mine" {
		t.Fatalf("got %#v", vars["cookies"])
	}
}

func TestWithCookiesAddsNothingBeforeAnyCookie(t *testing.T) {
	vars := map[string]any{}
	WithCookies(vars, httpclient.New())
	if _, ok := vars["cookies"]; ok {
		t.Fatalf("got %#v", vars["cookies"])
	}
}
