package partners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/config"
	localdiscord "github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	partnersvc "github.com/small-frappuccino/discordcore/pkg/discord/partners"
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

	urlStr := req.URL.String()
	if strings.Contains(urlStr, "hastebin.com") || strings.Contains(urlStr, "pastebin.com") {
		status = m.status
		respBody = m.body
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

func newTestContext(t *testing.T, event discord.InteractionEvent, cm config.Provider) *commands.ArikawaContext {
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

type fakeIOStore struct {
	mu     sync.RWMutex
	memory *config.MemoryConfigStore
	writes int
}

func (s *fakeIOStore) Load() (*files.BotConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.memory.Load()
}

func (s *fakeIOStore) Exists() (bool, error) {
	return s.memory.Exists()
}

func (s *fakeIOStore) Save(c *files.BotConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	return s.memory.Save(c)
}

func (s *fakeIOStore) Describe() string {
	return "Fake IO Intercepted Store"
}

func TestPartnerCommands_ConcurrentStateMutation(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)

	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Partners: []files.PartnerEntryConfig{},
		},
	}); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	svc := partnersvc.NewPartnerService(cm)
	addCmd := newPartnerAddSubCommand(cm, svc)
	removeCmd := newPartnerRemoveSubCommand(cm, svc)

	eg, ctx := errgroup.WithContext(context.Background())
	start := make(chan struct{})

	const numRoutines = 50

	for i := 0; i < numRoutines; i++ {
		idx := i
		// Goroutine for Add
		eg.Go(func() error {
			select {
			case <-start:
			case <-ctx.Done():
				return ctx.Err()
			}

			options := []discord.CommandInteractionOption{
				{Name: optionName, Type: discord.StringOptionType, Value: []byte(fmt.Sprintf(`"Partner%d"`, idx))},
				{Name: optionFandom, Type: discord.StringOptionType, Value: []byte(`"Fandom"`)},
				{Name: optionLink, Type: discord.StringOptionType, Value: []byte(`"https://discord.gg/test"`)},
			}

			actx := &commands.ArikawaContext{
				Client: api.NewClient("mockToken"),
				Interaction: &discord.InteractionEvent{
					GuildID: discord.GuildID(12345),
					Member:  &discord.Member{User: discord.User{ID: 999}},
					Data: &discord.CommandInteraction{
						Options: []discord.CommandInteractionOption{
							{
								Type:    discord.SubcommandOptionType,
								Name:    "add",
								Options: options,
							},
						},
					},
				},
				GuildID: discord.GuildID(12345),
				Config:  cm,
			}

			_ = addCmd.Handle(actx)
			return nil
		})

		// Goroutine for Remove
		eg.Go(func() error {
			select {
			case <-start:
			case <-ctx.Done():
				return ctx.Err()
			}

			options := []discord.CommandInteractionOption{
				{Name: optionName, Type: discord.StringOptionType, Value: []byte(fmt.Sprintf(`"Partner%d"`, idx-1))},
			}

			actx := &commands.ArikawaContext{
				Client: api.NewClient("mockToken"),
				Interaction: &discord.InteractionEvent{
					GuildID: discord.GuildID(12345),
					Member:  &discord.Member{User: discord.User{ID: 999}},
					Data: &discord.CommandInteraction{
						Options: []discord.CommandInteractionOption{
							{
								Type:    discord.SubcommandOptionType,
								Name:    "remove",
								Options: options,
							},
						},
					},
				},
				GuildID: discord.GuildID(12345),
				Config:  cm,
			}

			_ = removeCmd.Handle(actx)
			return nil
		})
	}

	close(start) // release the barrier
	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent state mutation execution failed: %v", err)
	}

	finalCfg := cm.GuildConfig("12345")
	if finalCfg == nil {
		t.Fatal("expected config to be present")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	// Verify transactional integrity: no dupes or corrupted entries
	partnerNames := make(map[string]bool)
	for _, p := range finalCfg.PartnerBoard.Partners {
		if strings.HasPrefix(p.Name, "Partner") {
			if partnerNames[p.Name] {
				t.Fatalf("found duplicate partner name: %s", p.Name)
			}
			partnerNames[p.Name] = true
		}
	}
}

func TestPartnerAddSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Partners: []files.PartnerEntryConfig{},
		},
	})
	svc := partnersvc.NewPartnerService(cm)
	cmd := newPartnerAddSubCommand(cm, svc)

	// Check helper methods
	if cmd.Name() != "add" || cmd.Description() == "" {
		t.Error("helper method failure")
	}
	if !cmd.RequiresGuild() || !cmd.RequiresPermissions() || len(cmd.Options()) == 0 {
		t.Error("invariants check failure")
	}

	// Test validation error: empty options
	ctx := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "add",
					Options: []discord.CommandInteractionOption{
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`""`)},
						{Name: optionFandom, Type: discord.StringOptionType, Value: []byte(`"Fandom"`)},
						{Name: optionLink, Type: discord.StringOptionType, Value: []byte(`"https://discord.gg/test"`)},
					},
				},
			},
		},
	}, cm)

	err := cmd.Handle(ctx)
	if err != nil {
		t.Errorf("unexpected execution error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected validation failure response, got: %s", getLastResponse(t))
	}

	// Test success
	ctx2 := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "add",
					Options: []discord.CommandInteractionOption{
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"Partner1"`)},
						{Name: optionFandom, Type: discord.StringOptionType, Value: []byte(`"Fandom"`)},
						{Name: optionLink, Type: discord.StringOptionType, Value: []byte(`"https://discord.gg/test"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctx2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	// Verify it was added
	cfg := cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Partners) != 1 || cfg.PartnerBoard.Partners[0].Name != "Partner1" {
		t.Errorf("partner was not added successfully: %+v", cfg.PartnerBoard.Partners)
	}

	// Test already exists
	err = cmd.Handle(ctx2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected duplicate failure response, got: %s", getLastResponse(t))
	}

	// Test guild not found
	ctxNoGuild := newTestContext(t, discord.InteractionEvent{
		GuildID: 99999,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "add",
					Options: []discord.CommandInteractionOption{
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"PartnerUnique"`)},
						{Name: optionFandom, Type: discord.StringOptionType, Value: []byte(`"Fandom"`)},
						{Name: optionLink, Type: discord.StringOptionType, Value: []byte(`"https://discord.gg/test"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxNoGuild)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected missing guild failure response, got: %s", getLastResponse(t))
	}
}

func TestPartnerRemoveSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Partners: []files.PartnerEntryConfig{
				{Name: "Partner1", Fandom: "Fandom", Link: "https://discord.gg/test"},
			},
		},
	})
	svc := partnersvc.NewPartnerService(cm)
	cmd := newPartnerRemoveSubCommand(cm, svc)

	// Check helper methods
	if cmd.Name() != "remove" || cmd.Description() == "" || len(cmd.Options()) == 0 {
		t.Error("helper method failure")
	}
	if !cmd.RequiresGuild() || !cmd.RequiresPermissions() {
		t.Error("requires guild/perms error")
	}

	// Test remove non-existent
	ctxNonExistent := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "remove",
					Options: []discord.CommandInteractionOption{
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"NonExistent"`)},
					},
				},
			},
		},
	}, cm)

	err := cmd.Handle(ctxNonExistent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}

	// Test autocomplete
	ctxAutocomplete := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.AutocompleteInteraction{
			Name: "partner",
			Options: []discord.AutocompleteOption{
				{
					Name: "remove",
					Type: discord.SubcommandOptionType,
					Options: []discord.AutocompleteOption{
						{Name: "name", Type: discord.StringOptionType, Value: []byte(`"Part"`), Focused: true},
					},
				},
			},
		},
	}, cm)

	choices, err := cmd.Autocomplete(ctxAutocomplete)
	if err != nil {
		t.Errorf("autocomplete error: %v", err)
	}
	strChoices, ok := choices.(api.AutocompleteStringChoices)
	if !ok {
		t.Error("expected AutocompleteStringChoices type")
	}
	if len(strChoices) == 0 {
		t.Error("expected autocomplete choices")
	}

	// Test remove success
	ctxSuccess := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "remove",
					Options: []discord.CommandInteractionOption{
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"Partner1"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxSuccess)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	// Verify removal
	cfg := cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Partners) != 0 {
		t.Error("partner was not removed")
	}
}

func TestPartnerLinkSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Partners: []files.PartnerEntryConfig{
				{Name: "Partner1", Fandom: "Fandom", Link: "https://discord.gg/test"},
			},
		},
	})
	svc := partnersvc.NewPartnerService(cm)
	cmd := newPartnerLinkSubCommand(cm, svc)

	// Check helper methods
	if cmd.Name() != "link" || !cmd.RequiresGuild() || !cmd.RequiresPermissions() || len(cmd.Options()) == 0 {
		t.Error("helper method failure")
	}

	// Test autocomplete
	ctxAutocomplete := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.AutocompleteInteraction{
			Name: "partner",
			Options: []discord.AutocompleteOption{
				{
					Name: "link",
					Type: discord.SubcommandOptionType,
					Options: []discord.AutocompleteOption{
						{Name: "name", Type: discord.StringOptionType, Value: []byte(`"Part"`), Focused: true},
					},
				},
			},
		},
	}, cm)

	choices, err := cmd.Autocomplete(ctxAutocomplete)
	if err != nil {
		t.Errorf("autocomplete error: %v", err)
	}
	strChoices, ok := choices.(api.AutocompleteStringChoices)
	if !ok {
		t.Error("expected AutocompleteStringChoices type")
	}
	if len(strChoices) == 0 {
		t.Error("expected autocomplete choices")
	}

	// Test success
	ctx := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "link",
					Options: []discord.CommandInteractionOption{
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"Partner1"`)},
						{Name: optionLink, Type: discord.StringOptionType, Value: []byte(`"https://discord.gg/new"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	cfg := cm.GuildConfig("12345")
	if cfg.PartnerBoard.Partners[0].Link != "https://discord.gg/new" {
		t.Error("link was not updated")
	}

	// Test error non-existent
	ctxNonExistent := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "link",
					Options: []discord.CommandInteractionOption{
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"NonExistent"`)},
						{Name: optionLink, Type: discord.StringOptionType, Value: []byte(`"https://discord.gg/new"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxNonExistent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}
}

func TestPartnerRenameSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Partners: []files.PartnerEntryConfig{
				{Name: "Partner1", Fandom: "Fandom1", Link: "https://discord.gg/test"},
				{Name: "Partner2", Fandom: "Fandom2", Link: "https://discord.gg/test2"},
			},
		},
	})
	svc := partnersvc.NewPartnerService(cm)
	cmd := newPartnerRenameSubCommand(cm, svc)

	// Check helper methods
	if cmd.Name() != "rename" || !cmd.RequiresGuild() || !cmd.RequiresPermissions() || len(cmd.Options()) == 0 {
		t.Error("helper method failure")
	}

	// Test autocomplete
	ctxAutocomplete := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.AutocompleteInteraction{
			Name: "partner",
			Options: []discord.AutocompleteOption{
				{
					Name: "rename",
					Type: discord.SubcommandOptionType,
					Options: []discord.AutocompleteOption{
						{Name: optionCurrentName, Type: discord.StringOptionType, Value: []byte(`"Part"`), Focused: true},
					},
				},
			},
		},
	}, cm)

	choices, err := cmd.Autocomplete(ctxAutocomplete)
	if err != nil {
		t.Errorf("autocomplete error: %v", err)
	}
	strChoices, ok := choices.(api.AutocompleteStringChoices)
	if !ok {
		t.Error("expected AutocompleteStringChoices type")
	}
	if len(strChoices) == 0 {
		t.Error("expected autocomplete choices")
	}

	// Test success renaming name and fandom
	ctx := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "rename",
					Options: []discord.CommandInteractionOption{
						{Name: optionCurrentName, Type: discord.StringOptionType, Value: []byte(`"Partner1"`)},
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"PartnerNew"`)},
						{Name: optionFandom, Type: discord.StringOptionType, Value: []byte(`"FandomNew"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	cfg := cm.GuildConfig("12345")
	if cfg.PartnerBoard.Partners[0].Name != "PartnerNew" || cfg.PartnerBoard.Partners[0].Fandom != "FandomNew" {
		t.Errorf("rename failed: %+v", cfg.PartnerBoard.Partners[0])
	}

	// Test rename to existing name error
	ctxExists := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "rename",
					Options: []discord.CommandInteractionOption{
						{Name: optionCurrentName, Type: discord.StringOptionType, Value: []byte(`"PartnerNew"`)},
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"Partner2"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxExists)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected failure response, got: %s", getLastResponse(t))
	}

	// Test non-existent partner rename error
	ctxNonExistent := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "rename",
					Options: []discord.CommandInteractionOption{
						{Name: optionCurrentName, Type: discord.StringOptionType, Value: []byte(`"NonExistent"`)},
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`"Something"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxNonExistent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected failure response, got: %s", getLastResponse(t))
	}

	// Test empty new name error
	ctxEmptyName := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "rename",
					Options: []discord.CommandInteractionOption{
						{Name: optionCurrentName, Type: discord.StringOptionType, Value: []byte(`"PartnerNew"`)},
						{Name: optionName, Type: discord.StringOptionType, Value: []byte(`""`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxEmptyName)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected failure response, got: %s", getLastResponse(t))
	}
}

func TestPartnerListSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Partners: []files.PartnerEntryConfig{},
		},
	})
	cmd := newPartnerListSubCommand(cm)

	// Check helper methods
	if cmd.Name() != "list" || !cmd.RequiresGuild() || !cmd.RequiresPermissions() || cmd.Options() != nil {
		t.Error("helper method failure")
	}

	// Empty list test
	ctxEmpty := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{Type: discord.SubcommandOptionType, Name: "list"},
			},
		},
	}, cm)

	err := cmd.Handle(ctxEmpty)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") && !strings.Contains(getLastResponse(t), "No partners") {
		t.Errorf("expected success/empty response, got: %s", getLastResponse(t))
	}

	// Non-empty list test
	_, _ = cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds[0].PartnerBoard.Partners = []files.PartnerEntryConfig{
			{Name: "P1", Fandom: "F1", Link: "L1"},
		}
		return nil
	})

	err = cmd.Handle(ctxEmpty)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "Partner List") {
		t.Errorf("expected Partner List in response, got: %s", getLastResponse(t))
	}

	// Missing guild config test
	ctxNoGuild := newTestContext(t, discord.InteractionEvent{
		GuildID: 99999,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{Type: discord.SubcommandOptionType, Name: "list"},
			},
		},
	}, cm)

	err = cmd.Handle(ctxNoGuild)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}
}

func TestPartnerPostSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Postings: []files.CustomEmbedPostingConfig{},
		},
	})
	svc := partnersvc.NewPartnerService(cm)
	cmd := newPartnerPostSubCommand(cm, svc)

	// Check helper methods
	if cmd.Name() != "post" || !cmd.RequiresGuild() || !cmd.RequiresPermissions() || len(cmd.Options()) == 0 {
		t.Error("helper method failure")
	}

	// Test webhook invalid URL
	ctxBadWebhook := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "post",
					Options: []discord.CommandInteractionOption{
						{Name: optionWebhookURL, Type: discord.StringOptionType, Value: []byte(`"http://invalid.url"`)},
					},
				},
			},
		},
	}, cm)

	err := cmd.Handle(ctxBadWebhook)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}

	// Test webhook success
	ctxGoodWebhook := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "post",
					Options: []discord.CommandInteractionOption{
						{Name: optionWebhookURL, Type: discord.StringOptionType, Value: []byte(`"https://discord.com/api/webhooks/11111/aaaaa"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxGoodWebhook)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	cfg := cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Postings) != 1 || cfg.PartnerBoard.Postings[0].WebhookID != "11111" {
		t.Errorf("webhook not added: %+v", cfg.PartnerBoard.Postings)
	}

	// Test webhook duplicate
	err = cmd.Handle(ctxGoodWebhook)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}

	// Test channel register success
	ctxChannel := newTestContext(t, discord.InteractionEvent{
		GuildID:   12345,
		ChannelID: 54321,
		Member:    &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type:    discord.SubcommandOptionType,
					Name:    "post",
					Options: []discord.CommandInteractionOption{},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxChannel)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	cfg = cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Postings) != 2 || cfg.PartnerBoard.Postings[1].ChannelID != "54321" {
		t.Errorf("channel not added: %+v", cfg.PartnerBoard.Postings)
	}

	// Test channel duplicate
	err = cmd.Handle(ctxChannel)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}
}

func TestPartnerUnpostSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
		PartnerBoard: files.PartnerBoardConfig{
			Postings: []files.CustomEmbedPostingConfig{
				{ChannelID: "54321", MessageID: "99999"},
				{WebhookID: "11111", WebhookToken: "aaaaa", MessageID: "88888"},
			},
		},
	})
	cmd := newPartnerUnpostSubCommand(cm)

	// Check helper methods
	if cmd.Name() != "unpost" || !cmd.RequiresGuild() || !cmd.RequiresPermissions() || len(cmd.Options()) == 0 {
		t.Error("helper method failure")
	}

	// Test error: no options provided
	ctxEmpty := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type:    discord.SubcommandOptionType,
					Name:    "unpost",
					Options: []discord.CommandInteractionOption{},
				},
			},
		},
	}, cm)

	err := cmd.Handle(ctxEmpty)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}

	// Test error: bad webhook URL
	ctxBadWebhook := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "unpost",
					Options: []discord.CommandInteractionOption{
						{Name: optionWebhookURL, Type: discord.StringOptionType, Value: []byte(`"http://bad.url"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxBadWebhook)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}

	// Test webhook unpost success
	ctxWebhook := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "unpost",
					Options: []discord.CommandInteractionOption{
						{Name: optionWebhookURL, Type: discord.StringOptionType, Value: []byte(`"https://discord.com/api/webhooks/11111/aaaaa"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxWebhook)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	cfg := cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Postings) != 1 || cfg.PartnerBoard.Postings[0].MessageID != "99999" {
		t.Errorf("webhook posting was not removed: %+v", cfg.PartnerBoard.Postings)
	}

	// Test message ID unpost success
	ctxMsg := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "unpost",
					Options: []discord.CommandInteractionOption{
						{Name: optionMessageID, Type: discord.StringOptionType, Value: []byte(`"99999"`)},
					},
				},
			},
		},
	}, cm)

	err = cmd.Handle(ctxMsg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	cfg = cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Postings) != 0 {
		t.Errorf("message posting was not removed: %+v", cfg.PartnerBoard.Postings)
	}
}

func TestPartnerRefreshSubCommand(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	svc := partnersvc.NewPartnerService(cm)
	cmd := newPartnerRefreshSubCommand(cm, svc)

	// Check helper methods
	if cmd.Name() != "refresh" || !cmd.RequiresGuild() || !cmd.RequiresPermissions() || cmd.Options() != nil {
		t.Error("helper method failure")
	}

	ctx := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{Type: discord.SubcommandOptionType, Name: "refresh"},
			},
		},
	}, cm)

	err := cmd.Handle(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}
}

func TestPartnerTemplates(t *testing.T) {
	t.Parallel()
	// 1. Test Import Template Success
	resetMockHTTP(t)
	store := &fakeIOStore{memory: &config.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})

	importCmd := newPartnerImportTemplateSubCommand(cm)
	if importCmd.Name() != "import_template" || !importCmd.RequiresGuild() || !importCmd.RequiresPermissions() || len(importCmd.Options()) == 0 {
		t.Error("helper method failure")
	}

	validJSON := `{
		"embeds": [{
			"title": "My Custom Title",
			"description": "My intro template"
		}]
	}`

	setMockStatusAndBody(t, http.StatusOK, []byte(validJSON))

	ctxImport := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "import_template",
					Options: []discord.CommandInteractionOption{
						{Name: optionURL, Type: discord.StringOptionType, Value: []byte(`"https://hastebin.com/raw/abcdef"`)},
					},
				},
			},
		},
	}, cm)

	err := importCmd.Handle(ctxImport)
	if err != nil {
		t.Errorf("unexpected error on import: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	var hastebinRequest *http.Request
	for _, req := range getMockReqs(t) {
		if strings.Contains(req.URL.String(), "hastebin.com") {
			hastebinRequest = req
		}
	}

	if hastebinRequest == nil {
		t.Fatal("expected a request to hastebin.com, but none was recorded")
	}
	if hastebinRequest.URL.String() != "https://hastebin.com/raw/abcdef" {
		t.Errorf("unexpected request URL: %s", hastebinRequest.URL.String())
	}

	cfg := cm.GuildConfig("12345")
	if cfg.PartnerBoard.Template.Title != "My Custom Title" {
		t.Errorf("template was not imported properly: %+v", cfg.PartnerBoard.Template)
	}

	// 2. Test Import Template Failures
	// Provider error
	resetMockHTTP(t)
	setMockStatusAndBody(t, http.StatusInternalServerError, []byte(`{}`))
	ctxImport = newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "import_template",
					Options: []discord.CommandInteractionOption{
						{Name: optionURL, Type: discord.StringOptionType, Value: []byte(`"https://hastebin.com/raw/abcdef"`)},
					},
				},
			},
		},
	}, cm)
	err = importCmd.Handle(ctxImport)
	if err != nil {
		t.Errorf("unexpected execution error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}

	// Invalid JSON
	resetMockHTTP(t)
	setMockStatusAndBody(t, http.StatusOK, []byte(`{invalid json`))
	ctxImport = newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Type: discord.SubcommandOptionType,
					Name: "import_template",
					Options: []discord.CommandInteractionOption{
						{Name: optionURL, Type: discord.StringOptionType, Value: []byte(`"https://hastebin.com/raw/abcdef"`)},
					},
				},
			},
		},
	}, cm)
	err = importCmd.Handle(ctxImport)
	if err != nil {
		t.Errorf("unexpected execution error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}

	// 3. Test Export Template Success
	resetMockHTTP(t)
	exportCmd := newPartnerExportTemplateSubCommand(cm)
	if exportCmd.Name() != "export_template" || !exportCmd.RequiresGuild() || !exportCmd.RequiresPermissions() || exportCmd.Options() != nil {
		t.Error("helper method failure")
	}

	// Set template on config
	_, _ = cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds[0].PartnerBoard.Template.Title = "Export Title"
		return nil
	})

	setMockStatusAndBody(t, http.StatusOK, []byte(`{"key": "exportkey"}`))

	ctxExport := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member: &discord.Member{
			User: discord.User{ID: 999},
		},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{Type: discord.SubcommandOptionType, Name: "export_template"},
			},
		},
	}, cm)

	err = exportCmd.Handle(ctxExport)
	if err != nil {
		t.Errorf("unexpected error on export: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse(t))
	}

	var hastebinDocReq *http.Request
	var hastebinDocBody []byte
	reqs := getMockReqs(t)
	bodies := getMockReqBodies(t)
	for idx, req := range reqs {
		if strings.Contains(req.URL.String(), "hastebin.com/documents") {
			hastebinDocReq = req
			hastebinDocBody = bodies[idx]
		}
	}

	if hastebinDocReq == nil {
		t.Fatal("expected request to hastebin.com/documents, but none was recorded")
	}

	var parsedExported map[string]interface{}
	_ = json.Unmarshal(hastebinDocBody, &parsedExported)
	// The export function builds a full DiscohookJSON containing the partner template
	// Let's extract embeds[0].title
	embeds, ok := parsedExported["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Fatalf("invalid exported structure: %s", string(hastebinDocBody))
	}
	embedObj, ok := embeds[0].(map[string]interface{})
	if !ok || embedObj["title"] != "Export Title" {
		t.Errorf("exported wrong content: %s", string(hastebinDocBody))
	}

	// Export failure: non-admin member
	resetMockHTTP(t)
	ctxNonAdminExport := newTestContext(t, discord.InteractionEvent{
		GuildID: 12345,
		Member: &discord.Member{
			User: discord.User{ID: 999},
		},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{Type: discord.SubcommandOptionType, Name: "export_template"},
			},
		},
	}, cm)

	// Make config have global credentials so admin check runs
	_, _ = cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.RuntimeConfig.PastebinDevKey = "devkey"
		cfg.RuntimeConfig.PastebinUserName = "username"
		cfg.RuntimeConfig.PastebinUserPassword = "password"
		return nil
	})

	err = exportCmd.Handle(ctxNonAdminExport)
	if err != nil {
		t.Errorf("unexpected execution error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse(t))
	}
}
