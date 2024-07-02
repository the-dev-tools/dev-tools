package mbody

type Body struct {
	Mode       string      `json:"mode"`
	Raw        string      `json:"raw,omitempty"`
	URLEncoded interface{} `json:"urlencoded,omitempty"`
	FormData   interface{} `json:"formdata,omitempty"`
	File       interface{} `json:"file,omitempty"`
	GraphQL    interface{} `json:"graphql,omitempty"`
	Disabled   bool        `json:"disabled,omitempty"`
	Options    BodyOptions `json:"options,omitempty"`
}

type BodyOptions struct {
	Raw BodyOptionsRaw `json:"raw,omitempty"`
}

type BodyOptionsRaw struct {
	Language string `json:"language,omitempty"`
}
