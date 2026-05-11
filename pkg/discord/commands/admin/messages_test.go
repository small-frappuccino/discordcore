package admin

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestAdminCatalogCompleteness(t *testing.T) {
	t.Parallel()
	for _, locale := range adminCatalogLocales {
		msgs := adminCatalog[locale]
		for k := adminMsgKey(0); k < numAdminMsgKeys; k++ {
			if _, ok := msgs[k]; !ok {
				t.Errorf("locale %s missing key %d", locale, k)
			}
		}
	}
}

func TestAdminCatalogPortugueseDistinct(t *testing.T) {
	t.Parallel()
	en := adminCatalog[discordgo.EnglishUS]
	pt := adminCatalog[discordgo.PortugueseBR]
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
