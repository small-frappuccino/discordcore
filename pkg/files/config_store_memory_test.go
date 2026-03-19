package files

import "testing"

func TestMemoryConfigStoreRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryConfigStore()

	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("exists before save: %v", err)
	}
	if exists {
		t.Fatal("expected empty memory store to report exists=false")
	}

	cfg := &BotConfig{
		Guilds: []GuildConfig{{
			GuildID: "g1",
			Channels: ChannelsConfig{
				Commands: "c1",
			},
		}},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	exists, err = store.Exists()
	if err != nil {
		t.Fatalf("exists after save: %v", err)
	}
	if !exists {
		t.Fatal("expected saved memory store to report exists=true")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(loaded.Guilds) != 1 || loaded.Guilds[0].Channels.Commands != "c1" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestMemoryConfigStoreReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	store := NewMemoryConfigStore()
	if err := store.Save(&BotConfig{
		Guilds: []GuildConfig{{
			GuildID: "g1",
			Channels: ChannelsConfig{
				MessageDelete: "c1",
			},
		}},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	first, err := store.Load()
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	second, err := store.Load()
	if err != nil {
		t.Fatalf("second load: %v", err)
	}

	first.Guilds[0].Channels.MessageDelete = "mutated"
	if second.Guilds[0].Channels.MessageDelete != "c1" {
		t.Fatalf("expected independent config copies, got %+v", second.Guilds[0].Channels)
	}
}
