package theme

import (
	"strings"
	"testing"
)

func TestTheme_Default(t *testing.T) {
	th := Default()
	if th == nil {
		t.Fatalf("Default() returned nil")
	}
	if th.Name != "default" {
		t.Errorf("expected theme name 'default', got %q", th.Name)
	}
	if th.Primary != 0x5865F2 {
		t.Errorf("expected Primary color 0x5865F2, got %x", th.Primary)
	}
}

func TestTheme_Register(t *testing.T) {
	// Test nil registration
	err := Register(nil)
	if err == nil || !strings.Contains(err.Error(), "cannot register nil theme") {
		t.Errorf("expected error for nil registration, got: %v", err)
	}

	// Test empty name registration
	err = Register(&Theme{})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected error for empty name, got: %v", err)
	}

	// Test successful registration and cloning
	myTheme := &Theme{
		Name:    "my_test_theme",
		Primary: 0x112233,
	}
	err = Register(myTheme)
	if err != nil {
		t.Fatalf("failed to register custom theme: %v", err)
	}

	// Try registering duplicate
	err = Register(myTheme)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected error for duplicate registration, got: %v", err)
	}
}

func TestTheme_SetCurrent(t *testing.T) {
	// Register a new theme to switch to
	custom := &Theme{
		Name:    "switch_theme",
		Primary: 0x990000,
	}
	if err := Register(custom); err != nil {
		t.Fatalf("failed to register theme: %v", err)
	}

	// Verify default is active first
	if Primary() != 0x5865F2 {
		t.Errorf("expected default Primary, got %x", Primary())
	}

	// Switch to custom theme
	err := SetCurrent("switch_theme")
	if err != nil {
		t.Fatalf("failed to set theme: %v", err)
	}

	// Check updated color
	if Primary() != 0x990000 {
		t.Errorf("expected updated Primary 0x990000, got %x", Primary())
	}

	// Test switching to non-existent theme
	err = SetCurrent("does_not_exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error for non-existent theme, got: %v", err)
	}

	// Switch to empty name resets to default
	err = SetCurrent("")
	if err != nil {
		t.Fatalf("expected reset to default to succeed, got %v", err)
	}
	if Primary() != 0x5865F2 {
		t.Errorf("expected default Primary after reset, got %x", Primary())
	}
}

func TestTheme_GettersAndDefaults(t *testing.T) {
	testTheme := &Theme{
		Name:    "getter_test",
		Primary: 0x111111,
	}
	if err := Register(testTheme); err != nil {
		t.Fatalf("failed to register: %v", err)
	}
	if err := SetCurrent("getter_test"); err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	// Reset to default at the end of the test
	t.Cleanup(func() {
		_ = SetCurrent("")
	})

	// Test value propagation and fallback logic in ensureDefaults()
	if Primary() != 0x111111 {
		t.Errorf("Primary mismatch: got %x", Primary())
	}
	if Accent() != 0x111111 {
		t.Errorf("Accent should default to Primary, got %x", Accent())
	}
	if Info() != 0x3B82F6 {
		t.Errorf("Info fallback mismatch, got %x", Info())
	}
	if Success() != 0x57F287 {
		t.Errorf("Success fallback mismatch, got %x", Success())
	}
	if Warning() != 0xF59E0B {
		t.Errorf("Warning fallback mismatch, got %x", Warning())
	}
	if Loading() != 0xFEE75C {
		t.Errorf("Loading fallback mismatch, got %x", Loading())
	}
	if Error() != 0xED4245 {
		t.Errorf("Error fallback mismatch, got %x", Error())
	}
	if Danger() != 0xED4245 {
		t.Errorf("Danger fallback mismatch, got %x", Danger())
	}
	if Muted() != 0x99AAB5 {
		t.Errorf("Muted fallback mismatch, got %x", Muted())
	}
	if ServiceList() != 0x111111 {
		t.Errorf("ServiceList fallback mismatch, got %x", ServiceList())
	}
	if SystemInfo() != 0x111111 {
		t.Errorf("SystemInfo fallback mismatch, got %x", SystemInfo())
	}
	if StatusOK() != 0x57F287 {
		t.Errorf("StatusOK fallback mismatch, got %x", StatusOK())
	}
	if StatusDegraded() != 0xF59E0B {
		t.Errorf("StatusDegraded fallback mismatch, got %x", StatusDegraded())
	}
	if StatusError() != 0xED4245 {
		t.Errorf("StatusError fallback mismatch, got %x", StatusError())
	}
	if StatusDefault() != 0x99AAB5 {
		t.Errorf("StatusDefault fallback mismatch, got %x", StatusDefault())
	}
	if AvatarChange() != 0x73DACA {
		t.Errorf("AvatarChange fallback mismatch, got %x", AvatarChange())
	}
	if MemberJoin() != 0x9ECE6A {
		t.Errorf("MemberJoin fallback mismatch, got %x", MemberJoin())
	}
	if MemberLeave() != 0xF7768E {
		t.Errorf("MemberLeave fallback mismatch, got %x", MemberLeave())
	}
	if MessageEdit() != 0xE0AF68 {
		t.Errorf("MessageEdit fallback mismatch, got %x", MessageEdit())
	}
	if MessageDelete() != 0xF7768E {
		t.Errorf("MessageDelete fallback mismatch, got %x", MessageDelete())
	}
	if AutomodAction() != 0xDFA3B7 {
		t.Errorf("AutomodAction fallback mismatch, got %x", AutomodAction())
	}
	if MemberRoleUpdate() != 0x7AA2F7 {
		t.Errorf("MemberRoleUpdate fallback mismatch, got %x", MemberRoleUpdate())
	}
}

func TestTheme_HalloweenTheme(t *testing.T) {
	t.Cleanup(func() {
		_ = SetCurrent("")
	})

	err := SetCurrent("halloween")
	if err != nil {
		t.Fatalf("failed to set Halloween theme: %v", err)
	}

	// Verify Halloween specific colors
	if MemberJoin() != 0xEB6123 {
		t.Errorf("expected Halloween MemberJoin color 0xEB6123, got %x", MemberJoin())
	}
	if MessageDelete() != 0xF28B82 {
		t.Errorf("expected Halloween MessageDelete color 0xF28B82, got %x", MessageDelete())
	}
	// Verify defaults inherited from Default theme
	if Primary() != 0x5865F2 {
		t.Errorf("expected inherited Primary, got %x", Primary())
	}
}
