package qotd

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestEmbedCatalogCompleteness(t *testing.T) {
	t.Parallel()
	for _, locale := range embedCatalogLocales {
		msgs, ok := embedCatalog[locale]
		if !ok {
			t.Errorf("locale %q missing from embedCatalog", locale)
			continue
		}
		for key := embedMsgKey(0); key < numEmbedMsgKeys; key++ {
			if _, ok := msgs[key]; !ok {
				t.Errorf("locale %q missing embed key %d", locale, key)
			}
		}
	}
}

func TestEmbedCatalogPortugueseTranslationsAreDistinct(t *testing.T) {
	t.Parallel()
	keys := []embedMsgKey{embedMsgTitle, embedMsgPublishNext, embedMsgStatusReady}
	for _, key := range keys {
		en := te(discordgo.EnglishUS, key)
		pt := te(discordgo.PortugueseBR, key)
		if en == pt {
			t.Errorf("embed key %d: expected Portuguese translation to differ from English, got %q", key, en)
		}
	}
}
