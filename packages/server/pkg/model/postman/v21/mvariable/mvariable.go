//nolint:revive // exported
package mvariable

type Variable struct {
	ID          string `json:"id,omitempty"`
	Key         string `json:"key,omitempty"`
	Type        string `json:"type,omitempty"`
	Name        string `json:"name,omitempty"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
	System      bool   `json:"system,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}
