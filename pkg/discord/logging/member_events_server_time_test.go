package logging

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestCalculateServerTime_ReturnsErrorWhenStoreReadFails(t *testing.T) {
	service := &MemberEventService{
		store: storage.NewStore(filepath.Join(t.TempDir(), "not-initialized.db")),
	}

	got, ok, err := service.calculateServerTime(context.Background(), "g1", "u1")
	if err == nil {
		t.Fatalf("expected store read error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false when store read fails")
	}
	if got != 0 {
		t.Fatalf("expected duration 0 when store read fails, got %v", got)
	}
}
