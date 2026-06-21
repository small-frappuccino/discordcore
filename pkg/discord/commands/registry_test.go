package commands_test

import (
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
)

// mockArikawaCommand is a simple structural mock satisfying the ArikawaCommand interface.
type mockArikawaCommand struct {
	name string
}

func (m *mockArikawaCommand) Name() string                              { return m.name }
func (m *mockArikawaCommand) Description() string                       { return "Mock Command" }
func (m *mockArikawaCommand) Options() []discord.CommandOption          { return nil }
func (m *mockArikawaCommand) Handle(ctx *commands.ArikawaContext) error { return nil }
func (m *mockArikawaCommand) RequiresGuild() bool                       { return false }
func (m *mockArikawaCommand) RequiresPermissions() bool                 { return false }

// TestCommandRegistry_ConcurrentSafety validates the thread-safety invariants
// of the command registry, explicitly hunting for data races during simultaneous
// reads and writes by utilizing t.Parallel().
func TestCommandRegistry_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	registry := commands.NewCommandRegistry()
	var wg sync.WaitGroup

	// Stress-testing state mutation under high concurrency
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cmd := &mockArikawaCommand{name: "test_cmd"}

			// Operational Annotation: We execute writes simultaneously across
			// hundreds of goroutines. The underlying RWMutex must serialize
			// these strictly to prevent memory corruption.
			registry.Register(cmd)

			// Simultaneous reads to force race detection if read locks are missing
			_ = registry.Len()
			_, _ = registry.GetCommand("test_cmd")
		}(i)
	}

	wg.Wait()

	if registry.Len() == 0 {
		t.Fatal("Registry failed to record commands concurrently")
	}
}
