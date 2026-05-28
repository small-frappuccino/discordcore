package roles

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type interactionResponseRecorder struct {
	mu        sync.Mutex
	responses []discordgo.InteractionResponse
}

func (r *interactionResponseRecorder) record(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *interactionResponseRecorder) last(t *testing.T) discordgo.InteractionResponse {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.responses) == 0 {
		t.Fatal("expected at least one interaction response")
	}
	return r.responses[len(r.responses)-1]
}

func newRolePanelTestSession(t *testing.T) (*discordgo.Session, *interactionResponseRecorder) {
	t.Helper()
	rec := &interactionResponseRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/callback") {
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			rec.record(resp)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/"
	t.Cleanup(func() { discordgo.EndpointAPI = oldAPI })

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session, rec
}

// newRolePanelTestComponentInteraction builds an interaction that
// targets one role-panel button. The interaction's Member.Roles is
// intentionally left empty because the default member-lookup ignores
// the snapshot and round-trips through s.GuildMember; tests express
// the user's current role state by stubbing memberLookup on the
// handler instead.
func newRolePanelTestComponentInteraction(guildID, userID, roleID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-roles-panel",
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionMessageComponent,
			GuildID: guildID,
			Member: &discordgo.Member{
				User: &discordgo.User{ID: userID},
			},
			Data: discordgo.MessageComponentInteractionData{
				CustomID:      rolePanelButtonCustomID(roleID),
				ComponentType: discordgo.ButtonComponent,
			},
		},
	}
}

// stubMemberHasRole returns a memberLookup that always reports
// hasRole. Tests use this to express the user's current role state
// without exercising the real Discord round-trip.
func stubMemberHasRole(hasRole bool) func(*discordgo.Session, *discordgo.InteractionCreate, string) (bool, error) {
	return func(*discordgo.Session, *discordgo.InteractionCreate, string) (bool, error) {
		return hasRole, nil
	}
}

func newRolePanelTestConfigManager(t *testing.T, guildID, roleID string) *files.ConfigManager {
	t.Helper()
	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	if err := cm.UpsertRolePanelButton(guildID, "pings", files.RolePanelButtonConfig{
		RoleID: roleID,
		Label:  "Test",
	}); err != nil {
		t.Fatalf("upsert button: %v", err)
	}
	return cm
}

func runRolePanelComponent(t *testing.T, cm *files.ConfigManager, session *discordgo.Session, i *discordgo.InteractionCreate, handler *rolePanelComponentHandler) error {
	t.Helper()
	ctx := &core.Context{
		Session:     session,
		Interaction: i,
		Config:      cm,
		GuildID:     i.GuildID,
		UserID:      rolePanelInteractionUserID(i),
		RouteKey: core.InteractionRouteKey{
			Kind:     core.InteractionKindComponent,
			Path:     rolePanelComponentRouteID,
			CustomID: i.MessageComponentData().CustomID,
		},
	}
	return handler.HandleComponent(ctx)
}

func TestRolePanelComponentAddsRoleWhenMissing(t *testing.T) {
	guildID := "guild-add"
	userID := "user-add"
	roleID := "1380646673482518639"
	cm := newRolePanelTestConfigManager(t, guildID, roleID)
	session, rec := newRolePanelTestSession(t)
	interaction := newRolePanelTestComponentInteraction(guildID, userID, roleID)

	var calls struct {
		add, remove int
		lastRoleID  string
	}
	handler := newRolePanelComponentHandler(cm)
	handler.memberLookup = stubMemberHasRole(false)
	handler.addRole = func(_ *discordgo.Session, gid, uid, rid string) error {
		calls.add++
		calls.lastRoleID = rid
		if gid != guildID || uid != userID || rid != roleID {
			t.Fatalf("unexpected add call: %s/%s/%s", gid, uid, rid)
		}
		return nil
	}
	handler.removeRole = func(_ *discordgo.Session, _, _, _ string) error {
		calls.remove++
		return nil
	}

	if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
		t.Fatalf("handle component: %v", err)
	}
	if calls.add != 1 || calls.remove != 0 {
		t.Fatalf("unexpected toggle counts: add=%d remove=%d", calls.add, calls.remove)
	}
	resp := rec.last(t)
	if !strings.Contains(resp.Data.Content, "Assigned") {
		t.Fatalf("expected assigned confirmation, got %q", resp.Data.Content)
	}
}

