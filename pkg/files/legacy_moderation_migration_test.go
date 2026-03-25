package files

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBotConfigRoundTripDropsLegacyModerationFields(t *testing.T) {
	t.Parallel()

	const legacyConfig = `{
		"guilds": [
			{
				"guild_id": "g1",
				"rulesets": [
					{
						"id": "ruleset-a",
						"name": "Ruleset A",
						"enabled": true,
						"rules": [
							{
								"id": "rule-a",
								"name": "Rule A",
								"enabled": true,
								"lists": [
									{
										"id": "list-a",
										"name": "List A",
										"type": "custom",
										"blocked_keywords": ["spam"]
									}
								]
							}
						]
					}
				],
				"loose_rules": [
					{
						"id": "rule-b",
						"name": "Rule B",
						"enabled": true,
						"lists": [
							{
								"id": "list-b",
								"name": "List B",
								"type": "custom",
								"blocked_keywords": ["eggs"]
							}
						]
					}
				],
				"blocklist": ["legacy-word"]
			}
		]
	}`

	var cfg BotConfig
	if err := json.Unmarshal([]byte(legacyConfig), &cfg); err != nil {
		t.Fatalf("unmarshal legacy config: %v", err)
	}
	if len(cfg.Guilds) != 1 || cfg.Guilds[0].GuildID != "g1" {
		t.Fatalf("unexpected guilds after unmarshal: %+v", cfg.Guilds)
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal migrated config: %v", err)
	}

	output := string(encoded)
	for _, legacyField := range []string{`"rulesets"`, `"loose_rules"`, `"blocklist"`} {
		if strings.Contains(output, legacyField) {
			t.Fatalf("expected migrated config to drop %s, got %s", legacyField, output)
		}
	}
}
