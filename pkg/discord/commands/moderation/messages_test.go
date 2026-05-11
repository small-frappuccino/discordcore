package moderation

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestModCatalogCompleteness(t *testing.T) {
	t.Parallel()
	for _, locale := range modCatalogLocales {
		msgs := modCatalog[locale]
		for k := modMsgKey(0); k < numModMsgKeys; k++ {
			if _, ok := msgs[k]; !ok {
				t.Errorf("locale %s missing key %d", locale, k)
			}
		}
	}
}

func TestModCatalogPortugueseDistinct(t *testing.T) {
	t.Parallel()
	en := modCatalog[discordgo.EnglishUS]
	pt := modCatalog[discordgo.PortugueseBR]
	distinct := 0
	for k := range en {
		if en[k] != pt[k] {
			distinct++
		}
	}
	if distinct < 10 {
		t.Errorf("expected at least 10 distinct pt-BR translations, got %d", distinct)
	}
}
