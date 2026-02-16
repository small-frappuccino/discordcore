package logging

import (
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

func TestCreateAvatarChangeEmbed_UsesSingleEmbedWithNewAvatarAndOldLink(t *testing.T) {
	ns := &NotificationSender{}
	change := files.AvatarChange{
		UserID:    "1334727960959385752",
		Username:  "bluetowels",
		OldAvatar: "oldhash",
		NewAvatar: "newhash",
		Timestamp: time.Date(2026, 2, 16, 14, 48, 0, 0, time.UTC),
	}

	embed := ns.createAvatarChangeEmbed(change)
	if embed == nil {
		t.Fatalf("expected embed, got nil")
	}

	if embed.Author == nil || embed.Author.Name != "Avatar Updated" {
		t.Fatalf("expected author name 'Avatar Updated', got %+v", embed.Author)
	}
	if embed.Color != theme.AvatarChange() {
		t.Fatalf("expected color %d, got %d", theme.AvatarChange(), embed.Color)
	}
	if embed.Thumbnail == nil {
		t.Fatalf("expected thumbnail with new avatar")
	}

	expectedNew := "https://cdn.discordapp.com/avatars/1334727960959385752/newhash.png?size=128"
	if embed.Thumbnail.URL != expectedNew {
		t.Fatalf("expected new avatar thumbnail %q, got %q", expectedNew, embed.Thumbnail.URL)
	}

	if len(embed.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(embed.Fields))
	}
	if embed.Fields[0].Name != "User" {
		t.Fatalf("expected first field 'User', got %q", embed.Fields[0].Name)
	}
	if embed.Fields[1].Name != "Previous Avatar" {
		t.Fatalf("expected second field 'Previous Avatar', got %q", embed.Fields[1].Name)
	}

	expectedOld := "https://cdn.discordapp.com/avatars/1334727960959385752/oldhash.png?size=128"
	if !strings.Contains(embed.Fields[1].Value, expectedOld) {
		t.Fatalf("expected previous avatar link to contain %q, got %q", expectedOld, embed.Fields[1].Value)
	}
}
