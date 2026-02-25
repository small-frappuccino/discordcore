package logging

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func newNotificationTestSession(t *testing.T, handler http.HandlerFunc) *discordgo.Session {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldChannels := discordgo.EndpointChannels
	oldWebhooks := discordgo.EndpointWebhooks
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	discordgo.EndpointWebhooks = discordgo.EndpointAPI + "webhooks/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointChannels = oldChannels
		discordgo.EndpointWebhooks = oldWebhooks
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session
}

func TestSendMemberLeaveNotification_UnknownServerTimeRendersNA(t *testing.T) {
	type field struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type embed struct {
		Fields []field `json:"fields"`
	}
	type messagePayload struct {
		Embeds []embed `json:"embeds"`
	}

	var captured messagePayload
	session := newNotificationTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/channels/c1/messages") {
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "m1",
				"channel_id": "c1",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})

	sender := NewNotificationSender(session)
	err := sender.SendMemberLeaveNotification(
		"c1",
		&discordgo.GuildMemberRemove{
			Member: &discordgo.Member{
				User: &discordgo.User{ID: "u1", Username: "user-1"},
			},
		},
		unknownServerTimeSentinel,
		0,
	)
	if err != nil {
		t.Fatalf("send member leave notification: %v", err)
	}

	if len(captured.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(captured.Embeds))
	}
	if len(captured.Embeds[0].Fields) == 0 {
		t.Fatalf("expected at least one field in embed")
	}

	found := false
	for _, f := range captured.Embeds[0].Fields {
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
