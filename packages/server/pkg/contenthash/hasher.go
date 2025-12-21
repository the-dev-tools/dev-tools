package contenthash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Hasher provides deterministic content hashing
type Hasher struct{}

// New creates a new Hasher
func New() *Hasher {
	return &Hasher{}
}

// HashStruct generates a deterministic SHA-256 hash of a struct.
// It uses JSON marshaling (which sorts map keys) to ensure consistency.
// Important: Pass a struct that ONLY contains the "Content" fields you want to deduct on.
func (h *Hasher) HashStruct(v any) (string, error) {
	// 1. Serialize to JSON (Go's stdlib json sorts map keys by default)
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal struct for hashing: %w", err)
	}

	// 2. Calculate SHA-256
	hash := sha256.Sum256(data)

	// 3. Return hex string
	return hex.EncodeToString(hash[:]), nil
}

// HashString is a helper for simple string hashing (like paths)
func (h *Hasher) HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// HashBytes is a helper for byte slices (like file content)
func (h *Hasher) HashBytes(b []byte) string {
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}