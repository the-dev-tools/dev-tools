//nolint:revive // exported
package credvault

// EncryptionType specifies the algorithm used to encrypt credential secrets.
type EncryptionType = int8

const (
	EncryptionNone              EncryptionType = 0 // Plaintext (no encryption)
	EncryptionXChaCha20Poly1305 EncryptionType = 1 // XChaCha20-Poly1305 AEAD (recommended)
	EncryptionAES256GCM         EncryptionType = 2 // AES-256-GCM AEAD
)
