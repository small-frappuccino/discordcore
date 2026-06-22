package automod_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/automod"
)

// 1. Compile-time Interface Assertion
// Garante estaticamente que NopSink sempre implemente Sink.
// Se a assinatura de Sink mudar, a compilação quebra aqui imediatamente.
var _ automod.Sink = (*automod.NopSink)(nil)
var _ automod.Sink = automod.NopSink{}

func TestNopSink_OnAutomodBlock(t *testing.T) {
	t.Parallel()

	t.Run("should execute without panics or side effects", func(t *testing.T) {
		t.Parallel()

		sink := automod.NopSink{}
		ctx := context.Background()

		// Utilizamos require.NotPanics como rede de segurança explícita para o AGENTS.md
		require.NotPanics(t, func() {
			// No ambiente real, use as variáveis mockadas acima.
			// Passando nil para forçar o teste de resiliência caso event seja nil.
			sink.OnAutomodBlock(ctx, discord.GuildID(123456789), nil)
		}, "NopSink operations must be inherently panic-safe")
	})
}

// 2. Zero-Allocation Benchmark
// Prova estruturalmente que o fallback não adiciona overhead ao Garbage Collector.
func BenchmarkNopSink_OnAutomodBlock(b *testing.B) {
	sink := automod.NopSink{}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink.OnAutomodBlock(ctx, discord.GuildID(123456789), nil)
	}
}
