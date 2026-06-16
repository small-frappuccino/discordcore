package app

import (
	"context"
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
)

func TestDualSDKPublisher_ResolutionFailure(t *testing.T) {
	resolver := newBotRuntimeResolver(files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil), make(map[string]*botRuntime))
	// Unregistered guild will fail to resolve
	pub := newDualSDKPublisher(resolver)

	_, err := pub.getArikawaPublisher("guild-1")
	if err == nil || !strings.Contains(err.Error(), "resolve bot instance") {
		t.Fatalf("expected resolution failure, got: %v", err)
	}

	// These should also fail since resolution fails
	ctx := context.Background()
	_, err = pub.PublishOfficialPost(ctx, domain.PublishOfficialPostParams{GuildID: "guild-1"})
	if err == nil {
		t.Fatal("PublishOfficialPost should fail")
	}

	err = pub.DeleteOfficialPost(ctx, domain.DeleteOfficialPostParams{GuildID: "guild-1"})
	if err == nil {
		t.Fatal("DeleteOfficialPost should fail")
	}
}
