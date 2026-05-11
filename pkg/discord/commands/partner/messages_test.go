package partner

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestPtnCatalogCompleteness(t *testing.T) {
	t.Parallel()
	for _, locale := range ptnCatalogLocales {
		msgs := ptnCatalog[locale]
		for k := ptnMsgKey(0); k < numPtnMsgKeys; k++ {
			if _, ok := msgs[k]; !ok {
				t.Errorf("locale %s missing key %d", locale, k)
			}
		}
	}
}

func TestPtnCatalogPortugueseDistinct(t *testing.T) {
	t.Parallel()
	en := ptnCatalog[discordgo.EnglishUS]
	pt := ptnCatalog[discordgo.PortugueseBR]
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
