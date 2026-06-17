package control

import (
	"reflect"
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestFeatureRegistryMatchesCatalog(t *testing.T) {
	t.Parallel()

	registryIDs := make(map[string]struct{}, len(files.FeatureToggleIDs()))
	for _, id := range files.FeatureToggleIDs() {
		registryIDs[id] = struct{}{}
	}

	catalogIDs := make(map[string]struct{}, len(featureDefinitions))
	for _, def := range featureDefinitions {
		catalogIDs[def.ID] = struct{}{}
	}

	for id := range registryIDs {
		if _, ok := catalogIDs[id]; !ok {
			t.Errorf("featureDefinitions missing registry id %q", id)
		}
	}
	for id := range catalogIDs {
		if id == "stats_channels" || id == "auto_role_assignment" || id == "user_prune" || id == "backfill.enabled" {
			continue // Configuration-driven features are intentionally absent from the boolean toggle registry
		}
		if _, ok := registryIDs[id]; !ok {
			t.Errorf("feature registry missing catalog id %q", id)
		}
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
		"config_version",
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
