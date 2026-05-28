package files

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func newRolePanelTestManager(t *testing.T, guildID string) *ConfigManager {
	t.Helper()
	mgr := NewMemoryConfigManager()
	mgr.config = &BotConfig{Guilds: []GuildConfig{{GuildID: guildID}}}
	return mgr
}

func TestRolePanelKeyValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"trims and lowercases", "  Pings  ", "pings", false},
		{"keeps digits and dashes", "Welcome-Roles_2", "welcome-roles_2", false},
		{"rejects empty", "  ", "", true},
		{"rejects whitespace inside", "two words", "", true},
		{"rejects punctuation", "with.dot", "", true},
		{"rejects unicode letters", "rôles", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := validateRolePanelKey(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalized key = %q want %q", got, tc.want)
			}
		})
	}
}

func TestRolePanelButtonValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      RolePanelButtonConfig
		wantErr string
	}{
		{
			name: "valid custom emoji",
			in: RolePanelButtonConfig{
				RoleID:        "1380646673482518639",
				Label:         "Announcements",
				EmojiName:     "clouud",
				EmojiID:       "1378934415186464808",
				EmojiAnimated: false,
			},
		},
		{
			name: "valid unicode emoji",
			in: RolePanelButtonConfig{
				RoleID:    "1380646673482518639",
				Label:     "Hello",
				EmojiName: "👋",
			},
		},
		{
			name: "valid no emoji",
			in: RolePanelButtonConfig{
				RoleID: "1380646673482518639",
				Label:  "Click me",
			},
		},
		{
			name:    "missing role",
			in:      RolePanelButtonConfig{Label: "Click"},
			wantErr: "role_id is required",
		},
		{
			name:    "non-numeric role",
			in:      RolePanelButtonConfig{RoleID: "not-a-snowflake", Label: "Click"},
			wantErr: "role_id must be numeric",
		},
		{
			name:    "missing label",
			in:      RolePanelButtonConfig{RoleID: "100"},
			wantErr: "label is required",
		},
		{
			name:    "emoji id without name",
			in:      RolePanelButtonConfig{RoleID: "100", Label: "L", EmojiID: "200"},
			wantErr: "emoji_name is required when emoji_id is set",
		},
		{
			name:    "label too long",
			in:      RolePanelButtonConfig{RoleID: "100", Label: strings.Repeat("a", RolePanelLabelMaxLen+1)},
			wantErr: "label must be at most",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := normalizeRolePanelButton(tc.in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRolePanelEmbedFieldValidation(t *testing.T) {
	t.Parallel()
	validCfg := RolePanelConfig{
		Title:       "  Title  ",
		Description: "  Desc  ",
		Color:       16753104,
	}
	if _, err := validateRolePanelEmbedFields(validCfg); err != nil {
		t.Fatalf("unexpected error on valid input: %v", err)
	}
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{Color: -1}); err == nil {
		t.Fatalf("expected error for negative color")
	}
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{Color: RolePanelColorMax + 1}); err == nil {
		t.Fatalf("expected error for color overflow")
	}
	bigTitle := strings.Repeat("a", RolePanelTitleMaxLen+1)
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{Title: bigTitle}); err == nil {
		t.Fatalf("expected error for oversized title")
	}
	bigAuthor := strings.Repeat("a", RolePanelAuthorMaxLen+1)
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{AuthorName: bigAuthor}); err == nil {
		t.Fatalf("expected error for oversized author name")
	}
}

