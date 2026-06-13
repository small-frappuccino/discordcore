package messages

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestMessageCreateWriterEnqueueAfterStopReturnsStopped(t *testing.T) {
	writer := NewMessageCreateWriter(nil, nil, slog.Default())
	writer.flushInterval = time.Hour
	writer.Start()

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop writer: %v", err)
	}

	err := writer.Enqueue(storage.MessageRecord{
		GuildID:   "guild",
		MessageID: "message"}, nil, storage.DailyMessageCountDelta{})
	if !errors.Is(err, errMessageCreateWriterStopped) {
		t.Fatalf("expected stopped error after shutdown, got %v", err)
	}
}
