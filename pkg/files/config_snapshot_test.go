package files

import (
	"reflect"
	"testing"
)

func TestCloneFeatureTogglesRoundtripAllTrue(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	for _, spec := range featureRegistry {
		v := true
		src.SetToggle(spec.ID, &v)
	}

	got := cloneFeatureToggles(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("clone (all true) mismatch:\n  src=%#v\n  got=%#v", src, got)
	}
}

func TestCloneFeatureTogglesRoundtripAllFalse(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	for _, spec := range featureRegistry {
		v := false
		src.SetToggle(spec.ID, &v)
	}

	got := cloneFeatureToggles(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("clone (all false) mismatch:\n  src=%#v\n  got=%#v", src, got)
	}
}

func TestCloneFeatureTogglesRoundtripAllNil(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	got := cloneFeatureToggles(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("clone (all nil) mismatch:\n  src=%#v\n  got=%#v", src, got)
	}
}

func TestCloneFeatureTogglesIsolatesMutation(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	v := true
	src.SetToggle("moderation.clean", &v)

	clone := cloneFeatureToggles(src)
	falseVal := false
	src.SetToggle("moderation.clean", &falseVal)

	if ptr := clone.LookupToggle("moderation.clean"); ptr == nil || *ptr != true {
		t.Fatalf("expected clone to retain true after src mutation, got %v", ptr)
	}
}