func TestRolePanelFieldCRUD(t *testing.T) {
	t.Parallel()
	guildID := "guild-fields"
	mgr := newRolePanelTestManager(t, guildID)

	if err := mgr.SetRolePanelEmbed(guildID, "test-panel", RolePanelConfig{
		Title: "Panel with fields",
	}); err != nil {
		t.Fatalf("set embed: %v", err)
	}

	f1 := RolePanelEmbedFieldConfig{Name: "Field 1", Value: "Value 1"}
	if err := mgr.AddRolePanelField(guildID, "test-panel", f1); err != nil {
		t.Fatalf("add field 1: %v", err)
	}

	f2 := RolePanelEmbedFieldConfig{Name: "Field 2", Value: "Value 2", Inline: true}
	if err := mgr.AddRolePanelField(guildID, "test-panel", f2); err != nil {
		t.Fatalf("add field 2: %v", err)
	}

	panel, err := mgr.RolePanel(guildID, "test-panel")
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if len(panel.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(panel.Fields))
	}
	if panel.Fields[0].Name != "Field 1" || panel.Fields[1].Name != "Field 2" {
		t.Fatalf("fields did not match: %+v", panel.Fields)
	}

	if err := mgr.RemoveRolePanelField(guildID, "test-panel", 0); err != nil {
		t.Fatalf("remove field 0: %v", err)
	}

	panel, err = mgr.RolePanel(guildID, "test-panel")
	if err != nil {
		t.Fatalf("get panel after remove: %v", err)
	}
	if len(panel.Fields) != 1 || panel.Fields[0].Name != "Field 2" {
		t.Fatalf("expected 1 field named 'Field 2', got %+v", panel.Fields)
	}

	// Test out of bounds remove
	if err := mgr.RemoveRolePanelField(guildID, "test-panel", 5); err == nil {
		t.Fatalf("expected error for out of bounds field removal")
	}
}

func TestRolePanelCRUDLifecycle(t *testing.T) {
	t.Parallel()
	guildID := "guild-1"
	mgr := newRolePanelTestManager(t, guildID)

	if err := mgr.SetRolePanelEmbed(guildID, "pings", RolePanelConfig{
		Title:       "⋆｡°✩ ! ✩°｡⋆ Pings! ⋆｡°✩ ! ✩°｡⋆",
		Description: "Please select some of our optional roles below!",
		Color:       16753104,
	}); err != nil {
		t.Fatalf("set embed: %v", err)
	}

	if err := mgr.UpsertRolePanelButton(guildID, "pings", RolePanelButtonConfig{
		RoleID:    "1380646673482518639",
		Label:     "Announcements",
		EmojiName: "clouud",
		EmojiID:   "1378934415186464808",
	}); err != nil {
		t.Fatalf("upsert button: %v", err)
	}

	panel, err := mgr.RolePanel(guildID, "PINGS")
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if panel.Key != "pings" {
		t.Fatalf("expected normalized key, got %q", panel.Key)
	}
	if panel.Color != 16753104 {
		t.Fatalf("color persisted incorrectly: %d", panel.Color)
	}
	if len(panel.Buttons) != 1 || panel.Buttons[0].RoleID != "1380646673482518639" {
		t.Fatalf("unexpected buttons after upsert: %+v", panel.Buttons)
	}

	if err := mgr.UpsertRolePanelButton(guildID, "pings", RolePanelButtonConfig{
		RoleID:    "1380646673482518639",
		Label:     "Announcements (renamed)",
		EmojiName: "clouud",
		EmojiID:   "1378934415186464808",
	}); err != nil {
		t.Fatalf("re-upsert button: %v", err)
	}
	panel, err = mgr.RolePanel(guildID, "pings")
	if err != nil {
		t.Fatalf("re-fetch panel: %v", err)
	}
	if len(panel.Buttons) != 1 || panel.Buttons[0].Label != "Announcements (renamed)" {
		t.Fatalf("re-upsert did not replace existing button: %+v", panel.Buttons)
	}

	panels, err := mgr.RolePanels(guildID)
	if err != nil {
		t.Fatalf("list panels: %v", err)
	}
	if len(panels) != 1 {
		t.Fatalf("expected one panel, got %d", len(panels))
	}

	if err := mgr.DeleteRolePanelButton(guildID, "pings", "1380646673482518639"); err != nil {
		t.Fatalf("delete button: %v", err)
	}
	panel, err = mgr.RolePanel(guildID, "pings")
	if err != nil {
		t.Fatalf("re-fetch panel after delete: %v", err)
	}
	if len(panel.Buttons) != 0 {
		t.Fatalf("expected zero buttons after delete, got %+v", panel.Buttons)
	}

	if err := mgr.DeleteRolePanel(guildID, "pings"); err != nil {
		t.Fatalf("delete panel: %v", err)
	}
	if _, err := mgr.RolePanel(guildID, "pings"); !errors.Is(err, ErrRolePanelNotFound) {
		t.Fatalf("expected ErrRolePanelNotFound after delete, got %v", err)
	}
}

