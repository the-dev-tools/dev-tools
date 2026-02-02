package credvault

import (
	"bytes"
	"testing"
)

func TestVault_InvalidKeySize(t *testing.T) {
	_, err := New([]byte("too short"))
	if err != ErrInvalidKeySize {
		t.Errorf("expected ErrInvalidKeySize, got %v", err)
	}
}

func TestVault_EncryptDecrypt_XChaCha20(t *testing.T) {
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i)
	}

	vault, err := New(key)
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"short", "hello"},
		{"api_key", "sk-1234567890abcdefghijklmnopqrstuvwxyz"},
		{"unicode", "🔐 secret key 密钥"},
		{"long", string(make([]byte, 10000))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := vault.EncryptString(tc.plaintext, EncryptionXChaCha20Poly1305)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}

			// Ciphertext should be different from plaintext
			if tc.plaintext != "" && bytes.Equal(ciphertext, []byte(tc.plaintext)) {
				t.Error("ciphertext equals plaintext")
			}

			// Ciphertext should include nonce (24 bytes) + plaintext + tag (16 bytes)
			expectedMinLen := 24 + len(tc.plaintext) + 16
			if len(ciphertext) < expectedMinLen {
				t.Errorf("ciphertext too short: got %d, want >= %d", len(ciphertext), expectedMinLen)
			}

			decrypted, err := vault.DecryptString(ciphertext, EncryptionXChaCha20Poly1305)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}

			if decrypted != tc.plaintext {
				t.Errorf("decrypted mismatch: got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestVault_EncryptDecrypt_AES256GCM(t *testing.T) {
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i + 100)
	}

	vault, err := New(key)
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	plaintext := "sk-test-api-key-12345"
	ciphertext, err := vault.EncryptString(plaintext, EncryptionAES256GCM)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// AES-GCM nonce is 12 bytes, tag is 16 bytes
	expectedMinLen := 12 + len(plaintext) + 16
	if len(ciphertext) < expectedMinLen {
		t.Errorf("ciphertext too short: got %d, want >= %d", len(ciphertext), expectedMinLen)
	}

	decrypted, err := vault.DecryptString(ciphertext, EncryptionAES256GCM)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestVault_EncryptionNone(t *testing.T) {
	key := make([]byte, KeySize)
	vault, _ := New(key)

	plaintext := "not encrypted"
	result, err := vault.EncryptString(plaintext, EncryptionNone)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if string(result) != plaintext {
		t.Errorf("EncryptionNone should return plaintext unchanged")
	}
}

func TestVault_WrongKey_FailsDecrypt(t *testing.T) {
	key1 := make([]byte, KeySize)
	key2 := make([]byte, KeySize)
	key2[0] = 1 // Different key

	vault1, _ := New(key1)
	vault2, _ := New(key2)

	ciphertext, _ := vault1.EncryptString("secret", EncryptionXChaCha20Poly1305)

	_, err := vault2.DecryptString(ciphertext, EncryptionXChaCha20Poly1305)
	if err == nil {
		t.Error("expected decryption to fail with wrong key")
	}
}

func TestVault_TamperedCiphertext_FailsDecrypt(t *testing.T) {
	key := make([]byte, KeySize)
	vault, _ := New(key)

	ciphertext, _ := vault.EncryptString("secret", EncryptionXChaCha20Poly1305)

	// Tamper with ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err := vault.DecryptString(ciphertext, EncryptionXChaCha20Poly1305)
	if err == nil {
		t.Error("expected decryption to fail with tampered ciphertext")
	}
}

func TestVault_CiphertextTooShort(t *testing.T) {
	key := make([]byte, KeySize)
	vault, _ := New(key)

	_, err := vault.Decrypt([]byte("short"), EncryptionXChaCha20Poly1305)
	if err != ErrCiphertextTooShort {
		t.Errorf("expected ErrCiphertextTooShort, got %v", err)
	}
}

func TestVault_UniqueNonces(t *testing.T) {
	key := make([]byte, KeySize)
	vault, _ := New(key)

	plaintext := "same plaintext"
	ct1, _ := vault.EncryptString(plaintext, EncryptionXChaCha20Poly1305)
	ct2, _ := vault.EncryptString(plaintext, EncryptionXChaCha20Poly1305)

	if bytes.Equal(ct1, ct2) {
		t.Error("encrypting same plaintext should produce different ciphertexts (unique nonces)")
	}
}

func BenchmarkEncrypt_XChaCha20(b *testing.B) {
	key := make([]byte, KeySize)
	vault, _ := New(key)
	plaintext := []byte("sk-1234567890abcdefghijklmnopqrstuvwxyz")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = vault.Encrypt(plaintext, EncryptionXChaCha20Poly1305)
	}
}

func BenchmarkDecrypt_XChaCha20(b *testing.B) {
	key := make([]byte, KeySize)
	vault, _ := New(key)
	plaintext := []byte("sk-1234567890abcdefghijklmnopqrstuvwxyz")
	ciphertext, _ := vault.Encrypt(plaintext, EncryptionXChaCha20Poly1305)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = vault.Decrypt(ciphertext, EncryptionXChaCha20Poly1305)
	}
}
