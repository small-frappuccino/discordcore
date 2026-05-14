package files

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFeatureRegistryPathsResolveOnFeatureToggles(t *testing.T) {
	t.Parallel()

	var ft FeatureToggles
	for _, spec := range featureRegistry {
		field := togglePtrValue(&ft, spec.Path)
		if !field.IsValid() {
			t.Fatalf("feature registry: spec %q path %q does not resolve on FeatureToggles", spec.ID, spec.Path)
		}
		if field.Kind() != reflect.Pointer {
			t.Fatalf("feature registry: spec %q path %q resolves to kind %s on FeatureToggles, want pointer", spec.ID, spec.Path, field.Kind())
		}
		if field.Type().Elem().Kind() != reflect.Bool {
			t.Fatalf("feature registry: spec %q path %q resolves to *%s on FeatureToggles, want *bool", spec.ID, spec.Path, field.Type().Elem().Kind())
		}
	}
}

func TestFeatureRegistryPathsResolveOnResolvedFeatureToggles(t *testing.T) {
	t.Parallel()

	var rft ResolvedFeatureToggles
	for _, spec := range featureRegistry {
		field := resolvedToggleValue(&rft, spec.Path)
		if !field.IsValid() {
			t.Fatalf("feature registry: spec %q path %q does not resolve on ResolvedFeatureToggles", spec.ID, spec.Path)
		}
		if field.Kind() != reflect.Bool {
			t.Fatalf("feature registry: spec %q path %q resolves to kind %s on ResolvedFeatureToggles, want bool", spec.ID, spec.Path, field.Kind())
		}
	}
}

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

func TestFeatureRegistryMatchesUIContract(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "ui", "src", "features", "features", "featureContract.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ui feature contract %s: %v", path, err)
	}

	var contract struct {
		Features []struct {
			ID string `json:"id"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("decode ui feature contract: %v", err)
	}

	contractIDs := make(map[string]struct{}, len(contract.Features))
	for _, entry := range contract.Features {
		contractIDs[entry.ID] = struct{}{}
	}

	registryIDs := make(map[string]struct{}, len(featureRegistry))
	for _, spec := range featureRegistry {
		registryIDs[spec.ID] = struct{}{}
	}

	for id := range registryIDs {
		if _, ok := contractIDs[id]; !ok {
			t.Errorf("ui contract missing registry id %q", id)
		}
	}
	for id := range contractIDs {
		if _, ok := registryIDs[id]; !ok {
			t.Errorf("registry missing ui contract id %q", id)
		}
	}
}
