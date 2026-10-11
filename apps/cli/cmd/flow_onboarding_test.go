package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
)

// runNamedFlow runs `flow run <file> <flow>` and returns its error and JSON report.
func runNamedFlow(t *testing.T, flowFile, flow string) ([]model.FlowRunResult, error) {
	t.Helper()
	reportPath := filepath.Join(t.TempDir(), "report.json")
	prevFormats, prevQuiet := reportFormats, quietMode
	t.Cleanup(func() { reportFormats, quietMode = prevFormats, prevQuiet })
	reportFormats = []string{"json:" + reportPath}
	quietMode = true

	yamlflowRunCmd.SetContext(context.Background())
	err := yamlflowRunCmd.RunE(yamlflowRunCmd, []string{flowFile, flow})
	if _, statErr := os.Stat(reportPath); statErr != nil {
		return nil, err
	}
	return readReport(t, reportPath), err
}

// unsetAfter removes variables a test's .env loading set in the process environment.
func unsetAfter(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		t.Cleanup(func() { _ = os.Unsetenv(n) })
	}
}

// onboardingApp is a local app with a Secure session cookie (as many frameworks set it even
// in development), a login redirect and a signup endpoint that validates generated data.
func onboardingApp(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	ulidRe := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		loggedIn := false
		if c, err := r.Cookie("sid"); err == nil && c.Value == "s1" {
			loggedIn = true
		}
		switch r.URL.Path {
		case "/login":
			if r.Method == http.MethodPost {
				http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s1", Secure: true, HttpOnly: true, Path: "/"})
				http.Redirect(w, r, "/home", http.StatusSeeOther)
				return
			}
			_, _ = w.Write([]byte("<form>login</form>"))
		case "/home", "/old-home":
			if r.URL.Path == "/old-home" {
				http.Redirect(w, r, "/home", http.StatusMovedPermanently)
				return
			}
			if !loggedIn {
				http.Redirect(w, r, "/login?next=/home", http.StatusFound)
				return
			}
			_, _ = w.Write([]byte("home"))
		case "/users":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			switch {
			case !uuidRe.MatchString(body["id"]), !uuidRe.MatchString(r.URL.Query().Get("ref")):
				http.Error(w, "bad uuid", http.StatusBadRequest)
			case !ulidRe.MatchString(body["ulid"]), !ulidRe.MatchString(r.Header.Get("X-Request-Id")):
				http.Error(w, "bad ulid", http.StatusBadRequest)
			case !strings.Contains(body["email"], "@"), body["username"] == "":
				http.Error(w, "bad faker", http.StatusBadRequest)
			case !regexp.MustCompile(`^\d{4}-\d\d-\d\dT`).MatchString(body["at"]), !regexp.MustCompile(`^\d{10}$`).MatchString(body["ts"]):
				http.Error(w, "bad time", http.StatusBadRequest)
			default:
				w.WriteHeader(http.StatusCreated)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &requests
}

const onboardingFlow = `
workspace_name: Onboarding
flows:
  - name: Session
    steps:
      - request:
          name: Login
          method: POST
          url: '{{ #env:ONBOARD_E2E_URL }}/login'
          follow_redirects: false
          assertions:
            - response.status == 303
            - response.headers.Location == "/home"
      - request:
          name: Home
          depends_on: Login
          url: '{{ #env:ONBOARD_E2E_URL }}/old-home'
          assertions:
            - response.status == 200
            - response.url endsWith "/home"
            - response.redirects == 1
      - request:
          name: Signup
          depends_on: Home
          method: POST
          url: '{{ #env:ONBOARD_E2E_URL }}/users?ref={{ uuid() }}'
          headers:
            X-Request-Id: '{{ ulid() }}'
          body:
            type: json
            json:
              id: '{{ uuid() }}'
              ulid: '{{ ulid() }}'
              email: '{{ faker.email() }}'
              username: '{{ faker.username() }}'
              at: '{{ now() }}'
              ts: '{{ now().Unix() }}'
          assertions:
            - response.status == 201
`

// The Secure session cookie set over http://127.0.0.1 is sent back (Home is not redirected
// to /login), follow_redirects: false returns the 303, response.url and response.redirects
// report the followed redirect, the data helpers work in URL, header and body, and
// #env:ONBOARD_E2E_URL comes from the flow directory's .env.
func TestFlowRunOnboardingSession(t *testing.T) {
	app, _ := onboardingApp(t)
	dir := t.TempDir()
	flowFile := filepath.Join(dir, "flow.yaml")
	require.NoError(t, os.WriteFile(flowFile, []byte(onboardingFlow), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("ONBOARD_E2E_URL="+app+"\n"), 0o600))
	unsetAfter(t, "ONBOARD_E2E_URL")

	results, err := runNamedFlow(t, flowFile, "Session")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "success", strings.ToLower(results[0].Status), "%+v", results[0])
}

// Without the Secure-cookie exception the session would be lost, Home would redirect to
// /login, and this assertion is how a flow notices.
func TestFlowRunResponseURLCatchesALoginRedirect(t *testing.T) {
	app, _ := onboardingApp(t)
	dir := t.TempDir()
	flowFile := filepath.Join(dir, "flow.yaml")
	doc := strings.ReplaceAll(`
workspace_name: W
flows:
  - name: Anonymous
    steps:
      - request:
          name: Home
          url: APP/home
          assertions:
            - response.status == 200
            - '!(response.url contains "/login")'
`, "APP", app)
	require.NoError(t, os.WriteFile(flowFile, []byte(doc), 0o600))
	_, err := runNamedFlow(t, flowFile, "Anonymous")
	require.Error(t, err)
}

// Every missing #env: name is reported at once, before any request is sent; the process
// environment wins over .env.
func TestFlowRunMissingEnvFailsFast(t *testing.T) {
	app, requests := onboardingApp(t)
	dir := t.TempDir()
	flowFile := filepath.Join(dir, "flow.yaml")
	doc := `
workspace_name: W
flows:
  - name: Main
    steps:
      - request: { name: A, url: '{{ #env:ONBOARD_E2E_BASE }}/home' }
      - request: { name: B, depends_on: A, url: '{{ #env:ONBOARD_MISSING_ONE }}/x', headers: { X: '{{ #env:ONBOARD_MISSING_TWO }}' } }
  - name: Unrelated
    steps:
      - request: { name: C, url: '{{ #env:ONBOARD_NOT_IN_MAIN }}' }
`
	require.NoError(t, os.WriteFile(flowFile, []byte(doc), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("ONBOARD_E2E_BASE=http://from-dotenv.invalid\n"), 0o600))
	t.Setenv("ONBOARD_E2E_BASE", app)

	_, err := runNamedFlow(t, flowFile, "Main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing environment variables: ONBOARD_MISSING_ONE, ONBOARD_MISSING_TWO")
	require.NotContains(t, err.Error(), "ONBOARD_NOT_IN_MAIN")
	require.NotContains(t, err.Error(), "ONBOARD_E2E_BASE")
	require.Zero(t, requests.Load(), "no request is sent before the check")
	require.Equal(t, app, os.Getenv("ONBOARD_E2E_BASE"), "the environment wins over .env")
}
