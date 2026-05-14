package control

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestHasFeatureOverridesDetectsEachToggle(t *testing.T) {
	t.Parallel()

	if hasFeatureOverrides(files.FeatureToggles{}) {
		t.Fatalf("expected hasFeatureOverrides false for zero toggles")
	}

	for _, id := range files.FeatureToggleIDs() {
		var ft files.FeatureToggles
		v := false
		ft.SetToggle(id, &v)
		if !hasFeatureOverrides(ft) {
			t.Errorf("hasFeatureOverrides did not detect override for %q", id)
		}
	}
}
