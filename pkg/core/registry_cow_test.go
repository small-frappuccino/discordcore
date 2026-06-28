package core

import (
	"context"
	"strconv"
	"sync"
	"testing"
)

func TestInMemoryFeatureRegistry(t *testing.T) {
	registry := NewInMemoryFeatureRegistry()

	bot1 := BotInstance{ApplicationID: "bot1", GuildID: "guild1", Token: "token1"}
	bot2 := BotInstance{ApplicationID: "bot2", GuildID: "guild1", Token: "token2"}

	// Test initial state
	ctx := context.Background()
	_, err := registry.ResolveOwner(ctx, "guild1", FeatureBan)
	if !errorsIs(err, ErrFeatureNotAssigned) {
		t.Fatalf("expected ErrFeatureNotAssigned, got %v", err)
	}

	// Test update route
	if err := registry.UpdateRoute("guild1", FeatureBan, bot1); err != nil {
		t.Fatalf("failed to update route: %v", err)
	}
	resolved, err := registry.ResolveOwner(ctx, "guild1", FeatureBan)
	if err != nil {
		t.Fatalf("failed to resolve owner: %v", err)
	}
	if resolved != bot1 {
		t.Fatalf("resolved incorrect bot instance: %+v", resolved)
	}

	// Route theft prevention
	err = registry.UpdateRoute("guild1", FeatureBan, bot2)
	if !errorsIs(err, ErrRouteTheft) {
		t.Fatalf("expected ErrRouteTheft, got %v", err)
	}

	// Test removing route to allow new bot
	if err := registry.RemoveRoute("guild1", FeatureBan); err != nil {
		t.Fatalf("failed to remove route: %v", err)
	}

	// Now bot2 can claim it
	if err := registry.UpdateRoute("guild1", FeatureBan, bot2); err != nil {
		t.Fatalf("failed to update route for bot2: %v", err)
	}

	// Test cardinality (max 5 distinct bots)
	registry.UpdateRoute("guild1", FeatureKick, BotInstance{ApplicationID: "bot3"})
	registry.UpdateRoute("guild1", FeatureTimeout, BotInstance{ApplicationID: "bot4"})
	registry.UpdateRoute("guild1", FeatureDeafen, BotInstance{ApplicationID: "bot5"})
	registry.UpdateRoute("guild1", FeatureMoveMember, BotInstance{ApplicationID: "bot6"})

	err = registry.UpdateRoute("guild1", FeatureMsgDelete, BotInstance{ApplicationID: "bot7"})
	if !errorsIs(err, ErrGuildCapReached) {
		t.Fatalf("expected ErrGuildCapReached, got %v", err)
	}

	// Test concurrent reads and single-writer validation (CoW validation)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_, _ = registry.ResolveOwner(ctx, "guild1", FeatureBan)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		registry.RemoveRoute("guild1", FeatureBan) // Remove first to prevent theft err
		registry.UpdateRoute("guild1", FeatureBan, bot1)
	}()

	wg.Wait()

	// Check final state after updates
	finalBot, err := registry.ResolveOwner(ctx, "guild1", FeatureBan)
	if err != nil {
		t.Fatalf("failed to resolve owner in final state: %v", err)
	}
	if finalBot != bot1 {
		t.Fatalf("expected final bot to be bot1, got: %+v", finalBot)
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
		_ = registry.UpdateRoute(guildID, FeatureBan, BotInstance{GuildID: guildID, ApplicationID: "app"})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = registry.ResolveOwner(ctx, "guild500", FeatureBan)
		}
	})
}

// Benchmark the Single Writer CoW swap performance
func BenchmarkRegistry_UpdateRoute(b *testing.B) {
	b.ReportAllocs()
	registry := NewInMemoryFeatureRegistry()

	// Pre-fill
	for i := 0; i < 1000; i++ {
		guildID := "guild" + strconv.Itoa(i)
		_ = registry.UpdateRoute(guildID, FeatureBan, BotInstance{GuildID: guildID, ApplicationID: "app"})
	}

	b.ResetTimer()
	// Single writer context (no RunParallel)
	bot := BotInstance{GuildID: "guild500", ApplicationID: "app"}
	for i := 0; i < b.N; i++ {
		_ = registry.RemoveRoute("guild500", FeatureBan)
		_ = registry.UpdateRoute("guild500", FeatureBan, bot)
	}
}
