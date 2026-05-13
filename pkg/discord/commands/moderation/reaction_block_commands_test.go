package moderation

import "testing"

func TestParseReactionBlockEmojiListSupportsCustomEmojiAndShortcodes(t *testing.T) {
	t.Parallel()

	emojis, err := parseReactionBlockEmojiList("<:skrunklyalice:987654321098765432> :x:")
	if err != nil {
		t.Fatalf("parseReactionBlockEmojiList() failed: %v", err)
	}
	if len(emojis) != 2 {
		t.Fatalf("expected two parsed emojis, got %+v", emojis)
	}
	if emojis[0].Kind != "custom" || emojis[0].Value != "987654321098765432" || emojis[0].Name != "skrunklyalice" {
		t.Fatalf("unexpected custom emoji parse result: %+v", emojis[0])
	}
	if emojis[1].Kind != "unicode" || emojis[1].Alias != ":x:" || emojis[1].Value == "" || emojis[1].Value == ":x:" {
		t.Fatalf("unexpected shortcode parse result: %+v", emojis[1])
	}
}

func TestReactionBlockCommandsCRUD(t *testing.T) {
	const (
		guildID       = "777777777777777777"
		ownerID       = "999999999999999999"
		reactorUserID = "222222222222222222"
		targetUserID  = "111111111111111111"
	)

	harness := newModerationCommandTestHarness(t, guildID, ownerID)

	setResp := harness.runSlash(t, reactionBlockSetSubCommandName,
		moderationUserOpt(reactionBlockReactorOptionName, reactorUserID),
		moderationUserOpt(reactionBlockTargetOptionName, targetUserID),
		moderationStringOpt(reactionBlockEmojisOptionName, "<:skrunklyalice:987654321098765432> :x:"),
	)
	assertModerationPublicContains(t, setResp, "Blocked reactions from <@222222222222222222> to <@111111111111111111> are now")

	cfg, err := harness.cm.ReactionBlockConfig(guildID)
	if err != nil {
		t.Fatalf("ReactionBlockConfig() failed after set: %v", err)
	}
	if got := len(cfg.EmojisForPair(reactorUserID, targetUserID)); got != 2 {
		t.Fatalf("expected two blocked emojis after set, got %d", got)
	}

	listResp := harness.runSlash(t, reactionBlockListSubCommandName,
		moderationUserOpt(reactionBlockReactorOptionName, reactorUserID),
		moderationUserOpt(reactionBlockTargetOptionName, targetUserID),
	)
	assertModerationEphemeralContains(t, listResp, "skrunklyalice")
	assertModerationEphemeralContains(t, listResp, ":x:")

	addResp := harness.runSlash(t, reactionBlockAddSubCommandName,
		moderationUserOpt(reactionBlockReactorOptionName, reactorUserID),
		moderationUserOpt(reactionBlockTargetOptionName, targetUserID),
		moderationStringOpt(reactionBlockEmojisOptionName, "<:blobwave:123456789012345678>"),
	)
	assertModerationPublicContains(t, addResp, "blobwave")

	cfg, err = harness.cm.ReactionBlockConfig(guildID)
	if err != nil {
		t.Fatalf("ReactionBlockConfig() failed after add: %v", err)
	}
	if got := len(cfg.EmojisForPair(reactorUserID, targetUserID)); got != 3 {
		t.Fatalf("expected three blocked emojis after add, got %d", got)
	}

	removeResp := harness.runSlash(t, reactionBlockRemoveSubCommandName,
		moderationUserOpt(reactionBlockReactorOptionName, reactorUserID),
		moderationUserOpt(reactionBlockTargetOptionName, targetUserID),
		moderationStringOpt(reactionBlockEmojisOptionName, ":x:"),
	)
	assertModerationPublicContains(t, removeResp, "skrunklyalice")

	cfg, err = harness.cm.ReactionBlockConfig(guildID)
	if err != nil {
		t.Fatalf("ReactionBlockConfig() failed after remove: %v", err)
	}
	if got := len(cfg.EmojisForPair(reactorUserID, targetUserID)); got != 2 {
		t.Fatalf("expected two blocked emojis after remove, got %d", got)
	}

	clearResp := harness.runSlash(t, reactionBlockClearSubCommandName,
		moderationUserOpt(reactionBlockReactorOptionName, reactorUserID),
		moderationUserOpt(reactionBlockTargetOptionName, targetUserID),
	)
	assertModerationPublicContains(t, clearResp, "Cleared all blocked reactions")

	cfg, err = harness.cm.ReactionBlockConfig(guildID)
	if err != nil {
		t.Fatalf("ReactionBlockConfig() failed after clear: %v", err)
	}
	if got := len(cfg.EmojisForPair(reactorUserID, targetUserID)); got != 0 {
		t.Fatalf("expected no blocked emojis after clear, got %d", got)
	}
}

func TestReactionBlockSetRejectsInvalidEmojiList(t *testing.T) {
	const (
		guildID       = "777777777777777777"
		ownerID       = "999999999999999999"
		reactorUserID = "222222222222222222"
		targetUserID  = "111111111111111111"
	)

	harness := newModerationCommandTestHarness(t, guildID, ownerID)
	resp := harness.runSlash(t, reactionBlockSetSubCommandName,
		moderationUserOpt(reactionBlockReactorOptionName, reactorUserID),
		moderationUserOpt(reactionBlockTargetOptionName, targetUserID),
		moderationStringOpt(reactionBlockEmojisOptionName, "not-an-emoji"),
	)
	assertModerationEphemeralContains(t, resp, "invalid emoji list")
}
