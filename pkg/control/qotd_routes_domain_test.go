package control

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
)

func TestQOTDActionRoutesResolveDiscordSessionForQOTDDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "publish now", path: "/v1/guilds/g1/qotd/actions/publish-now"},
		{name: "reconcile", path: "/v1/guilds/g1/qotd/actions/reconcile"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv, _ := newControlTestServer(t)
			srv.SetQOTDService(&applicationqotd.Service{})

			var gotGuildID string
			srv.SetDiscordSessionResolver(func(guildID string) (*discordgo.Session, error) {
				gotGuildID = guildID
				return nil, errors.New("discord unavailable")
			})

			rec := performHandlerJSONRequest(t, srv.httpServer.Handler, http.MethodPost, tc.path, nil)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("POST %s status=%d body=%q", tc.path, rec.Code, rec.Body.String())
			}
			if gotGuildID != "g1" {
				t.Fatalf("expected resolver guild g1, got %q", gotGuildID)
			}

			if !strings.Contains(rec.Body.String(), "failed to resolve discord session") {
				t.Fatalf("expected discord session resolution failure body, got %q", rec.Body.String())
			}
		})
	}
}
