package config

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCfgCatalogCompleteness(t *testing.T) {
	t.Parallel()
	for _, locale := range cfgCatalogLocales {
		msgs, ok := cfgCatalog[locale]
		if !ok {
			t.Errorf("locale %q missing from cfgCatalog", locale)
			continue
		}
		for key := cfgMsgKey(0); key < numCfgMsgKeys; key++ {
			if _, ok := msgs[key]; !ok {
				t.Errorf("locale %q missing config key %d", locale, key)
			}
		}
	}
}

func TestCfgCatalogPortugueseTranslationsAreDistinct(t *testing.T) {
	t.Parallel()
	keys := []cfgMsgKey{cfgMsgHeader, cfgMsgStateEnabled, cfgMsgErrSaveFailed}
	for _, key := range keys {
		en := tc(discordgo.EnglishUS, key)
		pt := tc(discordgo.PortugueseBR, key)
		if en == pt {
			t.Errorf("cfg key %d: expected Portuguese translation to differ from English, got %q", key, en)
		}
	}
}
