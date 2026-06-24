package embeds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/config"
	localdiscord "github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	embedsvc "github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"golang.org/x/sync/errgroup"
)

var (
	testMocks sync.Map // map[string]*testHTTPMock
)

type testHTTPMock struct {
	mu        sync.Mutex
	status    int
	body      []byte
	extBody   []byte
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

	status := http.StatusOK
	respBody := []byte(`{}`)

	if strings.Contains(req.URL.Host, "discord") {
		status = m.status
		respBody = m.body
	} else if strings.Contains(req.URL.Host, "pastebin") || strings.Contains(req.URL.Host, "hastebin") {
		if len(m.extBody) > 0 {
			respBody = m.extBody
		} else if req.Method == http.MethodGet {
			respBody = []byte(`{"embeds": [{"title": "Imported Title", "description": "Imported Description", "fields": [{"name": "Imported Field", "value": "Imported Value", "inline": true}]}]}`)
		} else {
			respBody = []byte(`{"key": "mockkey123"}`)
		}
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     make(http.Header),
	}, nil
}

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

func setMockExtBody(t *testing.T, body []byte) {
	if m, ok := testMocks.Load(t.Name()); ok {
		mock := m.(*testHTTPMock)
		mock.mu.Lock()
		defer mock.mu.Unlock()
		mock.extBody = body
	}
}

func getMockReqs(t *testing.T) []*http.Request {
	m, ok := testMocks.Load(t.Name())
	if !ok {
		return nil
	}
	mock := m.(*testHTTPMock)
	mock.mu.Lock()
	defer mock.mu.Unlock()
	return mock.reqs
}

func getMockReqBodies(t *testing.T) [][]byte {
	m, ok := testMocks.Load(t.Name())
	if !ok {
		return nil
	}
	mock := m.(*testHTTPMock)
	mock.mu.Lock()
	defer mock.mu.Unlock()
	return mock.reqBodies
}

func newTestContext(t *testing.T, event discord.InteractionEvent, cm *files.ConfigManager) *commands.ArikawaContext {
	ctx, _ := commands.NewArikawaContext(event, cm)
	if ctx != nil {
		ctx.Client = api.NewClient("mockToken")
		if m, ok := testMocks.Load(t.Name()); ok {
			customClient := http.Client{Transport: m.(*testHTTPMock)}
			ctx.Client.Client.Client = httpdriver.WrapClient(customClient)
			ctx.WithContext(context.WithValue(ctx.Context(), localdiscord.HTTPTransportContextKey, m.(*testHTTPMock)))
		}
	}
	return ctx
}

// fakeIOStore introduces an artificial delay to simulate async I/O and expose race conditions.
type fakeIOStore struct {
	mu     sync.Mutex
	memory *config.MemoryConfigStore
}

func (s *fakeIOStore) Load() (*files.BotConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memory.Load()
}

func (s *fakeIOStore) Exists() (bool, error) {
	return s.memory.Exists()
}

func (s *fakeIOStore) Save(cfg *files.BotConfig) error {
	// Simulate async I/O delay deterministically
	for i := 0; i < 1000; i++ {
		runtime.Gosched()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memory.Save(cfg)
}

func (s *fakeIOStore) Describe() string {
	return "Fake IO Intercepted Store"
}

func (s *fakeIOStore) Finish() {}

func TestEmbedCommands_ConcurrentMutation(t *testing.T) {
	t.Parallel()
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	svc := embedsvc.NewEmbedService(cm)
	guildID := "guild-concurrent"

	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to init guild config: %v", err)
	}

	ce := files.CustomEmbedConfig{Key: "concurrent-embed", Title: "Initial Title"}
	svc.SetCustomEmbedProperties(guildID, ce.Key, ce)

	var eg errgroup.Group
	workers := 50

	for i := 0; i < workers; i++ {
		idx := i
		eg.Go(func() error {
			field := files.CustomEmbedFieldConfig{
				Name:  fmt.Sprintf("Field %d", idx),
				Value: "Val",
			}
			svc.AddCustomEmbedField(guildID, ce.Key, field)
			return nil
		})
	}

	for i := 0; i < 10; i++ {
		eg.Go(func() error {
			svc.RemoveCustomEmbedField(guildID, ce.Key, 0)
			return nil
		})
	}

	_ = eg.Wait()

	embeds, err := svc.CustomEmbed(guildID, ce.Key)
	if err != nil {
		t.Fatalf("failed to retrieve embed: %v", err)
	}

	if len(embeds.Fields) > workers {
		t.Errorf("Unexpected array bounds, got %d fields, expected max %d", len(embeds.Fields), workers)
	}
}

