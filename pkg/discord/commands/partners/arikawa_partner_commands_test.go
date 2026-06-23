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
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	partnersvc "github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

var (
	mockHTTPStatus    = http.StatusOK
	mockHTTPBody      = []byte(`{}`)
	mockHTTPReqs      []*http.Request
	mockHTTPReqBodies [][]byte
	mockHTTPMu        sync.Mutex
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

			// Default behavior for Discord is always success (200 OK)
			status := http.StatusOK
			respBody := []byte(`{}`)

			// If it's a provider lookup (Hastebin/Pastebin), apply custom mock settings
			urlStr := req.URL.String()
			if strings.Contains(urlStr, "hastebin.com") || strings.Contains(urlStr, "pastebin.com") {
				status = mockHTTPStatus
				respBody = mockHTTPBody
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

type fakeIOStore struct {
	mu     sync.RWMutex
	memory *files.MemoryConfigStore
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
	time.Sleep(10 * time.Millisecond)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	return s.memory.Save(c)
}

func (s *fakeIOStore) Describe() string {
	return "Fake IO Intercepted Store"
}

func TestPartnerCommands_ConcurrentStateMutation(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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

	var wg sync.WaitGroup
	start := make(chan struct{})

	const numRoutines = 50

	for i := 0; i < numRoutines; i++ {
		wg.Add(2)
		// Goroutine for Add
		go func(idx int) {
			defer wg.Done()
			<-start

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
				Config: cm,
			}

			_ = addCmd.Handle(actx)
		}(i)

		// Goroutine for Remove
		go func(idx int) {
			defer wg.Done()
			<-start

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
				Config: cm,
			}

			time.Sleep(2 * time.Millisecond) // Give Add a tiny head start
			_ = removeCmd.Handle(actx)
		}(i)
	}

	close(start) // release the barrier
	wg.Wait()

	finalCfg := cm.GuildConfig("12345")
	if finalCfg == nil {
		t.Fatal("expected config to be present")
	}

	// Wait for any trailing async I/O
	time.Sleep(10 * time.Millisecond)

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
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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
	ctx := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected validation failure response, got: %s", getLastResponse())
	}

	// Test success
	ctx2 := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected duplicate failure response, got: %s", getLastResponse())
	}

	// Test guild not found
	ctxNoGuild := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected missing guild failure response, got: %s", getLastResponse())
	}
}

func TestPartnerRemoveSubCommand(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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
	ctxNonExistent := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}

	// Test autocomplete
	ctxAutocomplete := newTestContext(discord.InteractionEvent{
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
	ctxSuccess := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}

	// Verify removal
	cfg := cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Partners) != 0 {
		t.Error("partner was not removed")
	}
}

func TestPartnerLinkSubCommand(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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
	ctxAutocomplete := newTestContext(discord.InteractionEvent{
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
	ctx := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}

	cfg := cm.GuildConfig("12345")
	if cfg.PartnerBoard.Partners[0].Link != "https://discord.gg/new" {
		t.Error("link was not updated")
	}

	// Test error non-existent
	ctxNonExistent := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}
}

func TestPartnerRenameSubCommand(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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
	ctxAutocomplete := newTestContext(discord.InteractionEvent{
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
	ctx := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}

	cfg := cm.GuildConfig("12345")
	if cfg.PartnerBoard.Partners[0].Name != "PartnerNew" || cfg.PartnerBoard.Partners[0].Fandom != "FandomNew" {
		t.Errorf("rename failed: %+v", cfg.PartnerBoard.Partners[0])
	}

	// Test rename to existing name error
	ctxExists := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected failure response, got: %s", getLastResponse())
	}

	// Test non-existent partner rename error
	ctxNonExistent := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected failure response, got: %s", getLastResponse())
	}

	// Test empty new name error
	ctxEmptyName := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected failure response, got: %s", getLastResponse())
	}
}

