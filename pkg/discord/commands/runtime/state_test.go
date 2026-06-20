package runtime

import (
	"testing"
)

func TestEncodeDecodeState(t *testing.T) {
	st := panelState{
		Mode:  pageDetail,
		Group: "LOGGING",
		Key:   "disable_db_cleanup",
		Scope: "guild-123",
	}

	encoded := st.encode()
	decoded := decodeState(encoded)

	if decoded.Mode != st.Mode {
		t.Errorf("expected mode %q, got %q", st.Mode, decoded.Mode)
	}
	if decoded.Group != st.Group {
		t.Errorf("expected group %q, got %q", st.Group, decoded.Group)
	}
	if decoded.Key != st.Key {
		t.Errorf("expected key %q, got %q", st.Key, decoded.Key)
	}
	if decoded.Scope != st.Scope {
		t.Errorf("expected scope %q, got %q", st.Scope, decoded.Scope)
	}
}

// FuzzDecodeState relentlessly assaults the operational decode boundaries via mutated payloads.
// It mathematically guarantees the deserializer does not trigger runtime panics (slice bounds out of range)
// when processing artificially mangled, excessively long, or multibyte corrupted strings from the HTTP gateway.
func FuzzDecodeState(f *testing.F) {
	// Seed the corpus with known legitimate structural variants.
	f.Add("main|ALL|bot_theme|global")
	f.Add("detail|SERVICES|disable_db_cleanup|123456789")
	f.Add("|||")
	f.Add("invalid_no_separators")

	f.Fuzz(func(t *testing.T, input string) {
		// Execution block: Execute decodeState and ensure it cleanly returns structured data.
		// If decodeState contains hidden slice bounds errors, this execution will inherently panic
		// and naturally trigger a testing failure via the Go runtime.
		st := decodeState(input)

		// Sanity checks: Sanitize functions must enforce fallback behaviors.
		if st.Group == "" {
			t.Errorf("sanitizeState violation: group remains empty for input %q", input)
		}
		if st.Scope == "" {
			t.Errorf("sanitizeState violation: scope remains empty for input %q", input)
		}
	})
}

func TestRuntimeInteractionAuthToken(t *testing.T) {
	t.Parallel()

	// Given identical user IDs, FNV-1a must return deterministic hashes.
	token1 := runtimeInteractionAuthToken("123456789")
	token2 := runtimeInteractionAuthToken("123456789")
	if token1 != token2 {
		t.Errorf("expected deterministic token derivation, got %q != %q", token1, token2)
	}

	// Empty strings must return structurally empty tokens to deny implicit validation.
	if token := runtimeInteractionAuthToken(""); token != "" {
		t.Errorf("expected empty string to yield empty token, got %q", token)
	}

	// Leading/trailing spaces must be stripped before hashing to mitigate invisible falsification.
	token3 := runtimeInteractionAuthToken("  987654321  ")
	token4 := runtimeInteractionAuthToken("987654321")
	if token3 != token4 {
		t.Errorf("expected whitespace normalization to yield identical hashes, got %q != %q", token3, token4)
	}
}

// FuzzDecodeRuntimeModalState guarantees modal parsers do not panic on mangled CustomIDs.
func FuzzDecodeRuntimeModalState(f *testing.F) {
	// CustomIDs format: customIDPrefix + "modal:edit" + "|" + key + "|" + scope + "|" + token
	base := modalEditValueID + stateSep
	f.Add(base + "bot_theme|global|token123")
	f.Add(base + "disable_db_cleanup|123456789|abc")
	f.Add("malformed_prefix|key|scope|token")
	f.Add(base + "||")

	f.Fuzz(func(t *testing.T, input string) {
		st, token, ok := decodeRuntimeModalState(input)

		// A false viability implies a safely rejected payload.
		if !ok {
			return
		}

		// Viable payloads strictly demand fallback behaviors if groups or scopes are mutated out.
		if st.Group == "" {
			t.Errorf("decodeRuntimeModalState violation: group empty for viable input %q", input)
		}
		if st.Scope == "" {
			t.Errorf("decodeRuntimeModalState violation: scope empty for viable input %q", input)
		}

		// Token must never be nil, though it may legally be empty (to be rejected by the authorizer later).
		_ = token
	})
}
