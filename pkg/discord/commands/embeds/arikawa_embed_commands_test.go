package embeds

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	embedsvc "github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

var (
	mockHTTPStatus       = http.StatusOK
	mockHTTPBody         = []byte(`{}`)
	mockExternalHTTPBody []byte
	mockHTTPReqs         []*http.Request
	mockHTTPReqBodies    [][]byte
	mockHTTPMu           sync.Mutex
)

type mockRoundTripper struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func init() {
	http.DefaultTransport = &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			mockHTTPMu.Lock()
			defer mockHTTPMu.Unlock()
			mockHTTPReqs = append(mockHTTPReqs, req)
			var body []byte
			if req.Body != nil {
				body, _ = io.ReadAll(req.Body)
			}
			mockHTTPReqBodies = append(mockHTTPReqBodies, body)

			status := http.StatusOK
			respBody := []byte(`{}`)

			// Intercept requests based on destination host
			if strings.Contains(req.URL.Host, "discord") {
				status = mockHTTPStatus
				respBody = mockHTTPBody
			} else if strings.Contains(req.URL.Host, "pastebin") || strings.Contains(req.URL.Host, "hastebin") {
				if len(mockExternalHTTPBody) > 0 {
					respBody = mockExternalHTTPBody
				} else if req.Method == http.MethodGet {
					// Pastebin import content (Discohook JSON format)
					respBody = []byte(`{"embeds": [{"title": "Imported Title", "description": "Imported Description", "fields": [{"name": "Imported Field", "value": "Imported Value", "inline": true}]}]}`)
				} else {
					// Hastebin/Pastebin upload key
					respBody = []byte(`{"key": "mockkey123"}`)
				}
			}

			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(bytes.NewReader(respBody)),
				Header:     make(http.Header),
			}, nil
		},
	}
}

func resetMockHTTP() {
	mockHTTPMu.Lock()
	defer mockHTTPMu.Unlock()
	mockHTTPStatus = http.StatusOK
	mockHTTPBody = []byte(`{}`)
	mockExternalHTTPBody = nil
	mockHTTPReqs = nil
	mockHTTPReqBodies = nil
}

func getLastResponse() string {
	mockHTTPMu.Lock()
	defer mockHTTPMu.Unlock()
	if len(mockHTTPReqBodies) == 0 {
		return ""
	}
	return string(mockHTTPReqBodies[len(mockHTTPReqBodies)-1])
}

func newTestContext(event discord.InteractionEvent, cm *files.ConfigManager) *commands.ArikawaContext {
	ctx, _ := commands.NewArikawaContext(event, cm)
	if ctx != nil {
		ctx.Client = api.NewClient("mockToken")
	}
	return ctx
}

// fakeIOStore introduces an artificial delay to simulate async I/O and expose race conditions.
type fakeIOStore struct {
	mu     sync.RWMutex
	memory *files.MemoryConfigStore
}

func (s *fakeIOStore) Load() (*files.BotConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.memory.Load()
}

func (s *fakeIOStore) Exists() (bool, error) {
	return s.memory.Exists()
}

func (s *fakeIOStore) Save(cfg *files.BotConfig) error {
	// Simulate async I/O delay to ensure the caller's synchronization barrier holds
	time.Sleep(10 * time.Millisecond)

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memory.Save(cfg)
}

func (s *fakeIOStore) Describe() string {
	return "Fake IO Intercepted Store"
}

func (s *fakeIOStore) Finish() {}

func TestEmbedCommands_ConcurrentMutation(t *testing.T) {
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	guildID := "guild-concurrent"

	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to init guild config: %v", err)
	}

	ce := files.CustomEmbedConfig{Key: "concurrent-embed", Title: "Initial Title"}
	cm.SetCustomEmbedProperties(guildID, ce.Key, ce)

	var wg sync.WaitGroup
	workers := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			field := files.CustomEmbedFieldConfig{
				Name:  fmt.Sprintf("Field %d", idx),
				Value: "Val",
			}
			cm.AddCustomEmbedField(guildID, ce.Key, field)
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.RemoveCustomEmbedField(guildID, ce.Key, 0)
		}()
	}

	wg.Wait()

	embeds, err := cm.CustomEmbed(guildID, ce.Key)
	if err != nil {
		t.Fatalf("failed to retrieve embed: %v", err)
	}

	if len(embeds.Fields) > workers {
		t.Errorf("Unexpected array bounds, got %d fields, expected max %d", len(embeds.Fields), workers)
	}
}