func TestPartnerListSubCommand(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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
	ctxEmpty := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") && !strings.Contains(getLastResponse(), "No partners") {
		t.Errorf("expected success/empty response, got: %s", getLastResponse())
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
	if !strings.Contains(getLastResponse(), "Partner List") {
		t.Errorf("expected Partner List in response, got: %s", getLastResponse())
	}

	// Missing guild config test
	ctxNoGuild := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}
}

func TestPartnerPostSubCommand(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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
	ctxBadWebhook := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}

	// Test webhook success
	ctxGoodWebhook := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}

	// Test channel register success
	ctxChannel := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}
}

func TestPartnerUnpostSubCommand(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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
	ctxEmpty := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}

	// Test error: bad webhook URL
	ctxBadWebhook := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}

	// Test webhook unpost success
	ctxWebhook := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}

	cfg := cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Postings) != 1 || cfg.PartnerBoard.Postings[0].MessageID != "99999" {
		t.Errorf("webhook posting was not removed: %+v", cfg.PartnerBoard.Postings)
	}

	// Test message ID unpost success
	ctxMsg := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}

	cfg = cm.GuildConfig("12345")
	if len(cfg.PartnerBoard.Postings) != 0 {
		t.Errorf("message posting was not removed: %+v", cfg.PartnerBoard.Postings)
	}
}

func TestPartnerRefreshSubCommand(t *testing.T) {
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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

	ctx := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}
}

func TestPartnerTemplates(t *testing.T) {
	// 1. Test Import Template Success
	resetMockHTTP()
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
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

	mockHTTPMu.Lock()
	mockHTTPStatus = http.StatusOK
	mockHTTPBody = []byte(validJSON)
	mockHTTPMu.Unlock()

	ctxImport := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}

	mockHTTPMu.Lock()
	var hastebinRequest *http.Request
	for _, req := range mockHTTPReqs {
		if strings.Contains(req.URL.String(), "hastebin.com") {
			hastebinRequest = req
			break
		}
	}
	mockHTTPMu.Unlock()

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
	resetMockHTTP()
	mockHTTPMu.Lock()
	mockHTTPStatus = http.StatusInternalServerError
	mockHTTPMu.Unlock()
	err = importCmd.Handle(ctxImport)
	if err != nil {
		t.Errorf("unexpected execution error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}

	// Invalid JSON
	resetMockHTTP()
	mockHTTPMu.Lock()
	mockHTTPStatus = http.StatusOK
	mockHTTPBody = []byte(`{invalid json`)
	mockHTTPMu.Unlock()
	err = importCmd.Handle(ctxImport)
	if err != nil {
		t.Errorf("unexpected execution error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}

	// 3. Test Export Template Success
	resetMockHTTP()
	exportCmd := newPartnerExportTemplateSubCommand(cm)
	if exportCmd.Name() != "export_template" || !exportCmd.RequiresGuild() || !exportCmd.RequiresPermissions() || exportCmd.Options() != nil {
		t.Error("helper method failure")
	}

	// Set template on config
	_, _ = cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds[0].PartnerBoard.Template.Title = "Export Title"
		return nil
	})

	mockHTTPMu.Lock()
	mockHTTPStatus = http.StatusOK
	mockHTTPBody = []byte(`{"key": "exportkey"}`)
	mockHTTPMu.Unlock()

	ctxExport := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "✅") {
		t.Errorf("expected success response, got: %s", getLastResponse())
	}

	mockHTTPMu.Lock()
	var hastebinDocReq *http.Request
	var hastebinDocBody []byte
	for idx, req := range mockHTTPReqs {
		if strings.Contains(req.URL.String(), "hastebin.com/documents") {
			hastebinDocReq = req
			hastebinDocBody = mockHTTPReqBodies[idx]
			break
		}
	}
	mockHTTPMu.Unlock()

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
	resetMockHTTP()
	ctxNonAdminExport := newTestContext(discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(), "❌") {
		t.Errorf("expected error response, got: %s", getLastResponse())
	}
}
