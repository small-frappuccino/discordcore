package notifications

import (
	"log/slog"
	"testing"
)

type mockPublisher struct {
	embeds []*Embed
}

func (m *mockPublisher) SendEmbed(channelID string, embed *Embed) error {
	m.embeds = append(m.embeds, embed)
	return nil
}

func TestSendMemberLeaveNotification_UnknownServerTimeRendersNA(t *testing.T) {
	pub := &mockPublisher{}
	sender := NewNotificationSender(pub, slog.Default())

	err := sender.SendMemberLeaveNotification(
		"c1",
		&MemberLeave{
			User: &User{ID: "u1", Username: "user-1"},
		},
		-1,
		0,
	)
	if err != nil {
		t.Fatalf("send member leave notification: %v", err)
	}

	if len(pub.embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(pub.embeds))
	}

	if len(pub.embeds[0].Fields) == 0 {
		t.Fatalf("expected at least one field in embed")
	}

	found := false
	for _, f := range pub.embeds[0].Fields {
		if f.Name == "Time on Server" {
			found = true
			if f.Value != "N/A" {
				t.Fatalf("expected 'N/A' for unknown server time, got %q", f.Value)
			}
		}
	}
	if !found {
		t.Fatalf("expected Time on Server field to be present")
	}
}
