//nolint:revive // exported
package expression

import (
	"os"
	"strings"
)

const (
	// FileRefPrefix is the prefix for file references in expressions.
	FileRefPrefix = "#file:"
	// EnvRefPrefix is the prefix for environment variable references.
	EnvRefPrefix = "#env:"
	// GCPRefPrefix is the prefix for GCP Secret Manager references.
	GCPRefPrefix = "#gcp:"
	// AWSRefPrefix is the prefix for AWS Secrets Manager references (future).
	AWSRefPrefix = "#aws:"
	// AzureRefPrefix is the prefix for Azure Key Vault references (future).
	AzureRefPrefix = "#azure:"
)

// IsFileReference checks if a string is a file reference (#file:/path).
func IsFileReference(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), FileRefPrefix)
}

// GetFilePath extracts the file path from a file reference.
// Returns empty string if not a file reference.
func GetFilePath(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, FileRefPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(s, FileRefPrefix))
}

// ReadFileContent reads the content of a file reference.
// Returns the file content as a string, or an error if the file cannot be read.
func ReadFileContent(fileRef string) (string, error) {
	path := GetFilePath(fileRef)
	if path == "" {
		return "", &FileReferenceError{Path: fileRef, Cause: ErrEmptyPath}
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: Intentional file inclusion for #file: references
	if err != nil {
		return "", &FileReferenceError{Path: path, Cause: err}
	}

	return string(data), nil
}

// IsEnvReference checks if a string is an environment variable reference (#env:VAR).
func IsEnvReference(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), EnvRefPrefix)
}

// GetEnvVarName extracts the environment variable name from a reference.
// Returns empty string if not an env reference.
func GetEnvVarName(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, EnvRefPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(s, EnvRefPrefix))
}

// ReadEnvVar reads the value of an environment variable reference.
// Returns the value and whether it was found.
func ReadEnvVar(envRef string) (string, error) {
	name := GetEnvVarName(envRef)
	if name == "" {
		return "", &EnvReferenceError{VarName: envRef, Cause: ErrEmptyPath}
	}

	value, ok := os.LookupEnv(name)
	if !ok {
		return "", &EnvReferenceError{VarName: name}
	}

	return value, nil
}

// ExtractVarKey extracts the variable key from a {{ key }} pattern.
// Returns the key without braces, or empty string if not a valid pattern.
func ExtractVarKey(s string) string {
	s = strings.TrimSpace(s)
	if !HasVars(s) {
		return ""
	}

	// Find the content between {{ and }}
	start := strings.Index(s, "{{")
	end := strings.Index(s, "}}")
	if start == -1 || end == -1 || end <= start+2 {
		return ""
	}

	return strings.TrimSpace(s[start+2 : end])
}

// IsVarPattern checks if a string is exactly a {{ key }} pattern (no surrounding text).
func IsVarPattern(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") &&
		strings.Count(s, "{{") == 1 && strings.Count(s, "}}") == 1
}

// IsSecretReference checks if a string is a cloud secret reference (#gcp:, #aws:, #azure:).
func IsSecretReference(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, GCPRefPrefix) ||
		strings.HasPrefix(s, AWSRefPrefix) ||
		strings.HasPrefix(s, AzureRefPrefix)
}

// ParseSecretReference parses a secret reference like "#gcp:path#fragment".
// Returns (provider, resourcePath, fragment).
func ParseSecretReference(s string) (provider, ref, fragment string) {
	s = strings.TrimSpace(s)

	switch {
	case strings.HasPrefix(s, GCPRefPrefix):
		provider = "gcp"
		s = strings.TrimPrefix(s, GCPRefPrefix)
	case strings.HasPrefix(s, AWSRefPrefix):
		provider = "aws"
		s = strings.TrimPrefix(s, AWSRefPrefix)
	case strings.HasPrefix(s, AzureRefPrefix):
		provider = "azure"
		s = strings.TrimPrefix(s, AzureRefPrefix)
	}

	// Split on last '#' for fragment
	if idx := strings.LastIndex(s, "#"); idx != -1 {
		return provider, s[:idx], s[idx+1:]
	}
	return provider, s, ""
}

// ExtractVarKeysFromMultiple extracts all unique variable keys from multiple strings.
func ExtractVarKeysFromMultiple(strs ...string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, s := range strs {
		keys := ExtractVarRefs(s)
		for _, key := range keys {
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				result = append(result, key)
			}
		}
	}

	return result
}