func TestRolePanelComponentRemovesRoleWhenPresent(t *testing.T) {
	guildID := "guild-remove"
	userID := "user-remove"
	roleID := "1380644552700067963"
	cm := newRolePanelTestConfigManager(t, guildID, roleID)
	session, rec := newRolePanelTestSession(t)
	interaction := newRolePanelTestComponentInteraction(guildID, userID, roleID)

	var calls struct{ add, remove int }
	handler := newRolePanelComponentHandler(cm)
	handler.memberLookup = stubMemberHasRole(true)
	handler.addRole = func(*discordgo.Session, string, string, string) error {
		calls.add++
		return nil
	}
	handler.removeRole = func(_ *discordgo.Session, gid, uid, rid string) error {
		calls.remove++
		if rid != roleID {
			t.Fatalf("unexpected role id passed to remove: %s", rid)
		}
		return nil
	}

	if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
		t.Fatalf("handle component: %v", err)
	}
	if calls.add != 0 || calls.remove != 1 {
		t.Fatalf("unexpected toggle counts: add=%d remove=%d", calls.add, calls.remove)
	}
	resp := rec.last(t)
	if !strings.Contains(resp.Data.Content, "Removed") {
		t.Fatalf("expected removed confirmation, got %q", resp.Data.Content)
	}
}

func TestRolePanelComponentRejectsUnknownRole(t *testing.T) {
	guildID := "guild-unknown"
	userID := "user-unknown"
	configuredRole := "1380646828294410342"
	unknownRole := "9999999999999999999"
	cm := newRolePanelTestConfigManager(t, guildID, configuredRole)
	session, rec := newRolePanelTestSession(t)
	interaction := newRolePanelTestComponentInteraction(guildID, userID, unknownRole)

	handler := newRolePanelComponentHandler(cm)
	handler.memberLookup = func(*discordgo.Session, *discordgo.InteractionCreate, string) (bool, error) {
		t.Fatal("memberLookup should not run when the role is not registered")
		return false, nil
	}
	calls := 0
	handler.addRole = func(*discordgo.Session, string, string, string) error {
		calls++
		return nil
	}
	handler.removeRole = func(*discordgo.Session, string, string, string) error {
		calls++
		return nil
	}

	if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
		t.Fatalf("handle component: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no add/remove calls for unknown role, got %d", calls)
	}
	resp := rec.last(t)
	if !strings.Contains(resp.Data.Content, "no longer linked") {
		t.Fatalf("expected unknown-role response, got %q", resp.Data.Content)
	}
}

func TestRolePanelComponentRespectsFeatureToggle(t *testing.T) {
	guildID := "guild-disabled"
	userID := "user-disabled"
	roleID := "1391513234091151430"
	cm := files.NewMemoryConfigManager()
	disabled := false
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			RolePanels: &disabled,
		},
	}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	if err := cm.UpsertRolePanelButton(guildID, "pings", files.RolePanelButtonConfig{
		RoleID: roleID,
		Label:  "Disabled",
	}); err != nil {
		t.Fatalf("upsert button: %v", err)
	}
	session, rec := newRolePanelTestSession(t)
	interaction := newRolePanelTestComponentInteraction(guildID, userID, roleID)

	handler := newRolePanelComponentHandler(cm)
	handler.memberLookup = func(*discordgo.Session, *discordgo.InteractionCreate, string) (bool, error) {
		t.Fatal("memberLookup should not run when feature disabled")
		return false, nil
	}
	handler.addRole = func(*discordgo.Session, string, string, string) error {
		t.Fatal("addRole should not run when feature disabled")
		return nil
	}
	handler.removeRole = func(*discordgo.Session, string, string, string) error {
		t.Fatal("removeRole should not run when feature disabled")
		return nil
	}

	if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
		t.Fatalf("handle component: %v", err)
	}
	resp := rec.last(t)
	if !strings.Contains(resp.Data.Content, "disabled") {
		t.Fatalf("expected disabled message, got %q", resp.Data.Content)
	}
}

