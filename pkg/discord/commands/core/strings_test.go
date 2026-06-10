package core

import (
	"reflect"
	"testing"
)

func TestStrings(t *testing.T) {
	if res := ProcessCommaSeparatedList(""); res != nil {
		t.Fatalf("expected nil, got %v", res)
	}
	if res := ProcessCommaSeparatedList("a, b, , c"); !reflect.DeepEqual(res, []string{"a", "b", "c"}) {
		t.Fatalf("expected [a b c], got %v", res)
	}

	if res := SanitizeInput(" a \n\n b "); res != "a \n b" {
		t.Fatalf("SanitizeInput failed: %q", res)
	}

	if res := TruncateString("abc", 5); res != "abc" {
		t.Fatalf("TruncateString short failed: %q", res)
	}
	if res := TruncateString("abc", 2); res != "ab" {
		t.Fatalf("TruncateString very short failed: %q", res)
	}
	if res := TruncateString("abcdef", 5); res != "ab..." {
		t.Fatalf("TruncateString long failed: %q", res)
	}
	if err := ValidateStringLength("a", 2, 5, "field"); err == nil {
		t.Fatal("ValidateStringLength too short expected error")
	}
	if err := ValidateStringLength("abcdef", 1, 5, "field"); err == nil {
		t.Fatal("ValidateStringLength too long expected error")
	}
	if err := ValidateStringLength("abc", 1, 5, "field"); err != nil {
		t.Fatalf("ValidateStringLength valid expected no error: %v", err)
	}
}
