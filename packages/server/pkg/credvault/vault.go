// Package credvault provides encryption/decryption for credential secrets.
// It supports multiple algorithms via EncryptionType enum, with XChaCha20-Poly1305
// as the recommended default.
package credvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// KeySize is the required size for the master encryption key (256-bit).
	KeySize = 32
)

var (
	ErrInvalidKeySize     = errors.New("credvault: key must be 32 bytes")
	ErrCiphertextTooShort = errors.New("credvault: ciphertext too short")
	ErrUnsupportedType    = errors.New("credvault: unsupported encryption type")

	// defaultKey is a static all-zeros key used when no master key is configured.
	// This provides basic obfuscation (secrets are not stored in plaintext) but is NOT
	// cryptographically secure. In production, configure a proper master key via environment.
	// TODO(config): Master key will be loaded from secure configuration (env var or secret manager).
	defaultKey = [KeySize]byte{}
)

// Vault handles encryption and decryption of credential secrets.
type Vault struct {
	masterKey []byte
}

// NewDefault creates a Vault with a static all-zeros key.
// Good for obfuscation (not plaintext), but not cryptographically secure.
func NewDefault() *Vault {
	return &Vault{masterKey: defaultKey[:]}
}

// New creates a new Vault with the given master key.
// The key must be exactly 32 bytes (256-bit).
func New(masterKey []byte) (*Vault, error) {
	if len(masterKey) != KeySize {
		return nil, ErrInvalidKeySize
	}
	// Copy key to prevent external mutation
	keyCopy := make([]byte, KeySize)
	copy(keyCopy, masterKey)
	return &Vault{masterKey: keyCopy}, nil
}

// Encrypt encrypts plaintext using the specified encryption type.
// For EncryptionNone, returns the plaintext unchanged.
func (v *Vault) Encrypt(plaintext []byte, encType EncryptionType) ([]byte, error) {
	switch encType {
	case EncryptionNone:
		return plaintext, nil
	case EncryptionXChaCha20Poly1305:
		return v.encryptXChaCha20(plaintext)
	case EncryptionAES256GCM:
		return v.encryptAES256GCM(plaintext)
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedType, encType)
	}
}

// Decrypt decrypts ciphertext using the specified encryption type.
// For EncryptionNone, returns the ciphertext unchanged.
func (v *Vault) Decrypt(ciphertext []byte, encType EncryptionType) ([]byte, error) {
	switch encType {
	case EncryptionNone:
		return ciphertext, nil
	case EncryptionXChaCha20Poly1305:
		return v.decryptXChaCha20(ciphertext)
	case EncryptionAES256GCM:
		return v.decryptAES256GCM(ciphertext)
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedType, encType)
	}
}

// EncryptString is a convenience method for encrypting strings.
func (v *Vault) EncryptString(plaintext string, encType EncryptionType) ([]byte, error) {
	return v.Encrypt([]byte(plaintext), encType)
}

// DecryptString is a convenience method for decrypting to a string.
func (v *Vault) DecryptString(ciphertext []byte, encType EncryptionType) (string, error) {
	plaintext, err := v.Decrypt(ciphertext, encType)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// encryptXChaCha20 encrypts using XChaCha20-Poly1305.
// Output format: [24-byte nonce][ciphertext+tag]
func (v *Vault) encryptXChaCha20(plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(v.masterKey)
	if err != nil {
		return nil, fmt.Errorf("credvault: failed to create chacha20 cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize()) // 24 bytes for XChaCha20
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("credvault: failed to generate nonce: %w", err)
	}

	// Seal appends ciphertext to nonce
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptXChaCha20 decrypts XChaCha20-Poly1305 ciphertext.
func (v *Vault) decryptXChaCha20(ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(v.masterKey)
	if err != nil {
		return nil, fmt.Errorf("credvault: failed to create chacha20 cipher: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("credvault: decryption failed: %w", err)
	}
	return plaintext, nil
}

// encryptAES256GCM encrypts using AES-256-GCM.
// Output format: [12-byte nonce][ciphertext+tag]
func (v *Vault) encryptAES256GCM(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, fmt.Errorf("credvault: failed to create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("credvault: failed to create gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize()) // 12 bytes for GCM
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("credvault: failed to generate nonce: %w", err)
	}

	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptAES256GCM decrypts AES-256-GCM ciphertext.
func (v *Vault) decryptAES256GCM(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, fmt.Errorf("credvault: failed to create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("credvault: failed to create gcm: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("credvault: decryption failed: %w", err)
	}
	return plaintext, nil
}
