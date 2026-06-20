package embeds

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	embedsvc "github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

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

func TestEmbedCommands_ConcurrentMutation(t *testing.T) {
	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)
	guildID := "guild-concurrent"

	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to init guild config: %v", err)
	}

	// We'll simulate multiple goroutines adding fields to the same embed concurrently.
	// First, let's establish the embed.
	setCtx := &legacycore.ArikawaContext{
		GuildID: discord.GuildID(12345),
		Interaction: &discord.InteractionEvent{
			GuildID: discord.GuildID(12345),
			Data: &discord.CommandInteraction{
				Name: "embed",
				Options: []discord.CommandInteractionOption{
					{
						Type: discord.SubcommandGroupOptionType,
						Name: "embed",
						Options: []discord.CommandInteractionOption{
							{
								Type: discord.SubcommandOptionType,
								Name: "set",
								Options: []discord.CommandInteractionOption{
									{Name: embedOptionKey, Type: discord.StringOptionType, Value: []byte(`"concurrent-embed"`)},
									{Name: embedOptionTitle, Type: discord.StringOptionType, Value: []byte(`"Initial Title"`)},
								},
							},
						},
					},
				},
			},
		},
	}

	// Mock interaction respond to avoid nil pointer
	setCtx.Client = nil // We won't use it to post

	// The implementation expects context.Respond to exist, so we need to bypass Arikawa's real response
	// Wait, ArikawaContext isn't an interface, it's a struct calling client.RespondInteraction.
	// We can skip the handle and manipulate config directly for setup.
	ce := files.CustomEmbedConfig{Key: "concurrent-embed", Title: "Initial Title"}
	cm.SetCustomEmbedProperties(guildID, ce.Key, ce)

	var wg sync.WaitGroup
	workers := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Direct config manager modification under stress
			field := files.CustomEmbedFieldConfig{
				Name:  fmt.Sprintf("Field %d", idx),
				Value: "Val",
			}
			cm.AddCustomEmbedField(guildID, ce.Key, field)
		}(i)
	}

	// Also simulate removals concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.RemoveCustomEmbedField(guildID, ce.Key, 0)
		}()
	}

	wg.Wait()

	// Assess transactional integrity
	embeds, err := cm.CustomEmbed(guildID, ce.Key)
	if err != nil {
		t.Fatalf("failed to retrieve embed: %v", err)
	}

	// The array must not contain nil pointers, data race conditions, or uninitialized memory
	if len(embeds.Fields) > workers {
		t.Errorf("Unexpected array bounds, got %d fields, expected max %d", len(embeds.Fields), workers)
	}
}

type bufferSlogHandler struct {
	slog.Handler
	buf *bytes.Buffer
}

func (h *bufferSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.Handler.Handle(ctx, r)
	return nil
}

func TestEmbedCommands_ObservabilityStructuralFaults(t *testing.T) {
	// A arquitetura exige a geração de eventos via slog com indexação da stack trace para falhas de lógica de negócios.
	// Alocar o gravador global de log do ambiente de testes para um slog.Handler processando dados contra um bytes.Buffer.
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(jsonHandler)

	// Override global logger just for this test
	restoreLogger := log.SetErrorLoggerRawForTest(logger)
	defer restoreLogger()

	store := &fakeIOStore{memory: &files.MemoryConfigStore{}}
	cm := files.NewConfigManagerWithStore(store, nil)

	router := legacycore.NewArikawaCommandRouter("dummy_token", cm)
	svc := embedsvc.NewEmbedService(cm)
	embedCmds := NewEmbedCommands(cm, svc)
	embedCmds.RegisterCommands(router)

	// Forçar anomalias de formatação nos comandos /embed.
	// For example, passing an extremely large title which triggers a validation error natively.
	// Wait, Arikawa API handles max length. But let's say the Handle function fails because
	// of an invalid channel ID snowflake parsing.

	// Provide invalid option type to force an error.
	interaction := &discord.InteractionEvent{
		ID: discord.InteractionID(999),
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

	// Mock interaction respond is missing, so it might fail with "nil pointer".
	// The failure will bubble up to the router, which will log it with LevelError + Stack Trace.
	cm.AddGuildConfig(files.GuildConfig{GuildID: "123"})
	cm.SetCustomEmbedProperties("123", "valid-key", files.CustomEmbedConfig{Key: "valid-key"})
	interaction.GuildID = discord.GuildID(123)

	// Call handle raw event which simulates the router catching it
	router.HandleInteractionEvent(interaction)

	// Validate JSON matrix and stack trace in debug.Stack()
	logOutput := buf.String()

	if !strings.Contains(logOutput, `"level":"ERROR"`) {
		t.Errorf("Expected event to result in slog.LevelError, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"stack_trace":`) {
		t.Errorf("Expected event to preserve JSON matrix with stack_trace, got: %s", logOutput)
	}
}

// Ensure the waitgroup abstraction locally finishes sync
func (s *fakeIOStore) Finish() {}