func TestRolePanelButtonByRoleIDFindsAcrossPanels(t *testing.T) {
	t.Parallel()
	guildID := "guild-2"
	mgr := newRolePanelTestManager(t, guildID)

	if err := mgr.UpsertRolePanelButton(guildID, "alpha", RolePanelButtonConfig{
		RoleID: "111",
		Label:  "Alpha button",
	}); err != nil {
		t.Fatalf("upsert alpha: %v", err)
	}
	if err := mgr.UpsertRolePanelButton(guildID, "beta", RolePanelButtonConfig{
		RoleID: "222",
		Label:  "Beta button",
	}); err != nil {
		t.Fatalf("upsert beta: %v", err)
	}

	panel, btn, err := mgr.RolePanelButtonByRoleID(guildID, "222")
	if err != nil {
		t.Fatalf("find by role: %v", err)
	}
	if panel.Key != "beta" || btn.Label != "Beta button" {
		t.Fatalf("unexpected match: %+v / %+v", panel, btn)
	}

	if _, _, err := mgr.RolePanelButtonByRoleID(guildID, "999"); !errors.Is(err, ErrRolePanelButtonNotFound) {
		t.Fatalf("expected ErrRolePanelButtonNotFound, got %v", err)
	}
}

func TestRolePanelButtonCapEnforced(t *testing.T) {
	t.Parallel()
	guildID := "guild-cap"
	mgr := newRolePanelTestManager(t, guildID)
	for i := 0; i < RolePanelMaxButtons; i++ {
		btn := RolePanelButtonConfig{
			RoleID: pad19DigitID(i + 1),
			Label:  "Button",
		}
		if err := mgr.UpsertRolePanelButton(guildID, "cap", btn); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	overflow := RolePanelButtonConfig{
		RoleID: pad19DigitID(RolePanelMaxButtons + 1),
		Label:  "Overflow",
	}
	if err := mgr.UpsertRolePanelButton(guildID, "cap", overflow); !errors.Is(err, ErrInvalidRolePanelInput) {
		t.Fatalf("expected ErrInvalidRolePanelInput at cap, got %v", err)
	}
}

func TestRolePanelMutationsAreSnapshotIsolated(t *testing.T) {
	t.Parallel()
	guildID := "guild-snap"
	mgr := newRolePanelTestManager(t, guildID)
	if err := mgr.UpsertRolePanelButton(guildID, "iso", RolePanelButtonConfig{
		RoleID: "100",
		Label:  "Original",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	panel, err := mgr.RolePanel(guildID, "iso")
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	panel.Buttons[0].Label = "Mutated outside"

	again, err := mgr.RolePanel(guildID, "iso")
	if err != nil {
		t.Fatalf("re-fetch panel: %v", err)
	}
	if !reflect.DeepEqual(again.Buttons[0].Label, "Original") {
		t.Fatalf("snapshot leaked mutation: %q", again.Buttons[0].Label)
	}
}

func TestRolePanelPostingValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      RolePanelPostingConfig
		wantErr string
	}{
		{name: "valid", in: RolePanelPostingConfig{ChannelID: "100", MessageID: "200"}},
		{name: "missing channel", in: RolePanelPostingConfig{MessageID: "200"}, wantErr: "channel_id is required"},
		{name: "non-numeric channel", in: RolePanelPostingConfig{ChannelID: "abc", MessageID: "200"}, wantErr: "channel_id must be numeric"},
		{name: "missing message", in: RolePanelPostingConfig{ChannelID: "100"}, wantErr: "message_id is required"},
		{name: "non-numeric message", in: RolePanelPostingConfig{ChannelID: "100", MessageID: "abc"}, wantErr: "message_id must be numeric"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := normalizeRolePanelPosting(tc.in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRolePanelPostingsCRUD(t *testing.T) {
	t.Parallel()
	guildID := "guild-postings"
	mgr := newRolePanelTestManager(t, guildID)
	if err := mgr.UpsertRolePanelButton(guildID, "pings", RolePanelButtonConfig{RoleID: "100", Label: "Click"}); err != nil {
		t.Fatalf("seed button: %v", err)
	}

	const (
		firstID  = "601"
		secondID = "602"
	)
	posting := RolePanelPostingConfig{ChannelID: "111", MessageID: firstID}
	if err := mgr.AddRolePanelPosting(guildID, "pings", posting); err != nil {
		t.Fatalf("add posting: %v", err)
	}

	if err := mgr.AddRolePanelPosting(guildID, "pings", posting); err != nil {
		t.Fatalf("re-add same posting must be idempotent, got: %v", err)
	}
	listed, err := mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list postings: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected dedupe to keep one entry, got %d", len(listed))
	}

	another := RolePanelPostingConfig{ChannelID: "111", MessageID: secondID}
	if err := mgr.AddRolePanelPosting(guildID, "pings", another); err != nil {
		t.Fatalf("add second posting: %v", err)
	}
	listed, err = mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list after second add: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(listed))
	}

	if err := mgr.RemoveRolePanelPosting(guildID, "pings", firstID); err != nil {
		t.Fatalf("remove posting: %v", err)
	}
	listed, err = mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if len(listed) != 1 || listed[0].MessageID != secondID {
		t.Fatalf("unexpected postings after remove: %+v", listed)
	}

	if err := mgr.RemoveRolePanelPosting(guildID, "pings", firstID); !errors.Is(err, ErrRolePanelPostingNotFound) {
		t.Fatalf("expected ErrRolePanelPostingNotFound for repeat remove, got %v", err)
	}

	if err := mgr.ClearRolePanelPostings(guildID, "pings"); err != nil {
		t.Fatalf("clear postings: %v", err)
	}
	listed, err = mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected zero postings after clear, got %d", len(listed))
	}
}

func TestFindRolePanelPostingSearchesAcrossPanels(t *testing.T) {
	t.Parallel()
	guildID := "guild-find-posting"
	mgr := newRolePanelTestManager(t, guildID)
	if err := mgr.UpsertRolePanelButton(guildID, "alpha", RolePanelButtonConfig{RoleID: "100", Label: "A"}); err != nil {
		t.Fatalf("seed alpha: %v", err)
	}
	if err := mgr.UpsertRolePanelButton(guildID, "beta", RolePanelButtonConfig{RoleID: "200", Label: "B"}); err != nil {
		t.Fatalf("seed beta: %v", err)
	}
	if err := mgr.AddRolePanelPosting(guildID, "alpha", RolePanelPostingConfig{ChannelID: "11", MessageID: "501"}); err != nil {
		t.Fatalf("add alpha posting: %v", err)
	}
	if err := mgr.AddRolePanelPosting(guildID, "beta", RolePanelPostingConfig{ChannelID: "22", MessageID: "502"}); err != nil {
		t.Fatalf("add beta posting: %v", err)
	}

	key, posting, err := mgr.FindRolePanelPosting(guildID, "502")
	if err != nil {
		t.Fatalf("find posting: %v", err)
	}
	if key != "beta" || posting.ChannelID != "22" {
		t.Fatalf("unexpected match: %s / %+v", key, posting)
	}

	if _, _, err := mgr.FindRolePanelPosting(guildID, "999"); !errors.Is(err, ErrRolePanelPostingNotFound) {
		t.Fatalf("expected ErrRolePanelPostingNotFound, got %v", err)
	}
}

func pad19DigitID(n int) string {
	const base = "1000000000000000000"
	if n <= 0 {
		return base
	}
	out := []byte(base)
	for i := len(out) - 1; i >= 0 && n > 0; i-- {
		digit := byte(n%10) + '0'
		out[i] = digit
		n /= 10
	}
	return string(out)
}