func TestEmbedCommands_ObservabilityStructuralFaults(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(jsonHandler)

	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, logger)
	svc := embedsvc.NewEmbedService(cm)

	router := commands.NewCommandRouter(api.NewClient("dummy_token"), cm).WithLogger(logger)
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
	svc.SetCustomEmbedProperties("123", "valid-key", files.CustomEmbedConfig{Key: "valid-key"})
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
	t.Parallel()
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
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
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:         "test-key",
		Title:       "Test Embed Title",
		Description: "Test Embed Description",
	})

	// Mock successful Discord API response for Message Create
	setMockStatusAndBody(t, http.StatusOK, []byte(`{"id": "99999", "channel_id": "88888", "content": ""}`))

	ctx := newTestContext(t, discord.InteractionEvent{
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
	ce, _ := svc.CustomEmbed("12345", "test-key")
	if len(ce.Postings) != 1 || ce.Postings[0].MessageID != "99999" || ce.Postings[0].ChannelID != "88888" {
		t.Errorf("expected 1 posting with msg=99999, got: %v", ce.Postings)
	}

	// Safety check with missing/invalid key
	ctxInvalidKey := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "does not exist") {
		t.Errorf("expected does not exist error message, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_Preview(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:         "test-key",
		Title:       "Test Embed Title",
		Description: "Test Embed Description",
	})

	ctx := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "Test Embed Title") {
		t.Errorf("expected response to contain embed title, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_Set(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})

	ctx := newTestContext(t, discord.InteractionEvent{
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

	ce, err := svc.CustomEmbed("12345", "new-embed")
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
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{Key: "test-key"})

	ctx := newTestContext(t, discord.InteractionEvent{
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

	cmd := newEmbedDeleteSubCommand(cm, svc)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.CustomEmbed("12345", "test-key")
	if !errors.Is(err, embedsvc.ErrCustomEmbedNotFound) {
		t.Errorf("expected embed to be deleted, but got: %v", err)
	}

	// Delete non-existent
	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "does not exist") {
		t.Errorf("expected does not exist error message, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_List(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})

	ctx := newTestContext(t, discord.InteractionEvent{
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

	cmd := newEmbedListSubCommand(cm, svc)
	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "No custom embeds") {
		t.Errorf("expected empty state message, got: %s", getLastResponse(t))
	}

	_ = svc.SetCustomEmbedProperties("12345", "test-key-1", files.CustomEmbedConfig{Key: "test-key-1"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key-2", files.CustomEmbedConfig{Key: "test-key-2"})

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "test-key-1") || !strings.Contains(getLastResponse(t), "test-key-2") {
		t.Errorf("expected configured embeds list, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_Refresh(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:   "test-key",
		Title: "Title",
	})
	_ = svc.AddCustomEmbedPosting("12345", "test-key", files.CustomEmbedPostingConfig{ChannelID: "111", MessageID: "222"})

	ctx := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "Refreshed") && !strings.Contains(getLastResponse(t), "updating") {
		t.Errorf("expected refresh summary message, got: %s", getLastResponse(t))
	}

	// Refresh empty postings
	_ = svc.SetCustomEmbedProperties("12345", "test-key-no-posts", files.CustomEmbedConfig{Key: "test-key-no-posts"})
	ctxNoPosts := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "no tracked postings") {
		t.Errorf("expected no tracked postings message, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_Unpost(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:   "test-key",
		Title: "Title",
	})
	_ = svc.AddCustomEmbedPosting("12345", "test-key", files.CustomEmbedPostingConfig{ChannelID: "111", MessageID: "222"})

	ctx := newTestContext(t, discord.InteractionEvent{
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
	ce, _ := svc.CustomEmbed("12345", "test-key")
	if len(ce.Postings) != 0 {
		t.Errorf("expected posting to be removed, got: %v", ce.Postings)
	}

	// Unpost non-existent posting message
	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "No tracked posting") {
		t.Errorf("expected no tracked posting warning, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_Fields(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{Key: "test-key"})

	// Add Field
	ctxAdd := newTestContext(t, discord.InteractionEvent{
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

	ce, _ := svc.CustomEmbed("12345", "test-key")
	if len(ce.Fields) != 1 || ce.Fields[0].Name != "FieldName" || ce.Fields[0].Value != "FieldValue" || !ce.Fields[0].Inline {
		t.Errorf("unexpected fields configuration: %v", ce.Fields)
	}

	// List Fields
	ctxList := newTestContext(t, discord.InteractionEvent{
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

	cmdList := newEmbedFieldListSubCommand(cm, svc)
	err = cmdList.Handle(ctxList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "FieldName") {
		t.Errorf("expected fields list output to contain field name, got: %s", getLastResponse(t))
	}

	// Remove Field
	ctxRemove := newTestContext(t, discord.InteractionEvent{
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

	ce, _ = svc.CustomEmbed("12345", "test-key")
	if len(ce.Fields) != 0 {
		t.Errorf("expected fields list to be empty, got: %v", ce.Fields)
	}

	// List Empty Fields
	err = cmdList.Handle(ctxList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "no fields configured") {
		t.Errorf("expected empty fields warning, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_ImportExport(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{
		Key:         "test-key",
		Title:       "Initial Title",
		Description: "Initial Description",
	})

	// Import subcommand
	ctxImport := newTestContext(t, discord.InteractionEvent{
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

	cmdImport := newEmbedImportSubCommand(cm, svc)
	err := cmdImport.Handle(ctxImport)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ce, _ := svc.CustomEmbed("12345", "test-key")
	if ce.Title != "Imported Title" || ce.Description != "Imported Description" || len(ce.Fields) != 1 || ce.Fields[0].Name != "Imported Field" {
		t.Errorf("unexpected properties on imported embed: %v", ce)
	}

	// Export subcommand
	ctxExport := newTestContext(t, discord.InteractionEvent{
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

	cmdExport := newEmbedExportSubCommand(cm, svc)
	err = cmdExport.Handle(ctxExport)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "mockkey123") {
		t.Errorf("expected export response to contain uploaded hastebin paste URL key, got: %s", getLastResponse(t))
	}
}

func TestEmbedCommands_ErrorAndEdgeCases(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := embedsvc.NewEmbedService(cm)
	_ = cm.AddGuildConfig(files.GuildConfig{GuildID: "12345"})
	_ = svc.SetCustomEmbedProperties("12345", "test-key", files.CustomEmbedConfig{Key: "test-key"})

	// 1. Missing Key Option (embedKeyFromOptions failure)
	ctxNoKey := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "key option is required") {
		t.Errorf("expected missing key error, got: %s", getLastResponse(t))
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

	ctxErr := newTestContext(t, discord.InteractionEvent{
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
	ctxImportBadURL := newTestContext(t, discord.InteractionEvent{
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
	cmdImport := newEmbedImportSubCommand(cm, svc)
	err = cmdImport.Handle(ctxImportBadURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "unsupported URL scheme") {
		t.Errorf("expected unsupported URL scheme error, got: %s", getLastResponse(t))
	}

	// 5. embedImportSubCommand invalid Discohook JSON
	// Inject invalid JSON body response for pastebin/hastebin host
	setMockExtBody(t, []byte(`{"invalid": json`))
	ctxImportBadJSON := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "Invalid embed JSON") {
		t.Errorf("expected invalid JSON error, got: %s", getLastResponse(t))
	}

	// Reset custom HTTP body for subsequent tests
	setMockExtBody(t, nil)

	// 6. embedExportSubCommand non-existent key
	ctxExportNotFound := newTestContext(t, discord.InteractionEvent{
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
	cmdExport := newEmbedExportSubCommand(cm, svc)
	err = cmdExport.Handle(ctxExportNotFound)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "does not exist") {
		t.Errorf("expected does not exist error, got: %s", getLastResponse(t))
	}

	// 7. embedFieldRemoveSubCommand out of bounds index
	ctxRemoveBadIdx := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "invalid field index") {
		t.Errorf("expected invalid field index error, got: %s", getLastResponse(t))
	}

	// 8. embedFieldRemoveSubCommand missing index option
	ctxRemoveNoIdx := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "field index is required") {
		t.Errorf("expected field index required error, got: %s", getLastResponse(t))
	}
}
