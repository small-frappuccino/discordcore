package files

import (
	"errors"
	"testing"
)

func TestNormalizeReactionBlockConfigMergesPairsAndDedupesEmojis(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeReactionBlockConfig(ReactionBlockConfig{Rules: []ReactionBlockRuleConfig{
		{
			ReactorUserID: " 222222222222222222 ",
			TargetUserID:  " 111111111111111111 ",
			Emojis: []ReactionBlockEmojiConfig{
				{Kind: ReactionBlockEmojiKindCustom, Value: " 987654321098765432 ", Name: "skrunklytest"},
				{Kind: ReactionBlockEmojiKindUnicode, Value: "❌", Alias: " :X: "},
			},
		},
		{
			ReactorUserID: "222222222222222222",
			TargetUserID:  "111111111111111111",
			Emojis: []ReactionBlockEmojiConfig{
				{Kind: ReactionBlockEmojiKindUnicode, Value: "❌"},
				{Kind: ReactionBlockEmojiKindCustom, Value: "123456789012345678", Name: "blobwave"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("NormalizeReactionBlockConfig() failed: %v", err)
	}
	if len(normalized.Rules) != 1 {
		t.Fatalf("expected one merged rule, got %+v", normalized.Rules)
	}

	rule := normalized.Rules[0]
	if rule.ReactorUserID != "222222222222222222" {
		t.Fatalf("expected canonical reactor user id, got %q", rule.ReactorUserID)
	}
	if rule.TargetUserID != "111111111111111111" {
		t.Fatalf("expected canonical target user id, got %q", rule.TargetUserID)
	}
	if len(rule.Emojis) != 3 {
		t.Fatalf("expected three unique blocked emojis, got %+v", rule.Emojis)
	}
	if rule.Emojis[2].Alias != ":x:" {
		t.Fatalf("expected unicode alias to be normalized, got %+v", rule.Emojis[2])
	}
	if !normalized.BlocksEmojiForPair(
		"222222222222222222",
		"111111111111111111",
		ReactionBlockEmojiConfig{Kind: ReactionBlockEmojiKindCustom, Value: "987654321098765432"},
	) {
		t.Fatalf("expected custom emoji to match normalized rule, got %+v", normalized)
	}
	if !normalized.BlocksEmojiForPair(
		"222222222222222222",
		"111111111111111111",
		ReactionBlockEmojiConfig{Kind: ReactionBlockEmojiKindUnicode, Value: "❌"},
	) {
		t.Fatalf("expected unicode emoji to match normalized rule, got %+v", normalized)
	}
}

func TestSetReactionBlockConfigCanonicalizesAndReadsBack(t *testing.T) {
	t.Parallel()

	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, nil)

	err := mgr.SetReactionBlockConfig("g1", ReactionBlockConfig{Rules: []ReactionBlockRuleConfig{{
		ReactorUserID: " 222222222222222222 ",
		TargetUserID:  " 111111111111111111 ",
		Emojis: []ReactionBlockEmojiConfig{{
			Kind:  ReactionBlockEmojiKindCustom,
			Value: " 987654321098765432 ",
			Name:  "skrunklytest",
		}},
	}}})
	if err != nil {
		t.Fatalf("SetReactionBlockConfig() failed: %v", err)
	}

	cfg, err := mgr.ReactionBlockConfig("g1")
	if err != nil {
		t.Fatalf("ReactionBlockConfig() failed: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected one persisted rule, got %+v", cfg)
	}
	if cfg.Rules[0].ReactorUserID != "222222222222222222" || cfg.Rules[0].TargetUserID != "111111111111111111" {
		t.Fatalf("expected canonical persisted pair ids, got %+v", cfg.Rules[0])
	}
	if len(cfg.Rules[0].Emojis) != 1 {
		t.Fatalf("expected one persisted emoji, got %+v", cfg.Rules[0].Emojis)
	}
	if cfg.Rules[0].Emojis[0].Display() != "<:skrunklytest:987654321098765432>" {
		t.Fatalf("expected custom emoji display label, got %+v", cfg.Rules[0].Emojis[0])
	}
}

func TestSetReactionBlockConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, saveErr)

	err := mgr.SetReactionBlockConfig("g1", ReactionBlockConfig{Rules: []ReactionBlockRuleConfig{{
		ReactorUserID: "222222222222222222",
		TargetUserID:  "111111111111111111",
		Emojis: []ReactionBlockEmojiConfig{{
			Kind:  ReactionBlockEmojiKindUnicode,
			Value: "❌",
			Alias: ":x:",
		}},
	}}})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected guild config to remain intact, got %+v", cfg.Guilds)
	}
	if !cfg.Guilds[0].ReactionBlocks.IsZero() {
		t.Fatalf("expected reaction block config rollback, got %+v", cfg.Guilds[0].ReactionBlocks)
	}
}
