package httpclient

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
)

// recordingJar is a cookie jar that also remembers the latest value of every cookie it was
// given, by name, so flows can read them: `X-CSRF-Token: {{ cookies.csrftoken }}`. Double-submit
// CSRF protection (Django, Laravel, Angular, many Express apps) has the client copy a cookie
// into a header; a flow can only do that if the cookie's value is a template variable.
type recordingJar struct {
	*cookiejar.Jar
	mu     sync.Mutex
	latest map[string]string
}

func newRecordingJar() *recordingJar {
	jar, _ := cookiejar.New(nil) // never errors with nil options
	return &recordingJar{Jar: jar, latest: map[string]string{}}
}

func (j *recordingJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	for _, c := range cookies {
		value := c.Value
		// Client code reads cookies URL-decoded (axios decodes XSRF-TOKEN before sending it).
		if decoded, err := url.QueryUnescape(value); err == nil {
			value = decoded
		}
		j.latest[c.Name] = value
	}
	j.mu.Unlock()
	j.Jar.SetCookies(u, cookies)
}

// Cookies returns the latest value of every cookie the client has received, by name. Empty
// for a client that doesn't use a recording jar.
func Cookies(client HttpClient) map[string]string {
	c, ok := client.(*http.Client)
	if !ok {
		return map[string]string{}
	}
	jar, ok := c.Jar.(*recordingJar)
	if !ok {
		return map[string]string{}
	}
	jar.mu.Lock()
	defer jar.mu.Unlock()
	out := make(map[string]string, len(jar.latest))
	for name, value := range jar.latest {
		out[name] = value
	}
	return out
}
