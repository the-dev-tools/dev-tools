package node

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"

// cookiesVar is the template variable holding the cookies a flow has received so far:
// `{{ cookies.csrftoken }}`, or `{{ cookies["XSRF-TOKEN"] }}` for names that aren't identifiers.
const cookiesVar = "cookies"

// WithCookies adds the client's received cookies to a step's variables, unless the flow
// already defines a variable of that name.
func WithCookies(vars map[string]any, client httpclient.HttpClient) {
	if _, taken := vars[cookiesVar]; taken {
		return
	}
	received := httpclient.Cookies(client)
	if len(received) == 0 {
		return
	}
	cookies := make(map[string]any, len(received))
	for name, value := range received {
		cookies[name] = value
	}
	vars[cookiesVar] = cookies
}
