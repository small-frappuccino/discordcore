package logging

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func findField(fields []*discordgo.MessageEmbedField, name string) *discordgo.MessageEmbedField {
	for _, f := range fields {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}

func TestBuildAutomodEmbed_MemberProfile_OmitsChannelField(t *testing.T) {
	t.Parallel()

	embed := buildAutomodEmbed(&discordgo.AutoModerationActionExecution{
		GuildID:         "g1",
		RuleID:          "r1",
		UserID:          "u1",
		RuleTriggerType: automodTriggerMemberProfile,
		MatchedKeyword:  "ass",
		MatchedContent:  "BigAss",
		Action:          discordgo.AutoModerationAction{Type: discordgo.AutoModerationActionType(automodActionBlockMemberInteraction)},
	})

	if embed.Title != "AutoMod • Member Profile Quarantined" {
		t.Fatalf("unexpected title: %q", embed.Title)
	}
	if findField(embed.Fields, "Channel") != nil {
		t.Fatal("Member Profile embed must not include a Channel field")
	}
	if findField(embed.Fields, "Member") == nil {
		t.Fatal("Member Profile embed must include a Member field")
	}
	if f := findField(embed.Fields, "Trigger"); f == nil || f.Value != "Member profile" {
		t.Fatalf("expected Trigger 'Member profile', got %+v", f)
	}
	// The Action field is intentionally omitted: per-action events are
	// coalesced into one embed and the description already conveys that
	// the user has been restricted. See the package-level "AutoMod
	// logging" comment block for the debug-mode hook.
	if findField(embed.Fields, "Action") != nil {
		t.Fatal("Member Profile embed must not include an Action field (coalesced per-violation)")
	}
	if f := findField(embed.Fields, "Offending fragment"); f == nil || !strings.Contains(f.Value, "BigAss") {
		t.Fatalf("expected Offending fragment containing 'BigAss', got %+v", f)
	}
}

func TestBuildAutomodEmbed_Message_IncludesChannelAndJumpLink(t *testing.T) {
	t.Parallel()

	embed := buildAutomodEmbed(&discordgo.AutoModerationActionExecution{
		GuildID:         "g1",
		ChannelID:       "c1",
		MessageID:       "m1",
		RuleID:          "r1",
		UserID:          "u1",
		RuleTriggerType: automodTriggerKeyword,
		MatchedKeyword:  "spam",
		Content:         "hello spam world",
	})

	if embed.Title != "AutoMod • Message Blocked" {
		t.Fatalf("unexpected title: %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "https://discord.com/channels/g1/c1/m1") {
		t.Fatalf("expected jump link in description, got %q", embed.Description)
	}
	if f := findField(embed.Fields, "Channel"); f == nil {
		t.Fatal("Message embed must include a Channel field")
	}
	if f := findField(embed.Fields, "Trigger"); f == nil || f.Value != "Keyword" {
		t.Fatalf("expected Trigger 'Keyword', got %+v", f)
	}
	if f := findField(embed.Fields, "Excerpt"); f == nil || !strings.Contains(f.Value, "hello spam world") {
		t.Fatalf("expected Excerpt containing the content, got %+v", f)
	}
}

func TestBuildAutomodEmbed_Message_NoJumpLinkWhenMessageIDMissing(t *testing.T) {
	t.Parallel()

	embed := buildAutomodEmbed(&discordgo.AutoModerationActionExecution{
		GuildID:         "g1",
		ChannelID:       "c1",
		RuleID:          "r1",
		UserID:          "u1",
		RuleTriggerType: automodTriggerSpam,
	})

	if strings.Contains(embed.Description, "Jump to message") {
		t.Fatalf("expected no jump link without MessageID, got %q", embed.Description)
	}
}

func TestBuildAutomodEmbed_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	embed := buildAutomodEmbed(&discordgo.AutoModerationActionExecution{
		GuildID:         "g1",
		RuleID:          "",
		UserID:          "u1",
		RuleTriggerType: automodTriggerMemberProfile,
	})

	if findField(embed.Fields, "Rule ID") != nil {
		t.Fatal("Rule ID field must be omitted when RuleID is empty")
	}
	if findField(embed.Fields, "Matched keyword") != nil {
		t.Fatal("Matched keyword field must be omitted when MatchedKeyword is empty")
	}
	if findField(embed.Fields, "Offending fragment") != nil {
		t.Fatal("Offending fragment field must be omitted when no content is present")
	}
}

func TestBuildAutomodEmbed_TruncatesLongExcerpt(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", automodExcerptMaxLen+50)
	embed := buildAutomodEmbed(&discordgo.AutoModerationActionExecution{
		GuildID:         "g1",
		ChannelID:       "c1",
		RuleID:          "r1",
		UserID:          "u1",
		RuleTriggerType: automodTriggerKeyword,
		Content:         long,
	})

	f := findField(embed.Fields, "Excerpt")
	if f == nil {
		t.Fatal("expected Excerpt field")
	}
	if !strings.HasSuffix(strings.TrimSuffix(f.Value, "```"), "...") {
		t.Fatalf("expected truncated excerpt ending with '...', got %q", f.Value)
	}
}
