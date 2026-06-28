package storage

import (
	"context"
	"iter"
	"log/slog"
	"strconv"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type mockFeatureRepo struct {
	configs []core.GuildFeatureConfig
}

func (m *mockFeatureRepo) FetchAllActive(ctx context.Context) (iter.Seq2[core.GuildFeatureConfig, error], error) {
	seq := func(yield func(core.GuildFeatureConfig, error) bool) {
		for _, cfg := range m.configs {
			if !yield(cfg, nil) {
				return
			}
		}
	}
	return seq, nil
}

type noOpHandler struct{}

func (noOpHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (noOpHandler) Handle(context.Context, slog.Record) error { return nil }
func (noOpHandler) WithAttrs([]slog.Attr) slog.Handler        { return noOpHandler{} }
func (noOpHandler) WithGroup(string) slog.Handler             { return noOpHandler{} }

func BenchmarkStorage_HydrateRegistry_ZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	slog.SetDefault(slog.New(noOpHandler{}))

	configs := make([]core.GuildFeatureConfig, 1000)
	for i := 0; i < 1000; i++ {
		configs[i] = core.GuildFeatureConfig{
			GuildID:       "guild" + strconv.Itoa(i),
			FeatureName:   "ban",
			ApplicationID: "app1",
			BotToken:      "token1",
		}
	}

	repo := &mockFeatureRepo{configs: configs}
	registry := core.NewInMemoryFeatureRegistry()
	ctx := context.Background()

	// Pre-fill so it updates existing (zero-alloc CoW)
	_ = HydrateRegistry(ctx, repo, registry)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HydrateRegistry(ctx, repo, registry)
	}
}
