package roles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	rolesvc "github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

var (
	testMocks sync.Map // map[string]*testHTTPMock
)

type testHTTPMock struct {
	mu        sync.Mutex
	status    int
	body      []byte
	reqs      []*http.Request
	reqBodies [][]byte
}

func (m *testHTTPMock) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqs = append(m.reqs, req)
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	m.reqBodies = append(m.reqBodies, body)

	status := m.status
	respBody := m.body

	if strings.Contains(req.URL.Path, "/interactions/") {
		status = http.StatusOK
		respBody = []byte(`{}`)
	} else if strings.Contains(req.URL.Path, "/channels/") && strings.Contains(req.URL.Path, "/messages") {
		if req.Method == http.MethodPost {
			respBody = []byte(`{"id": "999888777", "channel_id": "12345"}`)
		}
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     make(http.Header),
	}, nil
}

// init removed

func resetMockHTTP(t *testing.T) {
	mock := &testHTTPMock{
		status: http.StatusOK,
		body:   []byte(`{}`),
	}
	testMocks.Store(t.Name(), mock)
}

func getLastResponse(t *testing.T) string {
	m, ok := testMocks.Load(t.Name())
	if !ok {
		return ""
	}
	mock := m.(*testHTTPMock)
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.reqBodies) == 0 {
		return ""
	}
	return string(mock.reqBodies[len(mock.reqBodies)-1])
}

func setMockStatusAndBody(t *testing.T, status int, body []byte) {
	if m, ok := testMocks.Load(t.Name()); ok {
		mock := m.(*testHTTPMock)
		mock.mu.Lock()
		defer mock.mu.Unlock()
		mock.status = status
		mock.body = body
	}
}

func newTestContext(t *testing.T, event discord.InteractionEvent, cm *files.ConfigManager) *commands.ArikawaContext {
	ctx, _ := commands.NewArikawaContext(event, cm)
	if ctx != nil {
		ctx.Client = api.NewClient("mockToken")
		if m, ok := testMocks.Load(t.Name()); ok {
			customClient := http.Client{Transport: m.(*testHTTPMock)}
			ctx.Client.Client.Client = httpdriver.WrapClient(customClient)
		}
	}
	return ctx
}

func newSubCommandContext(t *testing.T, cm *files.ConfigManager, subCommandName string, options []discord.CommandInteractionOption) *commands.ArikawaContext {
	return newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type:    discord.SubcommandOptionType,
					Name:    subCommandName,
					Options: options,
				},
			},
		},
	}, cm)
}

func newNestedSubCommandContext(t *testing.T, cm *files.ConfigManager, groupName string, subCommandName string, options []discord.CommandInteractionOption) *commands.ArikawaContext {
	return newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: groupName,
					Options: []discord.CommandInteractionOption{
						{
							Type:    discord.SubcommandOptionType,
							Name:    subCommandName,
							Options: options,
						},
					},
				},
			},
		},
	}, cm)
}

