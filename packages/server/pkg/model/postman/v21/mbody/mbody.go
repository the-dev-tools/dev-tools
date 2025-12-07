//nolint:revive // exported
package mbody

const (
	ModeRaw        = "raw"
	ModeURLEncoded = "urlencoded"
	ModeFormData   = "formdata"
	ModeFile       = "file"
	ModeGraphQL    = "graphql"
)

type Body struct {
	Mode       string           `json:"mode"`
	Raw        string           `json:"raw,omitempty"`
	URLEncoded []BodyURLEncoded `json:"urlencoded,omitempty"`
	FormData   []BodyFormData   `json:"formdata,omitempty"`
	File       interface{}      `json:"file,omitempty"`
	GraphQL    interface{}      `json:"graphql,omitempty"`
	Disabled   bool             `json:"disabled,omitempty"`
	Options    BodyOptions      `json:"options,omitempty"`
}

type BodyOptions struct {
	Raw BodyOptionsRaw `json:"raw,omitempty"`
}

type BodyOptionsRaw struct {
	Language string `json:"language,omitempty"`
}

type BodyURLEncoded struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled"`
}

type BodyFormData struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled"`
}
