package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type sharedFeatureContract struct {
	Areas    []sharedFeatureArea    `json:"areas"`
	Features []sharedFeatureBinding `json:"features"`
}

type sharedFeatureArea struct {
	ID string `json:"id"`
}

type sharedFeatureBinding struct {
	ID   string   `json:"id"`
	Area string   `json:"area"`
	Tags []string `json:"tags"`
}

func TestFeatureDefinitionContractMatchesSharedUIMetadata(t *testing.T) {
	t.Parallel()

	contract := loadSharedFeatureContract(t)
	if len(contract.Features) != len(featureDefinitions) {
		t.Fatalf(
			"unexpected shared feature contract size: got=%d want=%d",
			len(contract.Features),
			len(featureDefinitions),
		)
	}

	knownAreas := make(map[string]struct{}, len(contract.Areas))
	for _, area := range contract.Areas {
		areaID := strings.TrimSpace(area.ID)
		if areaID == "" {
			t.Fatalf("shared feature contract contains empty area id: %+v", area)
		}
		knownAreas[areaID] = struct{}{}
	}

	sharedByID := make(map[string]sharedFeatureBinding, len(contract.Features))
	for _, feature := range contract.Features {
		featureID := strings.TrimSpace(feature.ID)
		if featureID == "" {
			t.Fatalf("shared feature contract contains empty feature id: %+v", feature)
		}
		if _, ok := knownAreas[strings.TrimSpace(feature.Area)]; !ok {
			t.Fatalf("shared feature contract area %q is not declared for feature %q", feature.Area, featureID)
		}
		if _, exists := sharedByID[featureID]; exists {
			t.Fatalf("shared feature contract contains duplicate feature id %q", featureID)
		}
		sharedByID[featureID] = feature
	}

	for _, def := range featureDefinitions {
		shared, ok := sharedByID[def.ID]
		if !ok {
			t.Fatalf("feature definition %q is missing from shared ui contract", def.ID)
		}
		if string(def.Area) != shared.Area {
			t.Fatalf("feature %q area mismatch: backend=%q shared=%q", def.ID, def.Area, shared.Area)
		}
		if !slices.Equal(normalizeFeatureContractTags(def.Tags), normalizeFeatureContractTags(shared.Tags)) {
			t.Fatalf("feature %q tag mismatch: backend=%v shared=%v", def.ID, def.Tags, shared.Tags)
		}
		delete(sharedByID, def.ID)
	}

	if len(sharedByID) > 0 {
		t.Fatalf("shared ui contract has extra feature ids: %+v", mapsKeys(sharedByID))
	}
}

func TestFeatureAPIContractShapesRemainStable(t *testing.T) {
	t.Parallel()

	expectJSONFieldOrder(t, reflect.TypeOf(featureCatalogEntry{}), []string{
		"id",
		"category",
		"label",
		"description",
		"area",
		"tags",
		"supports_guild_override",
		"global_editable_fields",
		"guild_editable_fields",
	})
	expectJSONFieldOrder(t, reflect.TypeOf(featureRecord{}), []string{
		"id",
		"category",
		"label",
		"description",
		"scope",
		"area",
		"tags",
		"supports_guild_override",
		"override_state",
		"effective_enabled",
		"effective_source",
		"readiness",
		"blockers",
		"details",
		"editable_fields",
	})
	expectJSONFieldOrder(t, reflect.TypeOf(featureBlocker{}), []string{
		"code",
		"message",
		"field",
	})
}

func loadSharedFeatureContract(t *testing.T) sharedFeatureContract {
	t.Helper()

	path := filepath.Join("..", "..", "ui", "src", "features", "features", "featureContract.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared feature contract %s: %v", path, err)
	}

	var contract sharedFeatureContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("decode shared feature contract %s: %v", path, err)
	}
	return contract
}

func expectJSONFieldOrder(t *testing.T, typ reflect.Type, expected []string) {
	t.Helper()

	actual := make([]string, 0, typ.NumField())
	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		field := typ.Field(fieldIndex)
		tag := strings.TrimSpace(strings.Split(field.Tag.Get("json"), ",")[0])
		if tag == "" || tag == "-" {
			continue
		}
		actual = append(actual, tag)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected json field order for %s: got=%v want=%v", typ.Name(), actual, expected)
	}
}

func normalizeFeatureContractTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	slices.Sort(normalized)
	return normalized
}

func mapsKeys(values map[string]sharedFeatureBinding) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
