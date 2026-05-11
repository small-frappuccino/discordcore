package runtime

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRuntimeCatalogCompleteness(t *testing.T) {
	for _, locale := range runtimeCatalogLocales {
		msgs, ok := runtimeCatalog[locale]
		if !ok {
			t.Errorf("locale %q missing from runtimeCatalog", locale)
			continue
		}
		for k := runtimeMsgKey(0); k < numRuntimeMsgKeys; k++ {
			if _, ok := msgs[k]; !ok {
				t.Errorf("locale %q missing key %d", locale, k)
			}
		}
		if len(msgs) != int(numRuntimeMsgKeys) {
			t.Errorf("locale %q has %d entries, want %d", locale, len(msgs), numRuntimeMsgKeys)
		}
	}
}

func TestRuntimeCatalogPortugueseDistinct(t *testing.T) {
	en := runtimeCatalog[discordgo.EnglishUS]
	pt := runtimeCatalog[discordgo.PortugueseBR]
	same := 0
	for k := runtimeMsgKey(0); k < numRuntimeMsgKeys; k++ {
		if en[k] == pt[k] {
			same++
		}
	}
	if same == int(numRuntimeMsgKeys) {
		t.Errorf("Portuguese catalog is identical to English — translations were not applied")
	}
}
