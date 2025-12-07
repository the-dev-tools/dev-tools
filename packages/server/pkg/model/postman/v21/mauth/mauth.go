//nolint:revive // exported
package mauth

type Auth struct {
	Version string
	Type    string `json:"type,omitempty"`

	// AuthParams is a map of auth type to auth parameters.
	APIKey []*AuthParam `json:"apikey,omitempty"`
	AWSV4  []*AuthParam `json:"awsv4,omitempty"`
	Basic  []*AuthParam `json:"basic,omitempty"`
	Bearer []*AuthParam `json:"bearer,omitempty"`
	Digest []*AuthParam `json:"digest,omitempty"`
	Hawk   []*AuthParam `json:"hawk,omitempty"`
	NoAuth []*AuthParam `json:"noauth,omitempty"`
	OAuth1 []*AuthParam `json:"oauth1,omitempty"`
	OAuth2 []*AuthParam `json:"oauth2,omitempty"`
	NTLM   []*AuthParam `json:"ntlm,omitempty"`
}

type AuthParam struct {
	Key   string `json:"key,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}
