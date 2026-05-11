package qotd

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestQOTDCommandCatalogCompleteness(t *testing.T) {
	t.Parallel()
	for _, locale := range catalogLocales {
		msgs, ok := catalog[locale]
		if !ok {
			t.Errorf("locale %q missing from catalog", locale)
			continue
		}
		for key := msgKey(0); key < numMsgKeys; key++ {
			if _, ok := msgs[key]; !ok {
				t.Errorf("locale %q missing key %d", locale, key)
			}
		}
	}
}

func TestQOTDCommandCatalogPortugueseTranslationsAreDistinct(t *testing.T) {
	t.Parallel()
	keys := []msgKey{msgDeckNotFound, msgQueueHeader, msgSlotStatusDue, msgAddedQuestion}
	for _, key := range keys {
		en := msg(discordgo.EnglishUS, key, 1, "x")
		pt := msg(discordgo.PortugueseBR, key, 1, "x")
		if en == pt {
			t.Errorf("key %d: expected Portuguese translation to differ from English, got %q", key, en)
		}
	}
}
