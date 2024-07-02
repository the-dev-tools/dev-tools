package mcookie

type Cookie struct {
	Domain     string      `json:"domain"`
	Expires    string      `json:"expires,omitempty"`
	MaxAge     string      `json:"maxAge,omitempty"`
	HostOnly   bool        `json:"hostOnly,omitempty"`
	HTTPOnly   bool        `json:"httpOnly,omitempty"`
	Name       string      `json:"name,omitempty"`
	Path       string      `json:"path"`
	Secure     string      `json:"secure,omitempty"`
	Session    bool        `json:"session,omitempty"`
	Value      string      `json:"value,omitempty"`
	Extensions interface{} `json:"extensions,omitempty"`
}
