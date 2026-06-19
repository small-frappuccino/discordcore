package files

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// TokenHash returns a 16-character SHA-256 hash of the token for deduplication.
func TokenHash(token string) string {
	if token == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:16])
}

// getEncryptionKey derives a 32-byte key from environment variables.
func getEncryptionKey() []byte {
	keys := []string{
		"PASTEBIN_ENCRYPTION_KEY",
		"DISCORDCORE_TOKEN",
		"DISCORD_TOKEN",
		"BOT_TOKEN",
	}
	var secret string
	for _, k := range keys {
		if val := os.Getenv(k); val != "" {
			secret = val
			break
		}
	}
	if secret == "" {
		// Fallback for testing/dev environments.
		secret = "discordcore-default-fallback-salt-super-secret-key-12345"
	}
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// Encrypt encrypts plainText using AES-GCM and returns a base64 encoded ciphertext.
func Encrypt(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("Encrypt: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("Encrypt: %w", err)
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("Encrypt: %w", err)
	}
	cipherText := aesGCM.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt decrypts a base64 encoded ciphertext using AES-GCM.
func Decrypt(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, actualCipherText := data[:nonceSize], data[nonceSize:]
	plainText, err := aesGCM.Open(nil, nonce, actualCipherText, nil)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	return string(plainText), nil
}

// EncryptedString represents a string that is transparently encrypted/decrypted
// when marshaling/unmarshaling JSON.
type EncryptedString string

// MarshalJSON encrypts the value before marshaling.
func (es EncryptedString) MarshalJSON() ([]byte, error) {
	enc, err := Encrypt(string(es))
	if err != nil {
		return nil, fmt.Errorf("EncryptedString.MarshalJSON: %w", err)
	}
	return json.Marshal(enc)
}

// UnmarshalJSON decrypts the base64 ciphertext during unmarshaling.
// If decryption fails, it falls back to storing the raw string, ensuring backwards
// compatibility and resilience against missing keys.
func (es *EncryptedString) UnmarshalJSON(data []byte) error {
	var val string
	if err := json.Unmarshal(data, &val); err != nil {
		return fmt.Errorf("EncryptedString.UnmarshalJSON: %w", err)
	}
	dec, err := Decrypt(val)
	if err != nil {
		// If the fallback value doesn't contain a dot, it's not a valid Discord
		// token and is likely an encrypted payload that failed to decrypt.
		// Dropping it prevents 4004 Auth Failures from passing ciphertext to the gateway.
		if !strings.Contains(val, ".") {
			*es = ""
			return nil
		}
		// Decryption failed. Fallback to raw string value.
		*es = EncryptedString(val)
		return nil
	}
	*es = EncryptedString(dec)
	return nil
}
