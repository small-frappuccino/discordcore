package files

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEncryptionSymmetric(t *testing.T) {
	os.Setenv("PASTEBIN_ENCRYPTION_KEY", "my-test-super-secret-key-12345")
	defer os.Unsetenv("PASTEBIN_ENCRYPTION_KEY")

	original := "hello secret credentials"
	cipher, err := Encrypt(original)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if cipher == original {
		t.Fatalf("cipher matches original, encryption failed to obfuscate")
	}

	decrypted, err := Decrypt(cipher)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != original {
		t.Errorf("decrypted string mismatch: got %q, want %q", decrypted, original)
	}
}

type testConfigContainer struct {
	Secret EncryptedString `json:"secret"`
}

func TestEncryptedStringJSON(t *testing.T) {
	os.Setenv("PASTEBIN_ENCRYPTION_KEY", "json-test-key")
	defer os.Unsetenv("PASTEBIN_ENCRYPTION_KEY")

	original := "my-secret-password-xyz"
	container := testConfigContainer{
		Secret: EncryptedString(original),
	}

	data, err := json.Marshal(container)
	if err != nil {
		t.Fatalf("failed to marshal container: %v", err)
	}

	// Verify it does not contain the raw secret in the JSON output
	if jsonStr := string(data); len(jsonStr) == 0 || jsonStr == original {
		t.Fatalf("marshalled JSON contains raw secret: %s", jsonStr)
	}

	var decoded testConfigContainer
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if string(decoded.Secret) != original {
		t.Errorf("unmarshalled secret mismatch: got %q, want %q", decoded.Secret, original)
	}
}

func TestEncryptedStringUnmarshalFallback(t *testing.T) {
	// If unmarshalling raw, unencrypted json, it should fallback to raw value.
	rawJSON := `{"secret": "plain-text-legacy.key"}`
	var decoded testConfigContainer
	if err := json.Unmarshal([]byte(rawJSON), &decoded); err != nil {
		t.Fatalf("failed to unmarshal unencrypted: %v", err)
	}

	if string(decoded.Secret) != "plain-text-legacy.key" {
		t.Errorf("unmarshalled plaintext fallback mismatch: got %q, want %q", decoded.Secret, "plain-text-legacy.key")
	}
}
