package files

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGuildConfigLegacyMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		jsonInput  string
		wantTokens []string
	}{
		{
			name:       "migrates bot_instance_id",
			jsonInput:  `{"guild_id": "g1", "bot_instance_id": "main"}`,
			wantTokens: []string{"main"},
		},
		{
			name:       "migrates domain_bot_instance_ids",
			jsonInput:  `{"guild_id": "g2", "domain_bot_instance_ids": {"qotd": "custom", "moderation": "admin"}}`,
			wantTokens: []string{"custom", "admin"},
		},
		{
			name:       "combines both legacy fields",
			jsonInput:  `{"guild_id": "g3", "bot_instance_id": "main", "domain_bot_instance_ids": {"qotd": "custom"}}`,
			wantTokens: []string{"main", "custom"},
		},
		{
			name:       "normalizes legacy names",
			jsonInput:  `{"guild_id": "g4", "bot_instance_id": " Alice ", "domain_bot_instance_ids": {"qotd": "Bob"}}`,
			wantTokens: []string{"Alice", "Bob"},
		},
		{
			name:       "ignores empty fields",
			jsonInput:  `{"guild_id": "g5"}`,
			wantTokens: nil,
		},
		{
			name:       "does not overwrite existing canonical tokens",
			jsonInput:  `{"guild_id": "g6", "bot_instance_id": "main", "bot_instance_tokens": {"main": "existing-token"}}`,
			wantTokens: []string{"main"}, // we should check that "main" has "existing-token"
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gc GuildConfig
			if err := json.Unmarshal([]byte(tc.jsonInput), &gc); err != nil {
				t.Fatalf("UnmarshalJSON failed: %v", err)
			}

			if len(tc.wantTokens) == 0 {
				if len(gc.BotInstanceTokens) > 0 {
					t.Fatalf("expected empty BotInstanceTokens, got %+v", gc.BotInstanceTokens)
				}
				return
			}

			if len(gc.BotInstanceTokens) != len(tc.wantTokens) {
				t.Fatalf("expected %d tokens, got %d: %+v", len(tc.wantTokens), len(gc.BotInstanceTokens), gc.BotInstanceTokens)
			}

			for _, want := range tc.wantTokens {
				val, exists := gc.BotInstanceTokens[want]
				if !exists {
					t.Errorf("expected BotInstanceTokens to contain key %q", want)
				}
				if tc.name == "does not overwrite existing canonical tokens" && want == "main" {
					if string(val) != "existing-token" {
						t.Errorf("expected token to remain 'existing-token', got %q", val)
					}
				} else if strings.HasPrefix(tc.name, "migrates legacy") || tc.name == "migrates mixed legacy tokens" {
					// We injected actual tokens in these tests
				} else if string(val) != "" {
					t.Errorf("expected empty token for migrated key %q, got %q", want, val)
				}
			}

			// Validate that legacy fields are NOT marshaled back at the top level
			marshaled, err := json.Marshal(gc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var m map[string]interface{}
			if err := json.Unmarshal(marshaled, &m); err != nil {
				t.Fatalf("Unmarshal of marshaled json failed: %v", err)
			}

			if _, ok := m["bot_instance_id"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'bot_instance_id'")
			}
			if _, ok := m["domain_bot_instance_ids"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'domain_bot_instance_ids'")
			}
			if _, ok := m["main"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'main'")
			}
			if _, ok := m["custom"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'custom'")
			}
		})
	}
}
