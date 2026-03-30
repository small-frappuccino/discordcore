package logging

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

func TestScheduleStartupMemberWarmupDispatchesToTaskRouter(t *testing.T) {
	origWarmup := monitoringWarmupTaskFn
	t.Cleanup(func() {
		monitoringWarmupTaskFn = origWarmup
	})

	called := make(chan cache.WarmupConfig, 1)
	monitoringWarmupTaskFn = func(ctx context.Context, session *discordgo.Session, unifiedCache *cache.UnifiedCache, store *storage.Store, config cache.WarmupConfig) error {
		called <- config
		return nil
	}

	router := task.NewRouter(task.Defaults())
	t.Cleanup(router.Close)

	ms := &MonitoringService{
		router:       router,
		runCtx:       context.Background(),
		isRunning:    true,
		unifiedCache: &cache.UnifiedCache{},
	}
	ms.registerStartupWarmupHandler(ms.runCtx)

	config := cache.WarmupConfig{
		FetchMissingMembers:  true,
		FetchMissingGuilds:   false,
		FetchMissingRoles:    false,
		FetchMissingChannels: false,
		MaxMembersPerGuild:   500,
	}
	if !ms.ScheduleStartupMemberWarmup(config) {
		t.Fatalf("expected member warmup to be scheduled")
	}

	select {
	case got := <-called:
		if !reflect.DeepEqual(got, config) {
			t.Fatalf("unexpected warmup config: got %+v want %+v", got, config)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for warmup dispatch")
	}
}
