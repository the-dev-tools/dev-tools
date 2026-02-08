package secretresolver

import "strings"

// ParseSecretRef splits a secret reference into its resource path and optional fragment.
// Input: "projects/p/secrets/s/versions/latest#client_secret"
// Returns: ("projects/p/secrets/s/versions/latest", "client_secret")
// If no fragment is present, fragment is empty.
func ParseSecretRef(ref string) (path, fragment string) {
	if idx := strings.LastIndex(ref, "#"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}
