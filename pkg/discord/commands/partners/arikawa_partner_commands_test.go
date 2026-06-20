package partners

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	partnersvc "github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

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

type fakeArikawaClient struct {
	*api.Client
}

func (c *fakeArikawaClient) RespondInteraction(interactionID discord.InteractionID, token string, data api.InteractionResponse) error {
	return nil // Mock response
}

func TestPartnerCommands_ConcurrentStateMutation(t *testing.T) {
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

	client := &fakeArikawaClient{Client: api.NewClient("fake")}

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

			actx := &legacycore.ArikawaContext{
				Client: client.Client,
				Interaction: &discord.InteractionEvent{
					GuildID: discord.GuildID(12345),
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

			actx := &legacycore.ArikawaContext{
				Client: client.Client,
				Interaction: &discord.InteractionEvent{
					GuildID: discord.GuildID(12345),
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

	t.Logf("Final partners count: %d. Total I/O writes: %d", len(finalCfg.PartnerBoard.Partners), store.writes)
}