func TestEmbedCommands_ObservabilityStructuralFaults(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(jsonHandler)

	restoreLogger := log.SetErrorLoggerRawForTest(logger)
	defer restoreLogger()

	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)

	router := commands.NewCommandRouter(api.NewClient("dummy_token"), cm)
	svc := embedsvc.NewEmbedService(cm)
	embedCmds := NewEmbedCommands(cm, svc)
	embedCmds.RegisterCommands(router)

	interaction := &discord.InteractionEvent{
		ID: discord.InteractionID(999),
		Member: &discord.Member{
			User: discord.User{ID: discord.UserID(456)},
		},
		Data: &discord.CommandInteraction{
			Name: "embed",
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "post",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"valid-key"`)},
								{Name: embedOptionChannel, Type: discord.StringOptionType, Value: []byte(`"not-a-snowflake"`)},
							},
						},
					},
				},
			},
		},
	}

	cm.AddGuildConfig(files.GuildConfig{GuildID: "123"})
	cm.SetCustomEmbedProperties("123", "valid-key", files.CustomEmbedConfig{Key: "valid-key"})
	interaction.GuildID = discord.GuildID(123)

	router.HandleEvent(interaction)

	logOutput := buf.String()

	if !strings.Contains(logOutput, `"level":"ERROR"`) {
		t.Errorf("Expected event to result in slog.LevelError, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"stack_trace":`) {
		t.Errorf("Expected event to preserve JSON matrix with stack_trace, got: %s", logOutput)
	}
}

// spyRouter mocks the ArikawaRegisterer to assert command and component registrations.
type spyRouter struct {
	registered commands.ArikawaCommand
}

func (s *spyRouter) Register(cmd commands.ArikawaCommand) {
	s.registered = cmd
}

func (s *spyRouter) RegisterComponent(customIDPrefix string, handler commands.ComponentHandler) {}

func TestEmbedCommands_RegisterCommands(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	ec := NewEmbedCommands(cm, svc)
	sr := &spyRouter{}
	ec.RegisterCommands(sr)

	if sr.registered == nil {
		t.Fatal("expected command to be registered")
	}
	if sr.registered.Name() != "embed" {
		t.Errorf("expected command name 'embed', got %s", sr.registered.Name())
	}
	if len(sr.registered.Options()) == 0 {
		t.Error("expected options to be registered")
	}

	// Nil routing safety checks
	ec.RegisterCommands(nil)
	ecNil := NewEmbedCommands(nil, nil)
	ecNil.RegisterCommands(sr)
}

