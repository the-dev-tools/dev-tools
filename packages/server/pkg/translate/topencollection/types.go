// Package topencollection parses Bruno's OpenCollection YAML format and converts
// it into DevTools internal models. This package is isolated from the rest of
// DevTools and can be removed without affecting other functionality.
package topencollection

// OpenCollectionRoot represents the top-level opencollection.yml file.
type OpenCollectionRoot struct {
	OpenCollection string             `yaml:"opencollection"`
	Info           OpenCollectionInfo `yaml:"info"`
}

// OpenCollectionInfo contains collection metadata.
type OpenCollectionInfo struct {
	Name    string                 `yaml:"name"`
	Summary string                 `yaml:"summary,omitempty"`
	Version string                 `yaml:"version,omitempty"`
	Authors []OpenCollectionAuthor `yaml:"authors,omitempty"`
}

// OpenCollectionAuthor represents a collection author.
type OpenCollectionAuthor struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
}

// OCRequest represents a single request file in the OpenCollection format.
type OCRequest struct {
	Info     OCRequestInfo `yaml:"info"`
	HTTP     *OCHTTPBlock  `yaml:"http,omitempty"`
	Runtime  *OCRuntime    `yaml:"runtime,omitempty"`
	Settings *OCSettings   `yaml:"settings,omitempty"`
	Docs     string        `yaml:"docs,omitempty"`
}

// OCRequestInfo contains request metadata.
type OCRequestInfo struct {
	Name string   `yaml:"name"`
	Type string   `yaml:"type"` // http, graphql, ws, grpc
	Seq  int      `yaml:"seq,omitempty"`
	Tags []string `yaml:"tags,omitempty"`
}

// OCHTTPBlock contains the HTTP request definition.
type OCHTTPBlock struct {
	Method  string     `yaml:"method"`
	URL     string     `yaml:"url"`
	Headers []OCHeader `yaml:"headers,omitempty"`
	Params  []OCParam  `yaml:"params,omitempty"`
	Body    *OCBody    `yaml:"body,omitempty"`
	Auth    *OCAuth    `yaml:"auth,omitempty"`
}

// OCHeader represents an HTTP header.
type OCHeader struct {
	Name     string `yaml:"name"`
	Value    string `yaml:"value"`
	Disabled bool   `yaml:"disabled,omitempty"`
}

// OCParam represents a request parameter (query or path).
type OCParam struct {
	Name     string `yaml:"name"`
	Value    string `yaml:"value"`
	Type     string `yaml:"type"` // query, path
	Disabled bool   `yaml:"disabled,omitempty"`
}

// OCBody represents the request body.
type OCBody struct {
	Type string      `yaml:"type"` // json, xml, text, form-urlencoded, multipart-form, graphql, none
	Data interface{} `yaml:"data"` // string for raw, []OCFormField for forms
}

// OCFormField represents a form field in multipart or urlencoded bodies.
type OCFormField struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	Disabled    bool   `yaml:"disabled,omitempty"`
	ContentType string `yaml:"contentType,omitempty"`
}

// OCAuth represents authentication configuration.
type OCAuth struct {
	Type      string `yaml:"type"`               // none, inherit, basic, bearer, apikey
	Token     string `yaml:"token,omitempty"`     // bearer
	Username  string `yaml:"username,omitempty"`  // basic
	Password  string `yaml:"password,omitempty"`  // basic
	Key       string `yaml:"key,omitempty"`       // apikey
	Value     string `yaml:"value,omitempty"`     // apikey
	Placement string `yaml:"placement,omitempty"` // apikey: header, query
}

// OCRuntime contains runtime configuration (scripts, assertions, actions).
type OCRuntime struct {
	Scripts    []OCScript    `yaml:"scripts,omitempty"`
	Assertions []OCAssertion `yaml:"assertions,omitempty"`
	Actions    []OCAction    `yaml:"actions,omitempty"`
}

// OCScript represents a pre/post request script.
type OCScript struct {
	Type string `yaml:"type"` // pre-request, post-response
	Code string `yaml:"code"`
}

// OCAssertion represents a test assertion.
type OCAssertion struct {
	Expression string `yaml:"expression"`
	Operator   string `yaml:"operator"`
	Value      string `yaml:"value,omitempty"`
}

// OCAction represents a runtime action.
type OCAction struct {
	Type  string `yaml:"type"`
	Key   string `yaml:"key,omitempty"`
	Value string `yaml:"value,omitempty"`
}

// OCSettings contains request-level settings.
type OCSettings struct {
	EncodeUrl       *bool `yaml:"encodeUrl,omitempty"`
	Timeout         *int  `yaml:"timeout,omitempty"`
	FollowRedirects *bool `yaml:"followRedirects,omitempty"`
	MaxRedirects    *int  `yaml:"maxRedirects,omitempty"`
}

// OCEnvironment represents an environment file.
type OCEnvironment struct {
	Name      string          `yaml:"name"`
	Variables []OCEnvVariable `yaml:"variables"`
}

// OCEnvVariable represents an environment variable.
type OCEnvVariable struct {
	Name    string `yaml:"name"`
	Value   string `yaml:"value"`
	Enabled *bool  `yaml:"enabled,omitempty"`
	Secret  *bool  `yaml:"secret,omitempty"`
}

// OCFolder represents a folder.yml file in a directory.
type OCFolder struct {
	Name string `yaml:"name,omitempty"`
	Seq  int    `yaml:"seq,omitempty"`
}
