package app

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/partners"
)

func TestBotPartnerSyncDispatcherStartIsLazy(t *testing.T) {
	t.Parallel()

	dispatcher := newBotPartnerSyncDispatcher(
		files.NewMemoryConfigManager(),
		&partners.BoardSyncService{},
		map[string]*botRuntime{
			"main": {instanceID: "main"},
		},
		"main",
	)
	if err := dispatcher.Start(); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	if !dispatcher.running {
		t.Fatal("expected dispatcher to be marked as running")
	}
	if got := len(dispatcher.coordinators); got != 0 {
		t.Fatalf("expected no eager partner auto-sync coordinators, got %d", got)
	}
}
