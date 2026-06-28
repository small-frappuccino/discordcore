package storage

import (
	"context"
	"iter"
	"log/slog"
	"strconv"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

var benchmarkConfigs []core.GuildFeatureConfig

type mockFeatureRepo struct{}

func benchmarkSeq(yield func(core.GuildFeatureConfig, error) bool) {
	for i := 0; i < len(benchmarkConfigs); i++ {
		if !yield(benchmarkConfigs[i], nil) {
			return
		}
	}
}

func (m mockFeatureRepo) FetchAllActive(ctx context.Context) (iter.Seq2[core.GuildFeatureConfig, error], error) {
	return benchmarkSeq, nil
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

	benchmarkConfigs = configs
	repo := mockFeatureRepo{}
	registry := core.NewInMemoryFeatureRegistry()
	ctx := context.Background()

	// Pre-fill so it updates existing (zero-alloc CoW)
	_ = HydrateRegistry(ctx, repo, registry)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var loopErr error
		for cfg, err := range benchmarkSeq {
			if loopErr != nil {
				continue
			}
			loopErr = processFeature(ctx, registry, cfg, err)
		}
		_ = loopErr
	}
}
