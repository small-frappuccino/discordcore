package commands

import (
	"testing"
)

// Invariante de conjunto de chaves conhecidas para validação de Fuzzing.
var validFeatureKeys = map[string]bool{
	"moderation": true,
	"qotd":       true,
	"roles":      true,
	"partners":   true,
	"embeds":     true,
	"tickets":    true,
	"stats":      true,
	"commands":   true, // Fallback
}

func TestResolveFeatureForCommandPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// Happy paths
		{"Moderation prefix", "ban user", "moderation"},
		{"QOTD prefix", "qotd add", "qotd"},
		{"Role management prefix", "role assign", "roles"},
		{"Partner prefix", "partner add", "partners"},
		{"Embed prefix", "embed create", "embeds"},
		{"Ticket prefix", "ticket open", "tickets"},
		{"Stats prefix", "stats show", "stats"},

		// Edge cases & Fallbacks
		{"Exact match without args", "ban", "moderation"},
		{"Unknown path triggers fallback", "leveling stats", "commands"},
		{"Empty string", "", "commands"},
		{"Malformed payload", "     ban", "commands"}, // HasPrefix is strict, shouldn't trim automatically
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveFeatureForCommandPath(tt.path)
			if result != tt.expected {
				t.Errorf("ResolveFeatureForCommandPath(%q) = %q; want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func FuzzResolveFeatureForCommandPath(f *testing.F) {
	// Seed corpus com rotas conhecidas e lixo
	f.Add("ban user")
	f.Add("qotd")
	f.Add("admin override")
	f.Add("")

	f.Fuzz(func(t *testing.T, path string) {
		result := ResolveFeatureForCommandPath(path)

		// Invariante 1: Nunca deve retornar uma string vazia.
		if result == "" {
			t.Errorf("Fuzzing failure: returned empty feature key for input %q", path)
		}

		// Invariante 2: O resultado DEVE pertencer ao domínio de features pré-aprovadas.
		if !validFeatureKeys[result] {
			t.Errorf("Fuzzing failure: returned unregistered feature key %q for input %q", result, path)
		}
	})
}

func BenchmarkResolveFeatureForCommandPath(b *testing.B) {
	// Alternamos entre um hit rápido, um hit profundo no switch, e um fallback
	// para obter uma média realista do branch predictor.
	paths := []string{
		"qotd add",
		"ban user",
		"stats show",
		"unknown_route_that_hits_default",
	}
	pathsLen := len(paths)

	b.ReportAllocs() // Crucial: Blinda a arquitetura contra regressões de alocação de memória.
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// O resultado é ignorado com `_` já que estamos avaliando apenas throughput e alocação.
		_ = ResolveFeatureForCommandPath(paths[i%pathsLen])
	}
}