func setupConfigManagerWithPanel(t *testing.T) (*files.ConfigManager, *rolesvc.RolePanelService) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	enabled := true
	_, err := cm.UpdateConfig(context.Background(), func(bc *files.BotConfig) error {
		bc.Guilds = []files.GuildConfig{
			{
				GuildID: "12345",
				Features: files.FeatureToggles{
					RolePanels: &enabled,
				},
				RolePanels: []files.RolePanelConfig{
					{
						Key:         "test-key",
						Title:       "Test Title",
						Description: "Test Description",
						Color:       0x00ff00,
						Buttons: []files.RolePanelButtonConfig{
							{RoleID: "987654321", Label: "Role A"},
						},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to setup config manager: %v", err)
	}
	svc := rolesvc.NewRolePanelService(cm)
	return cm, svc
}

func TestRolePanelCommands_Registration(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	svc := rolesvc.NewRolePanelService(cm)
	rc := NewRolePanelCommands(cm, svc)

	router := commands.NewCommandRouter(api.NewClient("dummy"), cm)
	rc.RegisterCommands(router)

	cmds := router.Registry().GetAllCommands()
	if len(cmds) == 0 {
		t.Errorf("expected commands to be registered, got none")
	}

	if _, ok := cmds[rolePanelCommandName]; !ok {
		t.Errorf("expected command %s to be registered", rolePanelCommandName)
	}
}

func TestRolePanelCommands_ConvertPanelToArikawa(t *testing.T) {
	panel := files.RolePanelConfig{
		Key:           "test-panel",
		Title:         "Test Title",
		Description:   "Test Description",
		Color:         0x00ff00,
		AuthorName:    "Author",
		AuthorIconURL: "http://author.icon",
		FooterText:    "Footer",
		FooterIconURL: "http://footer.icon",
		ImageURL:      "http://image.url",
		ThumbnailURL:  "http://thumbnail.url",
		Fields: []files.RolePanelEmbedFieldConfig{
			{Name: "Field 1", Value: "Value 1", Inline: true},
		},
		Buttons: []files.RolePanelButtonConfig{
			{RoleID: "1", Label: "B1", EmojiName: "emoji1", EmojiID: "111111111111111111"},
			{RoleID: "2", Label: "B2"},
			{RoleID: "3", Label: "B3"},
			{RoleID: "4", Label: "B4"},
			{RoleID: "5", Label: "B5"},
			{RoleID: "6", Label: "B6"},
		},
	}

	embed, components := convertPanelToArikawa(panel)

	if embed.Title != "Test Title" {
		t.Errorf("expected Title %q, got %q", "Test Title", embed.Title)
	}
	if embed.Color != 0x00ff00 {
		t.Errorf("expected Color %d, got %d", 0x00ff00, embed.Color)
	}
	if embed.Author == nil || embed.Author.Name != "Author" {
		t.Errorf("expected author name to be Author")
	}
	if embed.Footer == nil || embed.Footer.Text != "Footer" {
		t.Errorf("expected footer text to be Footer")
	}
	if len(embed.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(embed.Fields))
	}

	// Buttons should be split into 2 action rows because max buttons per row is 5.
	if len(components) != 2 {
		t.Errorf("expected 2 action rows, got %d", len(components))
	}
	row1, ok1 := components[0].(*discord.ActionRowComponent)
	row2, ok2 := components[1].(*discord.ActionRowComponent)
	if !ok1 || !ok2 {
		t.Fatalf("expected ActionRowComponent types")
	}
	if len(*row1) != 5 {
		t.Errorf("expected 5 buttons in row 1, got %d", len(*row1))
	}
	if len(*row2) != 1 {
		t.Errorf("expected 1 button in row 2, got %d", len(*row2))
	}
}

func TestRolePanelCommands_SubCommands(t *testing.T) {
	t.Run("post", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelPostSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "post", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("post handle failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "Panel `test-key` was posted") {
			t.Errorf("expected success response, got: %s", getLastResponse(t))
		}
		// Check that posting config is stored
		panel, err := cm.RolePanel("12345", "test-key")
		if err != nil {
			t.Fatalf("failed to fetch panel: %v", err)
		}
		if len(panel.Postings) != 1 {
			t.Errorf("expected 1 posting configured, got %d", len(panel.Postings))
		}
	})

	t.Run("preview", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelPreviewSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "preview", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("preview handle failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "test-key") && !strings.Contains(getLastResponse(t), "Test Title") {
			t.Errorf("expected preview embed payload, got: %s", getLastResponse(t))
		}
	})

	t.Run("set", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelSetSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "set", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionTitle, Type: discord.StringOptionType, Value: []byte(`"New Title"`)},
			{Name: rolePanelOptionDescription, Type: discord.StringOptionType, Value: []byte(`"New Description"`)},
			{Name: rolePanelOptionColor, Type: discord.IntegerOptionType, Value: []byte(`255`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("set handle failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "updated") {
			t.Errorf("expected updated message, got: %s", getLastResponse(t))
		}
		panel, _ := cm.RolePanel("12345", "test-key")
		if panel.Title != "New Title" || panel.Description != "New Description" || panel.Color != 255 {
			t.Errorf("panel values not updated: %+v", panel)
		}
	})

	t.Run("delete", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelDeleteSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "delete", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("delete handle failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "deleted") {
			t.Errorf("expected deleted message, got: %s", getLastResponse(t))
		}
		_, err = cm.RolePanel("12345", "test-key")
		if !errors.Is(err, files.ErrRolePanelNotFound) {
			t.Errorf("expected panel to be deleted, got: %v", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		resetMockHTTP(t)
		cm, _ := setupConfigManagerWithPanel(t)
		cmd := newRolePanelListSubCommand(cm)
		ctx := newSubCommandContext(t, cm, "list", nil)
		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("list handle failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "test-key") {
			t.Errorf("expected list to contain test-key, got: %s", getLastResponse(t))
		}
	})

	t.Run("placeholders", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)

		// refresh
		cmdRefresh := newRolePanelRefreshSubCommand(cm, svc)
		ctxRefresh := newSubCommandContext(t, cm, "refresh", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		_ = cmdRefresh.Handle(ctxRefresh)
		if !strings.Contains(getLastResponse(t), "Refresh logic placeholder") {
			t.Errorf("expected refresh placeholder, got: %s", getLastResponse(t))
		}

		// unpost
		cmdUnpost := newRolePanelUnpostSubCommand(cm, svc)
		ctxUnpost := newSubCommandContext(t, cm, "unpost", []discord.CommandInteractionOption{
			{Name: rolePanelOptionMessageID, Type: discord.StringOptionType, Value: []byte(`"999888"`)},
		})

		_ = cmdUnpost.Handle(ctxUnpost)
		if !strings.Contains(getLastResponse(t), "Unpost logic placeholder") {
			t.Errorf("expected unpost placeholder, got: %s", getLastResponse(t))
		}

		// toggle
		cmdToggle := newRolePanelToggleSubCommand(cm)
		ctxToggle := newSubCommandContext(t, cm, "toggle", nil)
		_ = cmdToggle.Handle(ctxToggle)
		if !strings.Contains(getLastResponse(t), "Toggle logic placeholder") {
			t.Errorf("expected toggle placeholder, got: %s", getLastResponse(t))
		}

		// import
		cmdImport := newRolePanelImportSubCommand(cm, svc)
		ctxImport := newSubCommandContext(t, cm, "import", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionURL, Type: discord.StringOptionType, Value: []byte(`"http://url"`)},
		})

		_ = cmdImport.Handle(ctxImport)
		if !strings.Contains(getLastResponse(t), "Import logic placeholder") {
			t.Errorf("expected import placeholder, got: %s", getLastResponse(t))
		}

		// export
		cmdExport := newRolePanelExportSubCommand(cm)
		ctxExport := newSubCommandContext(t, cm, "export", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		_ = cmdExport.Handle(ctxExport)
		if !strings.Contains(getLastResponse(t), "Export logic placeholder") {
			t.Errorf("expected export placeholder, got: %s", getLastResponse(t))
		}
	})

	t.Run("buttons", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)

		// add button
		cmdAdd := newRolePanelButtonAddSubCommand(cm, svc)
		ctxAdd := newNestedSubCommandContext(t, cm, "button", "add", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionRole, Type: discord.StringOptionType, Value: []byte(`"111222333"`)},
			{Name: rolePanelOptionLabel, Type: discord.StringOptionType, Value: []byte(`"Role B"`)},
			{Name: rolePanelOptionEmoji, Type: discord.StringOptionType, Value: []byte(`":smile:"`)},
		})

		err := cmdAdd.Handle(ctxAdd)
		if err != nil {
			t.Fatalf("button add failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "saved") {
			t.Errorf("expected saved response, got: %s", getLastResponse(t))
		}

		panel, _ := cm.RolePanel("12345", "test-key")
		if len(panel.Buttons) != 2 {
			t.Errorf("expected 2 buttons, got %d", len(panel.Buttons))
		}

		// list buttons
		cmdList := newRolePanelButtonListSubCommand(cm)
		ctxList := newNestedSubCommandContext(t, cm, "button", "list", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		err = cmdList.Handle(ctxList)
		if err != nil {
			t.Fatalf("button list failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "has 2 buttons") {
			t.Errorf("expected list to show 2 buttons, got: %s", getLastResponse(t))
		}

		// remove button
		cmdRemove := newRolePanelButtonRemoveSubCommand(cm, svc)
		ctxRemove := newNestedSubCommandContext(t, cm, "button", "remove", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionRole, Type: discord.StringOptionType, Value: []byte(`"111222333"`)},
		})

		err = cmdRemove.Handle(ctxRemove)
		if err != nil {
			t.Fatalf("button remove failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "removed") {
			t.Errorf("expected removed response, got: %s", getLastResponse(t))
		}

		panel, _ = cm.RolePanel("12345", "test-key")
		if len(panel.Buttons) != 1 {
			t.Errorf("expected 1 button, got %d", len(panel.Buttons))
		}
	})

	t.Run("fields", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)

		// add field
		cmdAdd := newRolePanelFieldAddSubCommand(cm, svc)
		ctxAdd := newNestedSubCommandContext(t, cm, "field", "add", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionFieldName, Type: discord.StringOptionType, Value: []byte(`"Name"`)},
			{Name: rolePanelOptionFieldValue, Type: discord.StringOptionType, Value: []byte(`"Value"`)},
		})

		_ = cmdAdd.Handle(ctxAdd)
		if !strings.Contains(getLastResponse(t), "Field add placeholder") {
			t.Errorf("expected field add placeholder, got: %s", getLastResponse(t))
		}

		// remove field
		cmdRemove := newRolePanelFieldRemoveSubCommand(cm, svc)
		ctxRemove := newNestedSubCommandContext(t, cm, "field", "remove", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionFieldIndex, Type: discord.IntegerOptionType, Value: []byte(`1`)},
		})

		_ = cmdRemove.Handle(ctxRemove)
		if !strings.Contains(getLastResponse(t), "Field remove placeholder") {
			t.Errorf("expected field remove placeholder, got: %s", getLastResponse(t))
		}

		// list fields
		cmdList := newRolePanelFieldListSubCommand(cm)
		ctxList := newNestedSubCommandContext(t, cm, "field", "list", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		_ = cmdList.Handle(ctxList)
		if !strings.Contains(getLastResponse(t), "Field list placeholder") {
			t.Errorf("expected field list placeholder, got: %s", getLastResponse(t))
		}
	})
}

