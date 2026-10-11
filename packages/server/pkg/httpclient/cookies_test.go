package httpclient

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

// Browsers treat localhost as a secure context and send Secure cookies to it over plain
// http; a local dev server that sets `Secure` session cookies must keep working.
func TestSecureCookiesAreSentToLocalhostOverHTTP(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]", "app.localhost", "LOCALHOST"} {
		t.Run(host, func(t *testing.T) {
			jar := newRecordingJar()
			u, err := url.Parse("http://" + host + ":3000/login")
			if err != nil {
				t.Fatal(err)
			}
			jar.SetCookies(u, []*http.Cookie{
				{Name: "sid", Value: "s1", Secure: true, Path: "/"},
				{Name: "plain", Value: "p", Path: "/"},
			})
			got := map[string]string{}
			for _, c := range jar.Cookies(u) {
				got[c.Name] = c.Value
			}
			if got["sid"] != "s1" || got["plain"] != "p" {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestSecureCookiesAreNotSentToOtherHTTPHosts(t *testing.T) {
	jar := newRecordingJar()
	u, _ := url.Parse("http://example.com/login")
	jar.SetCookies(u, []*http.Cookie{
		{Name: "sid", Value: "s1", Secure: true, Path: "/"},
		{Name: "plain", Value: "p", Path: "/"},
	})
	got := jar.Cookies(u)
	if len(got) != 1 || got[0].Name != "plain" {
		t.Fatalf("got %v", got)
	}
	https, _ := url.Parse("https://example.com/login")
	if got := jar.Cookies(https); len(got) != 2 {
		t.Fatalf("https: got %v", got)
	}
}

// End to end: a local server sets a Secure session cookie and the client sends it back.
func TestClientSendsSecureCookieBackToLocalServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s1", Secure: true, HttpOnly: true, Path: "/"})
			return
		}
		if c, err := r.Cookie("sid"); err == nil && c.Value == "s1" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	client := New()
	for _, path := range []string{"/login", "/me"} {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}
}
