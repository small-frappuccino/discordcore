package files

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeGuildModerationConfigRejectsDuplicateRulesetIDs(t *testing.T) {
	t.Parallel()

	_, err := NormalizeGuildModerationConfig(
		[]Ruleset{
			testRuleset("ruleset-a", "Rule A", "list-a"),
			testRuleset("  RULESET-A  ", "Rule B", "list-b"),
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected duplicate ruleset id validation error")
	}
	if !strings.Contains(err.Error(), "ruleset id must be unique") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeGuildModerationConfigRejectsDuplicateRuleIDs(t *testing.T) {
	t.Parallel()

	_, err := NormalizeGuildModerationConfig(
		[]Ruleset{
			{
				ID:      "ruleset-a",
				Name:    "Ruleset A",
				Enabled: true,
				Rules: []Rule{
					testRule("rule-a", "Rule A", "list-a"),
					testRule(" RULE-A ", "Rule B", "list-b"),
				},
			},
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected duplicate rule id validation error")
	}
	if !strings.Contains(err.Error(), "rule id must be unique") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeGuildModerationConfigRejectsDuplicateLooseRuleIDs(t *testing.T) {
	t.Parallel()

	_, err := NormalizeGuildModerationConfig(
		nil,
		[]Rule{
			testRule("rule-a", "Rule A", "list-a"),
			testRule(" rule-a ", "Rule B", "list-b"),
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected duplicate loose rule id validation error")
	}
	if !strings.Contains(err.Error(), "rule id must be unique") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeGuildModerationConfigRejectsDuplicateListIDs(t *testing.T) {
	t.Parallel()

	_, err := NormalizeGuildModerationConfig(
		[]Ruleset{
			{
				ID:      "ruleset-a",
				Name:    "Ruleset A",
				Enabled: true,
				Rules: []Rule{
					{
						ID:      "rule-a",
						Name:    "Rule A",
						Enabled: true,
						Lists: []List{
							testCustomList("list-a", "List A", "alpha"),
							testCustomList(" LIST-A ", "List B", "beta"),
						},
					},
				},
			},
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected duplicate list id validation error")
	}
	if !strings.Contains(err.Error(), "list id must be unique") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeGuildModerationConfigRejectsInvalidListType(t *testing.T) {
	t.Parallel()

	_, err := NormalizeGuildModerationConfig(
		[]Ruleset{
			{
				ID:      "ruleset-a",
				Name:    "Ruleset A",
				Enabled: true,
				Rules: []Rule{
					{
						ID:      "rule-a",
						Name:    "Rule A",
						Enabled: true,
						Lists: []List{
							{
								ID:   "list-a",
								Type: "hybrid",
								Name: "List A",
							},
						},
					},
				},
			},
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected invalid list type validation error")
	}
	if !strings.Contains(err.Error(), "list type must be native or custom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeGuildModerationConfigRejectsNativeAndCustomFieldMismatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		list List
		want string
	}{
		{
			name: "native requires native id",
			list: List{
				ID:   "list-a",
				Type: RuleTypeNative,
				Name: "List A",
			},
			want: "native list requires native_id",
		},
		{
			name: "native rejects blocked keywords",
			list: List{
				ID:              "list-a",
				Type:            RuleTypeNative,
				Name:            "List A",
				NativeID:        "discord-spam",
				BlockedKeywords: []string{"alpha"},
			},
			want: "native list cannot include blocked_keywords",
		},
		{
			name: "custom rejects native id",
			list: List{
				ID:       "list-a",
				Type:     RuleTypeCustom,
				Name:     "List A",
				NativeID: "discord-spam",
			},
			want: "custom list cannot include native_id",
		},
		{
			name: "custom requires blocked keywords",
			list: List{
				ID:   "list-a",
				Type: RuleTypeCustom,
				Name: "List A",
			},
			want: "custom list requires blocked_keywords",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeGuildModerationConfig(
				[]Ruleset{
					{
						ID:      "ruleset-a",
						Name:    "Ruleset A",
						Enabled: true,
						Rules: []Rule{
							{
								ID:      "rule-a",
								Name:    "Rule A",
								Enabled: true,
								Lists:   []List{tt.list},
							},
						},
					},
				},
				nil,
				nil,
			)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeGuildModerationConfigRejectsEmptyRulesetsAndRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rulesets []Ruleset
		loose    []Rule
		want     string
	}{
		{
			name: "ruleset requires rules",
			rulesets: []Ruleset{
				{
					ID:      "ruleset-a",
					Name:    "Ruleset A",
					Enabled: true,
				},
			},
			want: "ruleset must contain at least one rule",
		},
		{
			name: "rule requires lists",
			loose: []Rule{
				{
					ID:      "rule-a",
					Name:    "Rule A",
					Enabled: true,
				},
			},
			want: "rule must contain at least one list",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeGuildModerationConfig(tt.rulesets, tt.loose, nil)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeGuildModerationConfigNormalizesBlockedKeywordsAndBlocklist(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeGuildModerationConfig(
		[]Ruleset{
			{
				ID:      " ruleset-a ",
				Name:    " Ruleset A ",
				Enabled: true,
				Rules: []Rule{
					{
						ID:      " rule-a ",
						Name:    " Rule A ",
						Enabled: true,
						Lists: []List{
							{
								ID:              " list-a ",
								Type:            " CUSTOM ",
								Name:            " List A ",
								Description:     " first line\nsecond line ",
								BlockedKeywords: []string{" alpha ", "", "ALPHA", "beta"},
							},
						},
					},
				},
			},
		},
		nil,
		[]string{" spam ", "", "SPAM", " eggs "},
	)
	if err != nil {
		t.Fatalf("normalize moderation config: %v", err)
	}

	list := normalized.Rulesets[0].Rules[0].Lists[0]
	if normalized.Rulesets[0].ID != "ruleset-a" {
		t.Fatalf("expected trimmed ruleset id, got %+v", normalized.Rulesets[0])
	}
	if normalized.Rulesets[0].Name != "Ruleset A" {
		t.Fatalf("expected trimmed ruleset name, got %+v", normalized.Rulesets[0])
	}
	if normalized.Rulesets[0].Rules[0].ID != "rule-a" || normalized.Rulesets[0].Rules[0].Name != "Rule A" {
		t.Fatalf("expected trimmed rule fields, got %+v", normalized.Rulesets[0].Rules[0])
	}
	if list.Type != RuleTypeCustom {
		t.Fatalf("expected lowercase custom list type, got %+v", list)
	}
	if list.Description != "first line second line" {
		t.Fatalf("expected single-line description, got %+v", list)
	}
	if strings.Join(list.BlockedKeywords, "|") != "alpha|beta" {
		t.Fatalf("unexpected blocked keywords: %+v", list.BlockedKeywords)
	}
	if strings.Join(normalized.Blocklist, "|") != "spam|eggs" {
		t.Fatalf("unexpected blocklist: %+v", normalized.Blocklist)
	}
}

func TestConfigManagerSaveConfigRejectsInvalidModeration(t *testing.T) {
	t.Parallel()

	mgr := NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	mgr.config = &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				Rulesets: []Ruleset{
					testRuleset("ruleset-a", "Rule A", "list-a"),
					testRuleset("RULESET-A", "Rule B", "list-b"),
				},
			},
		},
	}

	err := mgr.SaveConfig()
	if err == nil {
		t.Fatal("expected save to fail on invalid moderation config")
	}
	if !strings.Contains(err.Error(), ErrValidationFailed) {
		t.Fatalf("expected validation failure, got %v", err)
	}
}

func TestConfigManagerSaveConfigNormalizesModerationCollections(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	mgr := NewConfigManagerWithPath(path)
	mgr.config = &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				Rulesets: []Ruleset{
					{
						ID:      " ruleset-a ",
						Name:    " Ruleset A ",
						Enabled: true,
						Rules: []Rule{
							{
								ID:      " rule-a ",
								Name:    " Rule A ",
								Enabled: true,
								Lists: []List{
									{
										ID:              " list-a ",
										Type:            " CUSTOM ",
										Name:            " List A ",
										BlockedKeywords: []string{" alpha ", "ALPHA", "beta"},
									},
								},
							},
						},
					},
				},
				Blocklist: []string{" spam ", "SPAM", " eggs "},
			},
		},
	}

	if err := mgr.SaveConfig(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	guild := mgr.config.Guilds[0]
	if got := guild.Rulesets[0].Rules[0].Lists[0].Type; got != RuleTypeCustom {
		t.Fatalf("expected normalized list type, got %q", got)
	}
	if got := strings.Join(guild.Rulesets[0].Rules[0].Lists[0].BlockedKeywords, "|"); got != "alpha|beta" {
		t.Fatalf("unexpected normalized blocked keywords: %q", got)
	}
	if got := strings.Join(guild.Blocklist, "|"); got != "spam|eggs" {
		t.Fatalf("unexpected normalized blocklist: %q", got)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	var persisted BotConfig
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode saved config: %v", err)
	}
	if got := strings.Join(persisted.Guilds[0].Rulesets[0].Rules[0].Lists[0].BlockedKeywords, "|"); got != "alpha|beta" {
		t.Fatalf("unexpected persisted blocked keywords: %q", got)
	}
	if got := strings.Join(persisted.Guilds[0].Blocklist, "|"); got != "spam|eggs" {
		t.Fatalf("unexpected persisted blocklist: %q", got)
	}
}

func testRuleset(rulesetID, ruleID, listID string) Ruleset {
	return Ruleset{
		ID:      rulesetID,
		Name:    "Ruleset " + rulesetID,
		Enabled: true,
		Rules: []Rule{
			testRule(ruleID, "Rule "+ruleID, listID),
		},
	}
}

func testRule(ruleID, ruleName, listID string) Rule {
	return Rule{
		ID:      ruleID,
		Name:    ruleName,
		Enabled: true,
		Lists: []List{
			testCustomList(listID, "List "+listID, "keyword-"+listID),
		},
	}
}

func testCustomList(listID, listName, keyword string) List {
	return List{
		ID:              listID,
		Type:            RuleTypeCustom,
		Name:            listName,
		BlockedKeywords: []string{keyword},
	}
}
