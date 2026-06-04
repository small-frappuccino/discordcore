package files

import (
	"encoding/json"
	
	
	"reflect"
	"testing"
)

func TestFeatureRegistryIDsAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, len(featureRegistry))
	for _, spec := range featureRegistry {
		if _, ok := seen[spec.ID]; ok {
			t.Fatalf("feature registry: duplicate id %q", spec.ID)
		}
		seen[spec.ID] = struct{}{}
	}
}

func TestFeatureRegistryDefaultsMatchLegacyResolveFeatures(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{}
	resolved := cfg.ResolveFeatures("")

	for _, spec := range featureRegistry {
		got, ok := resolved.Lookup(spec.ID)
		if !ok {
			t.Fatalf("feature registry: spec %q not found on resolved toggles", spec.ID)
		}
		if got != spec.Default {
			t.Errorf("feature registry: spec %q default mismatch: got=%v want=%v", spec.ID, got, spec.Default)
		}
	}
}

func TestLookupToggleRoundTrip(t *testing.T) {
	t.Parallel()

	for _, spec := range featureRegistry {
		var ft FeatureToggles
		if ft.LookupToggle(spec.ID) != nil {
			t.Fatalf("spec %q: expected nil before set", spec.ID)
		}

		trueVal := true
		ft.SetToggle(spec.ID, &trueVal)
		if ptr := ft.LookupToggle(spec.ID); ptr == nil || *ptr != true {
			t.Fatalf("spec %q: expected true after set, got %v", spec.ID, ptr)
		}

		falseVal := false
		ft.SetToggle(spec.ID, &falseVal)
		if ptr := ft.LookupToggle(spec.ID); ptr == nil || *ptr != false {
			t.Fatalf("spec %q: expected false after set, got %v", spec.ID, ptr)
		}

		ft.SetToggle(spec.ID, nil)
		if ptr := ft.LookupToggle(spec.ID); ptr != nil {
			t.Fatalf("spec %q: expected nil after clear, got %v", spec.ID, *ptr)
		}
	}
}

func TestHasAnyOverrideDetectsEachToggle(t *testing.T) {
	t.Parallel()

	if (FeatureToggles{}).HasAnyOverride() {
		t.Fatalf("expected HasAnyOverride to be false for zero toggles")
	}

	for _, spec := range featureRegistry {
		var ft FeatureToggles
		v := false
		ft.SetToggle(spec.ID, &v)
		if !ft.HasAnyOverride() {
			t.Fatalf("HasAnyOverride did not detect override for %q", spec.ID)
		}
	}
}

func TestFeatureTogglesJSONRoundTrip(t *testing.T) {
	t.Parallel()

	src := FeatureToggles{}
	for _, spec := range featureRegistry {
		v := true
		src.SetToggle(spec.ID, &v)
	}

	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var round FeatureToggles
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(src, round) {
		t.Fatalf("json round-trip mismatch:\n  src=%#v\n  got=%#v", src, round)
	}
}

