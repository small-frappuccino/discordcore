package core

import (
	"context"
	"strconv"
	"sync"
	"testing"
)

func TestInMemoryFeatureRegistry(t *testing.T) {
	registry := NewInMemoryFeatureRegistry()

	bot1 := &BotInstance{ApplicationID: "bot1", GuildID: "guild1", Token: "token1"}
	bot2 := &BotInstance{ApplicationID: "bot2", GuildID: "guild2", Token: "token2"}

	// Test initial state
	ctx := context.Background()
	_, err := registry.ResolveOwner(ctx, "guild1", "moderation")
	if !errorsIs(err, ErrFeatureNotAssigned) {
		t.Fatalf("expected ErrFeatureNotAssigned, got %v", err)
	}

	// Test update route
	registry.UpdateRoute("guild1", "moderation", bot1)
	resolved, err := registry.ResolveOwner(ctx, "guild1", "moderation")
	if err != nil {
		t.Fatalf("failed to resolve owner: %v", err)
	}
	if resolved != bot1 {
		t.Fatalf("resolved incorrect bot instance: %+v", resolved)
	}

	// Test concurrent reads and updates (CoW validation)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_, _ = registry.ResolveOwner(ctx, "guild1", "moderation")
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		registry.UpdateRoute("guild1", "moderation", bot2)
	}()

	wg.Wait()

	// Check final state after updates
	finalBot, err := registry.ResolveOwner(ctx, "guild1", "moderation")
	if err != nil {
		t.Fatalf("failed to resolve owner in final state: %v", err)
	}
	if finalBot != bot2 {
		t.Fatalf("expected final bot to be bot2, got: %+v", finalBot)
	}

	// Test remove route
	registry.RemoveRoute("guild1", "moderation")
	_, err = registry.ResolveOwner(ctx, "guild1", "moderation")
	if !errorsIs(err, ErrFeatureNotAssigned) {
		t.Fatalf("expected ErrFeatureNotAssigned after route removal, got %v", err)
	}
}

func errorsIs(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}
	return err.Error() == target.Error()
}

// BenchmarkPrimitives
func BenchmarkRegistry_ResolveOwner(b *testing.B) {
	b.ReportAllocs()
	registry := NewInMemoryFeatureRegistry()
	ctx := context.Background()

	// Pre-fill
	for i := 0; i < 1000; i++ {
		guildID := "guild" + strconv.Itoa(i)
		registry.UpdateRoute(guildID, "moderation", &BotInstance{GuildID: guildID})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = registry.ResolveOwner(ctx, "guild500", "moderation")
		}
	})
}
