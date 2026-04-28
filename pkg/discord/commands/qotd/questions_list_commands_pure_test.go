package qotd

import (
	"strings"
	"testing"

	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
)

func TestDescribeResetDeckResultMentionsCurrentSlotPause(t *testing.T) {
	t.Parallel()

	message := describeResetDeckResult(applicationqotd.ResetDeckResult{
		OfficialPostsCleared:                  1,
		SuppressedCurrentSlotAutomaticPublish: true,
	}, "Default")

	if !strings.Contains(message, "Automatic publishing for the current slot is paused until you publish manually.") {
		t.Fatalf("expected reset description to mention the temporary publish pause, got %q", message)
	}
	if !strings.Contains(message, "cleared 1 QOTD publish record") {
		t.Fatalf("expected reset description to preserve the reset summary, got %q", message)
	}
}