func TestRolePanelComponentSurfacesLookupFailure(t *testing.T) {
	guildID := "guild-lookup-fail"
	userID := "user-lookup-fail"
	roleID := "1380646772698910863"
	cm := newRolePanelTestConfigManager(t, guildID, roleID)
	session, rec := newRolePanelTestSession(t)
	interaction := newRolePanelTestComponentInteraction(guildID, userID, roleID)

	handler := newRolePanelComponentHandler(cm)
	handler.memberLookup = func(*discordgo.Session, *discordgo.InteractionCreate, string) (bool, error) {
		return false, errors.New("rate limited")
	}
	handler.addRole = func(*discordgo.Session, string, string, string) error {
		t.Fatal("addRole should not run when lookup fails")
		return nil
	}
	handler.removeRole = func(*discordgo.Session, string, string, string) error {
		t.Fatal("removeRole should not run when lookup fails")
		return nil
	}

	if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
		t.Fatalf("handle component: %v", err)
	}
	resp := rec.last(t)
	if !strings.Contains(resp.Data.Content, "Could not read your current roles") {
		t.Fatalf("expected lookup-failure surface, got %q", resp.Data.Content)
	}
}

func TestRolePanelComponentSurfacesAddFailure(t *testing.T) {
	guildID := "guild-add-fail"
	userID := "user-add-fail"
	roleID := "1380646772698910862"
	cm := newRolePanelTestConfigManager(t, guildID, roleID)
	session, rec := newRolePanelTestSession(t)
	interaction := newRolePanelTestComponentInteraction(guildID, userID, roleID)

	handler := newRolePanelComponentHandler(cm)
	handler.memberLookup = stubMemberHasRole(false)
	handler.addRole = func(*discordgo.Session, string, string, string) error {
		return errors.New("permissions missing")
	}
	handler.removeRole = func(*discordgo.Session, string, string, string) error {
		return nil
	}

	if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
		t.Fatalf("handle component: %v", err)
	}
	resp := rec.last(t)
	if !strings.Contains(resp.Data.Content, "Could not assign") {
		t.Fatalf("expected add failure surface, got %q", resp.Data.Content)
	}
}

func TestRolePanelComponentHandler_AckBehavior(t *testing.T) {
	guildID := "guild-ack"
	userID := "user-ack"
	roleID := "1380646673482518639"
	cm := newRolePanelTestConfigManager(t, guildID, roleID)

	session, rec := newRolePanelTestSession(t)
	interaction := newRolePanelTestComponentInteraction(guildID, userID, roleID)

	handler := newRolePanelComponentHandler(cm)
	handler.memberLookup = stubMemberHasRole(false)
	handler.addRole = func(_ *discordgo.Session, _, _, _ string) error {
		return nil
	}
	handler.removeRole = func(_ *discordgo.Session, _, _, _ string) error {
		return nil
	}

	t.Run("EphemeralEnabled", func(t *testing.T) {
		rec.responses = nil

		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID,
			RuntimeConfig: files.RuntimeConfig{
				DisableInteractiveEphemeral: false,
			},
		}); err != nil {
			t.Fatalf("add guild config: %v", err)
		}

		if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
			t.Fatalf("handle component: %v", err)
		}

		if len(rec.responses) == 0 {
			t.Fatal("expected at least one response")
		}

		firstResp := rec.responses[0]
		if firstResp.Type == discordgo.InteractionResponseDeferredMessageUpdate {
			t.Fatalf("did not expect deferred message update when ephemeral is enabled, got %v", firstResp.Type)
		}
	})

	t.Run("EphemeralDisabled", func(t *testing.T) {
		rec.responses = nil

		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID,
			RuntimeConfig: files.RuntimeConfig{
				DisableInteractiveEphemeral: true,
			},
		}); err != nil {
			t.Fatalf("add guild config: %v", err)
		}

		if err := runRolePanelComponent(t, cm, session, interaction, handler); err != nil {
			t.Fatalf("handle component: %v", err)
		}

		if len(rec.responses) == 0 {
			t.Fatal("expected at least one response")
		}

		firstResp := rec.responses[0]
		if firstResp.Type != discordgo.InteractionResponseDeferredMessageUpdate {
			t.Fatalf("expected deferred message update when ephemeral is disabled, got %v", firstResp.Type)
		}
	})
}