func TestRolePanelCommands_ErrorsAndEdgeCases(t *testing.T) {
	t.Run("disabled feature", func(t *testing.T) {
		resetMockHTTP(t)
		cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
		disabled := false
		_, _ = cm.UpdateConfig(context.Background(), func(bc *files.BotConfig) error {
			bc.Guilds = []files.GuildConfig{
				{
					GuildID: "12345",
					Features: files.FeatureToggles{
						RolePanels: &disabled,
					},
				},
			}
			return nil
		})
		svc := rolesvc.NewRolePanelService(cm)
		cmd := newRolePanelPostSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "post", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("unexpected handle error: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "disabled") {
			t.Errorf("expected disabled error message, got: %s", getLastResponse(t))
		}
	})

	t.Run("post without buttons", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		_ = cm.DeleteRolePanelButton("12345", "test-key", "987654321")

		cmd := newRolePanelPostSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "post", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "has no buttons configured") {
			t.Errorf("expected no buttons message, got: %s", getLastResponse(t))
		}
	})

	t.Run("webhook url unsupported", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelPostSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "post", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionWebhookURL, Type: discord.StringOptionType, Value: []byte(`"http://webhook"`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "not implemented in this mock") {
			t.Errorf("expected webhook error, got: %s", getLastResponse(t))
		}
	})

	t.Run("non-existent panel on set", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelSetSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "set", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"non-existent"`)},
			{Name: rolePanelOptionTitle, Type: discord.StringOptionType, Value: []byte(`"title"`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "updated") {
			t.Errorf("expected updated message, got: %s", getLastResponse(t))
		}
	})

	t.Run("non-existent panel on delete", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelDeleteSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "delete", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"non-existent"`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "does not exist") {
			t.Errorf("expected does not exist error, got: %s", getLastResponse(t))
		}
	})

	t.Run("empty panel key", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelPostSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "post", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`""`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "a non-empty key option is required") {
			t.Errorf("expected key required error, got: %s", getLastResponse(t))
		}
	})

	t.Run("missing button options", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelButtonAddSubCommand(cm, svc)

		ctxNoRole := newNestedSubCommandContext(t, cm, "button", "add", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionLabel, Type: discord.StringOptionType, Value: []byte(`"label"`)},
		})

		_ = cmd.Handle(ctxNoRole)
		if !strings.Contains(getLastResponse(t), "role is required") {
			t.Errorf("expected role required, got: %s", getLastResponse(t))
		}

		ctxNoLabel := newNestedSubCommandContext(t, cm, "button", "add", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionRole, Type: discord.StringOptionType, Value: []byte(`"123"`)},
		})

		_ = cmd.Handle(ctxNoLabel)
		if !strings.Contains(getLastResponse(t), "Label is required") {
			t.Errorf("expected label required, got: %s", getLastResponse(t))
		}
	})

	t.Run("missing button remove options", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelButtonRemoveSubCommand(cm, svc)
		ctxNoRole := newNestedSubCommandContext(t, cm, "button", "remove", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		_ = cmd.Handle(ctxNoRole)
		if !strings.Contains(getLastResponse(t), "role is required") {
			t.Errorf("expected role required, got: %s", getLastResponse(t))
		}
	})

	t.Run("list empty buttons panel", func(t *testing.T) {
		resetMockHTTP(t)
		cm, _ := setupConfigManagerWithPanel(t)
		_ = cm.DeleteRolePanelButton("12345", "test-key", "987654321")

		cmd := newRolePanelButtonListSubCommand(cm)
		ctx := newNestedSubCommandContext(t, cm, "button", "list", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "has no buttons") {
			t.Errorf("expected no buttons message, got: %s", getLastResponse(t))
		}
	})

	t.Run("list empty panels list", func(t *testing.T) {
		resetMockHTTP(t)
		cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
		enabled := true
		_, _ = cm.UpdateConfig(context.Background(), func(bc *files.BotConfig) error {
			bc.Guilds = []files.GuildConfig{{GuildID: "12345", Features: files.FeatureToggles{RolePanels: &enabled}}}
			return nil
		})
		cmd := newRolePanelListSubCommand(cm)
		ctx := newSubCommandContext(t, cm, "list", nil)
		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "No role panels are configured") {
			t.Errorf("expected no panels configured message, got: %s", getLastResponse(t))
		}
	})

	t.Run("respondStructuralError", func(t *testing.T) {
		resetMockHTTP(t)
		cm, _ := setupConfigManagerWithPanel(t)

		var logBuf bytes.Buffer
		jsonHandler := slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError})
		oldDefault := slog.Default()
		slog.SetDefault(slog.New(jsonHandler))
		defer slog.SetDefault(oldDefault)

		ctxErr := newTestContext(t, discord.InteractionEvent{
			GuildID: 12345,
			Member:  &discord.Member{User: discord.User{ID: 999}},
			Data:    &discord.CommandInteraction{},
		}, cm)

		err := respondStructuralError(ctxErr, "Test Action", errors.New("underlying error"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(logBuf.String(), "underlying error") {
			t.Errorf("expected log to contain error details, got: %s", logBuf.String())
		}
	})

	t.Run("refreshRolePanelPostingsBestEffort nil safety", func(t *testing.T) {
		res := refreshRolePanelPostingsBestEffort(nil, nil, nil, "")
		if res != "" {
			t.Errorf("expected empty string for nil parameters, got %q", res)
		}
	})

	t.Run("post failure", func(t *testing.T) {
		resetMockHTTP(t)
		setMockStatusAndBody(t, http.StatusInternalServerError, []byte(`{"message": "Internal Server Error", "code": 0}`))

		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelPostSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "post", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "Failed to post the panel") {
			t.Errorf("expected post failure error, got: %s", getLastResponse(t))
		}
	})

	t.Run("delete with postings success", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		_ = cm.AddRolePanelPosting("12345", "test-key", files.RolePanelPostingConfig{
			ChannelID: "12345",
			MessageID: "999888777",
		})

		cmd := newRolePanelDeleteSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "delete", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "deleted") {
			t.Errorf("expected deleted response, got: %s", getLastResponse(t))
		}
	})

	t.Run("delete with postings sync failure", func(t *testing.T) {
		resetMockHTTP(t)
		setMockStatusAndBody(t, http.StatusInternalServerError, []byte(`{"message": "Internal error", "code": 50001}`))

		cm, svc := setupConfigManagerWithPanel(t)
		_ = cm.AddRolePanelPosting("12345", "test-key", files.RolePanelPostingConfig{
			ChannelID: "12345",
			MessageID: "999888777",
		})

		cmd := newRolePanelDeleteSubCommand(cm, svc)
		ctx := newSubCommandContext(t, cm, "delete", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "Could not reconcile") {
			t.Errorf("expected sync failure message, got: %s", getLastResponse(t))
		}
	})

	t.Run("button add limit reached", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		for i := 1; i <= 25; i++ {
			_ = cm.UpsertRolePanelButton("12345", "test-key", files.RolePanelButtonConfig{
				RoleID: fmt.Sprintf("987%d", i),
				Label:  fmt.Sprintf("Role %d", i),
			})
		}

		cmd := newRolePanelButtonAddSubCommand(cm, svc)
		ctx := newNestedSubCommandContext(t, cm, "button", "add", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionRole, Type: discord.StringOptionType, Value: []byte(`"111222333"`)},
			{Name: rolePanelOptionLabel, Type: discord.StringOptionType, Value: []byte(`"Role B"`)},
		})

		err := cmd.Handle(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(getLastResponse(t), "Failed to save button") {
			t.Errorf("expected save error response, got: %s", getLastResponse(t))
		}
	})

	t.Run("button remove non-existent", func(t *testing.T) {
		resetMockHTTP(t)
		cm, svc := setupConfigManagerWithPanel(t)
		cmd := newRolePanelButtonRemoveSubCommand(cm, svc)
		ctx := newNestedSubCommandContext(t, cm, "button", "remove", []discord.CommandInteractionOption{
			{Name: rolePanelOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
			{Name: rolePanelOptionRole, Type: discord.StringOptionType, Value: []byte(`"999999999"`)},
		})

		_ = cmd.Handle(ctx)
		if !strings.Contains(getLastResponse(t), "Failed to delete button") {
			t.Errorf("expected button not found error, got: %s", getLastResponse(t))
		}
	})
}
