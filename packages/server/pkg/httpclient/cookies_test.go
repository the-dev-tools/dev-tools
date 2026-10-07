package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookiesRemembersEveryCookieTheServerSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s1", HttpOnly: true, Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "XSRF-TOKEN", Value: "a%2Bb%3D", Path: "/"})
		if r.URL.Path == "/rotate" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s2", Path: "/"})
		}
	}))
	defer srv.Close()

	client := New()
	if got := Cookies(client); len(got) != 0 {
		t.Fatalf("expected no cookies before any response, got %v", got)
	}
	for _, path := range []string{"/", "/rotate"} {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}

	got := Cookies(client)
	// The latest value wins, and values are URL-decoded the way browsers' client code reads
	// them (axios decodes XSRF-TOKEN before sending it back as X-XSRF-TOKEN).
	if got["sid"] != "s2" || got["XSRF-TOKEN"] != "a+b=" {
		t.Fatalf("got %v", got)
	}
}

func TestCookiesOfAClientWithoutARecordingJar(t *testing.T) {
	if got := Cookies(&http.Client{}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