func TestEmbedCommands_Post(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:         "test-key",
		Title:       "Test Embed Title",
		Description: "Test Embed Description",
	})
	svc := embedsvc.NewEmbedService(cm)

	// Mock successful Discord API response for Message Create
	mockHTTPMu.Lock()
	mockHTTPBody = []byte(`{"id": "99999", "channel_id": "88888", "content": ""}`)
	mockHTTPMu.Unlock()

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "post",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
								{Name: embedOptionChannel, Type: discord.ChannelOptionType, Value: []byte(`"88888"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmd := newEmbedPostSubCommand(cm, svc)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that a posting was added
	ce, _ := cm.CustomEmbed("12345", "test-key")
	if len(ce.Postings) != 1 || ce.Postings[0].MessageID != "99999" || ce.Postings[0].ChannelID != "88888" {
		t.Errorf("expected 1 posting with msg=99999, got: %v", ce.Postings)
	}

	// Safety check with missing/invalid key
	ctxInvalidKey := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "post",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"non-existent-key"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxInvalidKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "does not exist") {
		t.Errorf("expected does not exist error message, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_Preview(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:         "test-key",
		Title:       "Test Embed Title",
		Description: "Test Embed Description",
	})
	svc := embedsvc.NewEmbedService(cm)

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "preview",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmd := newEmbedPreviewSubCommand(cm, svc)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that the interaction response includes the preview embed
	if !strings.Contains(getLastResponse(), "Test Embed Title") {
		t.Errorf("expected response to contain embed title, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_Set(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	svc := embedsvc.NewEmbedService(cm)

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "set",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"new-embed"`)},
								{Name: embedOptionTitle, Type: discord.StringOptionType, Value: []byte(`"My Title"`)},
								{Name: embedOptionDescription, Type: discord.StringOptionType, Value: []byte(`"My Description"`)},
								{Name: embedOptionColor, Type: discord.IntegerOptionType, Value: []byte(`16711680`)}, // Red
								{Name: embedOptionAuthorName, Type: discord.StringOptionType, Value: []byte(`"Author"`)},
								{Name: embedOptionAuthorIcon, Type: discord.StringOptionType, Value: []byte(`"http://icon"`)},
								{Name: embedOptionFooterText, Type: discord.StringOptionType, Value: []byte(`"Footer"`)},
								{Name: embedOptionFooterIcon, Type: discord.StringOptionType, Value: []byte(`"http://footer"`)},
								{Name: embedOptionImageURL, Type: discord.StringOptionType, Value: []byte(`"http://image"`)},
								{Name: embedOptionThumbnailURL, Type: discord.StringOptionType, Value: []byte(`"http://thumb"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmd := newEmbedSetSubCommand(cm, svc)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ce, err := cm.CustomEmbed("12345", "new-embed")
	if err != nil {
		t.Fatalf("failed to retrieve embed: %v", err)
	}
	if ce.Title != "My Title" || ce.Description != "My Description" || ce.Color != 16711680 {
		t.Errorf("unexpected properties on set custom embed: %v", ce)
	}
	if ce.AuthorName != "Author" || ce.FooterText != "Footer" || ce.ImageURL != "http://image" || ce.ThumbnailURL != "http://thumb" {
		t.Errorf("unexpected sub-properties on set custom embed: %v", ce)
	}
}

func TestEmbedCommands_Delete(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{Key: "test-key"})

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "delete",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmd := newEmbedDeleteSubCommand(cm)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = cm.CustomEmbed("12345", "test-key")
	if !errors.Is(err, files.ErrCustomEmbedNotFound) {
		t.Errorf("expected embed to be deleted, but got: %v", err)
	}

	// Delete non-existent
	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "does not exist") {
		t.Errorf("expected does not exist error message, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_List(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "list",
						},
					},
				},
			},
		},
	}, cm)

	cmd := newEmbedListSubCommand(cm)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "No custom embeds") {
		t.Errorf("expected empty state message, got: %s", getLastResponse())
	}

	_ = cm.SetCustomEmbedProperties("12345", "test-key-1", files.CustomEmbedConfig{Key: "test-key-1"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key-2", files.CustomEmbedConfig{Key: "test-key-2"})

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "test-key-1") || !strings.Contains(getLastResponse(), "test-key-2") {
		t.Errorf("expected configured embeds list, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_Refresh(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:   "test-key",
		Title: "Title",
	})
	_ = cm.AddCustomEmbedPosting("12345", "test-key", files.CustomEmbedPostingConfig{ChannelID: "111", MessageID: "222"})

	svc := embedsvc.NewEmbedService(cm)

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "refresh",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmd := newEmbedRefreshSubCommand(cm, svc)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "Refreshed") && !strings.Contains(getLastResponse(), "updating") {
		t.Errorf("expected refresh summary message, got: %s", getLastResponse())
	}

	// Refresh empty postings
	_ = cm.SetCustomEmbedProperties("12345", "test-key-no-posts", files.CustomEmbedConfig{Key: "test-key-no-posts"})
	ctxNoPosts := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "refresh",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key-no-posts"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxNoPosts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "no tracked postings") {
		t.Errorf("expected no tracked postings message, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_Unpost(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:   "test-key",
		Title: "Title",
	})
	_ = cm.AddCustomEmbedPosting("12345", "test-key", files.CustomEmbedPostingConfig{ChannelID: "111", MessageID: "222"})

	svc := embedsvc.NewEmbedService(cm)

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "unpost",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionMessageID, Type: discord.StringOptionType, Value: []byte(`"222"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmd := newEmbedUnpostSubCommand(cm, svc)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify posting was removed from config
	ce, _ := cm.CustomEmbed("12345", "test-key")
	if len(ce.Postings) != 0 {
		t.Errorf("expected posting to be removed, got: %v", ce.Postings)
	}

	// Unpost non-existent posting message
	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "No tracked posting") {
		t.Errorf("expected no tracked posting warning, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_Fields(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{Key: "test-key"})
	svc := embedsvc.NewEmbedService(cm)

	// Add Field
	ctxAdd := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "field",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "add",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
								{Name: embedOptionFieldName, Type: discord.StringOptionType, Value: []byte(`"FieldName"`)},
								{Name: embedOptionFieldValue, Type: discord.StringOptionType, Value: []byte(`"FieldValue"`)},
								{Name: embedOptionFieldInline, Type: discord.BooleanOptionType, Value: []byte(`true`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmdAdd := newEmbedFieldAddSubCommand(cm, svc)
	err := cmdAdd.Handle(ctxAdd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ce, _ := cm.CustomEmbed("12345", "test-key")
	if len(ce.Fields) != 1 || ce.Fields[0].Name != "FieldName" || ce.Fields[0].Value != "FieldValue" || !ce.Fields[0].Inline {
		t.Errorf("unexpected fields configuration: %v", ce.Fields)
	}

	// List Fields
	ctxList := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "field",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "list",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmdList := newEmbedFieldListSubCommand(cm)
	err = cmdList.Handle(ctxList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "FieldName") {
		t.Errorf("expected fields list output to contain field name, got: %s", getLastResponse())
	}

	// Remove Field
	ctxRemove := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "field",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "remove",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
								{Name: embedOptionFieldIndex, Type: discord.IntegerOptionType, Value: []byte(`1`)}, // 1-based index
							},
						},
					},
				},
			},
		},
	}, cm)

	cmdRemove := newEmbedFieldRemoveSubCommand(cm, svc)
	err = cmdRemove.Handle(ctxRemove)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ce, _ = cm.CustomEmbed("12345", "test-key")
	if len(ce.Fields) != 0 {
		t.Errorf("expected fields list to be empty, got: %v", ce.Fields)
	}

	// List Empty Fields
	err = cmdList.Handle(ctxList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "no fields configured") {
		t.Errorf("expected empty fields warning, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_ImportExport(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:         "test-key",
		Title:       "Initial Title",
		Description: "Initial Description",
	})

	// Import subcommand
	ctxImport := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "import",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
								{Name: embedOptionURL, Type: discord.StringOptionType, Value: []byte(`"https://hastebin.com/raw/mockkey"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmdImport := newEmbedImportSubCommand(cm)
	err := cmdImport.Handle(ctxImport)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ce, _ := cm.CustomEmbed("12345", "test-key")
	if ce.Title != "Imported Title" || ce.Description != "Imported Description" || len(ce.Fields) != 1 || ce.Fields[0].Name != "Imported Field" {
		t.Errorf("unexpected properties on imported embed: %v", ce)
	}

	// Export subcommand
	ctxExport := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "export",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmdExport := newEmbedExportSubCommand(cm)
	err = cmdExport.Handle(ctxExport)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "mockkey123") {
		t.Errorf("expected export response to contain uploaded hastebin paste URL key, got: %s", getLastResponse())
	}
}

func TestEmbedCommands_ErrorAndEdgeCases(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = cm.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{Key: "test-key"})
	svc := embedsvc.NewEmbedService(cm)

	// 1. Missing Key Option (embedKeyFromOptions failure)
	ctxNoKey := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "post",
							Options: []discord.CommandInteractionOption{
								{Name: "not-key", Type: discord.StringOptionType, Value: []byte(`"val"`)},
							},
						},
					},
				},
			},
		},
	}, cm)

	cmdPost := newEmbedPostSubCommand(cm, svc)
	err := cmdPost.Handle(ctxNoKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "key option is required") {
		t.Errorf("expected missing key error, got: %s", getLastResponse())
	}

	// 2. refreshCustomEmbedPostingsBestEffort Nil Safety
	resNil := refreshCustomEmbedPostingsBestEffort(nil, nil, nil, "")
	if resNil != "" {
		t.Errorf("expected empty response for nil parameters, got: %s", resNil)
	}

	// 3. respondStructuralError logging verification
	var logBuf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError})
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(jsonHandler))
	defer slog.SetDefault(oldDefault)

	ctxErr := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data:    &discord.CommandInteraction{},
	}, cm)
	err = respondStructuralError(ctxErr, "Test Action", errors.New("underlying error"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(logBuf.String(), "underlying error") {
		t.Errorf("expected log to contain error details, got: %s", logBuf.String())
	}

	// 4. embedImportSubCommand invalid URL scheme
	ctxImportBadURL := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "import",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
								{Name: embedOptionURL, Type: discord.StringOptionType, Value: []byte(`"ftp://invalid-scheme"`)},
							},
						},
					},
				},
			},
		},
	}, cm)
	cmdImport := newEmbedImportSubCommand(cm)
	err = cmdImport.Handle(ctxImportBadURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "unsupported URL scheme") {
		t.Errorf("expected unsupported URL scheme error, got: %s", getLastResponse())
	}

	// 5. embedImportSubCommand invalid Discohook JSON
	// Inject invalid JSON body response for pastebin/hastebin host
	mockExternalHTTPBody = []byte(`{"invalid": json`)
	ctxImportBadJSON := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "import",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
								{Name: embedOptionURL, Type: discord.StringOptionType, Value: []byte(`"https://hastebin.com/raw/badjson"`)},
							},
						},
					},
				},
			},
		},
	}, cm)
	err = cmdImport.Handle(ctxImportBadJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "Invalid embed JSON") {
		t.Errorf("expected invalid JSON error, got: %s", getLastResponse())
	}

	// Reset custom HTTP body for subsequent tests
	mockExternalHTTPBody = nil

	// 6. embedExportSubCommand non-existent key
	ctxExportNotFound := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "embed",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "export",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"non-existent"`)},
							},
						},
					},
				},
			},
		},
	}, cm)
	cmdExport := newEmbedExportSubCommand(cm)
	err = cmdExport.Handle(ctxExportNotFound)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "does not exist") {
		t.Errorf("expected does not exist error, got: %s", getLastResponse())
	}

	// 7. embedFieldRemoveSubCommand out of bounds index
	ctxRemoveBadIdx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "field",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "remove",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
								{Name: embedOptionFieldIndex, Type: discord.IntegerOptionType, Value: []byte(`99`)}, // Out of bounds
							},
						},
					},
				},
			},
		},
	}, cm)
	cmdRemove := newEmbedFieldRemoveSubCommand(cm, svc)
	err = cmdRemove.Handle(ctxRemoveBadIdx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "invalid field index") {
		t.Errorf("expected invalid field index error, got: %s", getLastResponse())
	}

	// 8. embedFieldRemoveSubCommand missing index option
	ctxRemoveNoIdx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandGroupOptionType,
					Name: "field",
					Options: []discord.CommandInteractionOption{
						{
							Type: discord.SubcommandOptionType,
							Name: "remove",
							Options: []discord.CommandInteractionOption{
								{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"test-key"`)},
							},
						},
					},
				},
			},
		},
	}, cm)
	err = cmdRemove.Handle(ctxRemoveNoIdx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "field index is required") {
		t.Errorf("expected field index required error, got: %s", getLastResponse())
	}
}
