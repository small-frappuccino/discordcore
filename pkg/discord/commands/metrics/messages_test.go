package metrics

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestMetricsCatalogCompleteness(t *testing.T) {
	for _, locale := range []discordgo.Locale{discordgo.EnglishUS, discordgo.PortugueseBR} {
		msgs, ok := metricsCatalog[locale]
		if !ok {
			t.Errorf("locale %q missing from metricsCatalog", locale)
			continue
		}
		for k := metricsMsgKey(0); k < numMetricsMsgKeys; k++ {
			if _, ok := msgs[k]; !ok {
				t.Errorf("locale %q missing key %d", locale, k)
			}
		}
		if len(msgs) != int(numMetricsMsgKeys) {
			t.Errorf("locale %q has %d entries, want %d", locale, len(msgs), numMetricsMsgKeys)
		}
	}
}

func TestMetricsCatalogPortugueseDistinct(t *testing.T) {
	en := metricsCatalog[discordgo.EnglishUS]
	pt := metricsCatalog[discordgo.PortugueseBR]
	same := 0
	for k := metricsMsgKey(0); k < numMetricsMsgKeys; k++ {
		if en[k] == pt[k] {
			same++
		}
	}
	if same == int(numMetricsMsgKeys) {
		t.Errorf("Portuguese catalog is identical to English — translations were not applied")
	}
}
