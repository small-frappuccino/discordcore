# Domain Architecture: theme

## Layout Topology
```text
theme/
├── halloween.go
├── theme.go
└── theme_test.go
```

## Source Stream Aggregation

// === FILE: pkg/theme/halloween.go ===
```go
package theme

// halloween.go
//
// Built-in Halloween theme tailored for an "Alice mains (Zenless Zone Zero)"
// vibe. It keeps core sentiment colors (success, warning, loading, error, etc.)
// from the default theme, but provides notification-focused overrides to give
// a seasonal, spooky flair without breaking semantic color expectations.
//
// To use:
//   files.SetTheme("halloween")
// or
//   ALICE_BOT_THEME=halloween
//
// Only notification/feature roles are overridden here; all other roles inherit
// from the default theme via ensureDefaults().

func init() {
	// Register the built-in theme. Errors are ignored because failures here
	// are purely developer errors (e.g. duplicate name) validated at compile/test time.
	Register(&Theme{
		Name: "halloween",

		// Notification-focused overrides
		// Purple & Pumpkin palette for a seasonal look
		AvatarChange:     0xEB6123, // Pumpkin
		MemberJoin:       0xEB6123, // Pumpkin
		MemberLeave:      0xEB6123, // Pumpkin
		MessageEdit:      0xEB6123, // Pumpkin
		MessageDelete:    0xF28B82, // Pastel red
		AutomodAction:    0xDFA3B7, // Soft rose
		MemberRoleUpdate: 0xEB6123, // Pumpkin
	})
}

```

// === FILE: pkg/theme/theme.go ===
```go
package theme

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Color is the int value used by discordgo.MessageEmbed.Color
type Color = int

// Theme holds all color roles used across the project.
// Keep these roles generic enough so they can be reused across features.
// If a feature needs a very specific color (e.g. "AutomodAction"),
// add it here so themes can override it explicitly.
type Theme struct {
	// Human-friendly name for the theme (unique within the registry).
	Name string

	// Core roles
	Primary Color // General primary color (Discord "blurple" by default)
	Accent  Color // Accent color (often the same as primary)
	Info    Color
	Success Color
	Warning Color
	Loading Color // Distinct loading color
	Error   Color
	Danger  Color // When we want a stronger red than Error
	Muted   Color // Neutral / disabled / default

	// Feature-specific roles (used by existing embeds)
	// Admin/service
	ServiceList    Color
	SystemInfo     Color
	StatusOK       Color // service running & healthy
	StatusDegraded Color // service running but unhealthy
	StatusError    Color
	StatusDefault  Color

	// Notifications/logging
	AvatarChange     Color
	MemberJoin       Color
	MemberLeave      Color
	MessageEdit      Color
	MessageDelete    Color
	AutomodAction    Color
	MemberRoleUpdate Color
}

// Clone returns a copy of the Theme.
func (t *Theme) Clone() *Theme {
	cp := *t
	return &cp
}

// ensureDefaults fills zero-valued fields with sensible fallbacks derived from other roles.
// This allows themes to override only a subset of fields.
func (t *Theme) ensureDefaults() {
	// If some fields are unset (zero), inherit from related roles.

	if t.Primary == 0 {
		t.Primary = 0x5865F2 // Default Primary (blurple)
	}
	if t.Accent == 0 {
		t.Accent = t.Primary
	}
	if t.Info == 0 {
		t.Info = 0x3B82F6
	}
	if t.Success == 0 {
		t.Success = 0x57F287
	}
	if t.Warning == 0 {
		t.Warning = 0xF59E0B
	}
	if t.Loading == 0 {
		t.Loading = 0xFEE75C
	}
	if t.Error == 0 {
		t.Error = 0xED4245
	}
	if t.Danger == 0 {
		t.Danger = 0xED4245
	}
	if t.Muted == 0 {
		t.Muted = 0x99AAB5
	}

	// Feature defaults
	if t.ServiceList == 0 {
		t.ServiceList = t.Primary
	}
	if t.SystemInfo == 0 {
		t.SystemInfo = t.Primary
	}
	if t.StatusOK == 0 {
		t.StatusOK = t.Success
	}
	if t.StatusDegraded == 0 {
		t.StatusDegraded = t.Warning
	}
	if t.StatusError == 0 {
		t.StatusError = t.Error
	}
	if t.StatusDefault == 0 {
		t.StatusDefault = t.Muted
	}

	if t.AvatarChange == 0 {
		t.AvatarChange = 0x73DACA
	}
	if t.MemberJoin == 0 {
		t.MemberJoin = 0x9ECE6A
	}
	if t.MemberLeave == 0 {
		t.MemberLeave = 0xF7768E
	}
	if t.MessageEdit == 0 {
		t.MessageEdit = 0xE0AF68
	}
	if t.MessageDelete == 0 {
		t.MessageDelete = 0xF7768E
	}
	if t.AutomodAction == 0 {
		t.AutomodAction = 0xDFA3B7
	}
	if t.MemberRoleUpdate == 0 {
		t.MemberRoleUpdate = 0x7AA2F7
	}
}

// defaultTheme returns the current built-in theme.
func defaultTheme() *Theme {
	th := &Theme{
		Name:    "default",
		Primary: 0x5865F2, // Discord blurple

		// Core roles (explicit to match existing visuals)
		Info:    0x3B82F6,
		Success: 0x57F287,
		Warning: 0xF59E0B,
		Loading: 0xFEE75C,
		Error:   0xED4245,
		Danger:  0xED4245,
		Muted:   0x99AAB5,

		// Feature roles explicitly matching current embeds
		ServiceList:    0x5865F2,
		SystemInfo:     0x5865F2,
		StatusOK:       0x57F287,
		StatusDegraded: 0xF59E0B,
		StatusError:    0xED4245,
		StatusDefault:  0x99AAB5,

		AvatarChange:     0x73DACA,
		MemberJoin:       0x9ECE6A,
		MemberLeave:      0xF7768E,
		MessageEdit:      0xE0AF68,
		MessageDelete:    0xF7768E,
		AutomodAction:    0xDFA3B7,
		MemberRoleUpdate: 0x7AA2F7,
	}
	th.ensureDefaults()
	return th
}

var (
	mu          sync.Mutex
	registry    = map[string]*Theme{}
	activeTheme atomic.Pointer[Theme]
)

func init() {
	activeTheme.Store(defaultTheme())
}

// Register adds a theme to the registry. It returns an error if the name is empty or already registered.
func Register(t *Theme) error {
	if t == nil {
		return fmt.Errorf("theme: cannot register nil theme")
	}
	if t.Name == "" {
		return fmt.Errorf("theme: name is required")
	}
	cp := t.Clone()
	cp.ensureDefaults()

	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[cp.Name]; exists {
		return fmt.Errorf("theme: theme %q already registered", cp.Name)
	}
	registry[cp.Name] = cp
	return nil
}

// SetCurrent switches the active theme by name.
func SetCurrent(name string) error {
	mu.Lock()
	defer mu.Unlock()
	if name == "" {
		activeTheme.Store(defaultTheme())
		return nil
	}
	th, ok := registry[name]
	if !ok {
		return fmt.Errorf("theme: theme %q not found", name)
	}
	cp := th.Clone()
	cp.ensureDefaults()
	activeTheme.Store(cp)
	return nil
}

// Current returns the current active theme.
func Current() *Theme {
	return activeTheme.Load()
}

// Default returns a copy of the built-in default theme.
func Default() *Theme {
	return defaultTheme()
}

// Helper getters to use directly in code (avoid exposing globals).
// These read from the current theme and simplify adoption throughout the codebase.

// Primary primarys.
func Primary() Color { return Current().Primary }

// Accent accents.
func Accent() Color { return Current().Accent }

// Info infos.
func Info() Color { return Current().Info }

// Success success.
func Success() Color { return Current().Success }

// Warning warnings.
func Warning() Color { return Current().Warning }

// Error errors.
func Error() Color { return Current().Error }

// Danger dangers.
func Danger() Color { return Current().Danger }

// Muted muteds.
func Muted() Color { return Current().Muted }

// ServiceList services list.
func ServiceList() Color { return Current().ServiceList }

// SystemInfo systems info.
func SystemInfo() Color { return Current().SystemInfo }

// StatusOK status ok.
func StatusOK() Color { return Current().StatusOK }

// StatusDegraded status degraded.
func StatusDegraded() Color { return Current().StatusDegraded }

// StatusError status error.
func StatusError() Color { return Current().StatusError }

// StatusDefault status default.
func StatusDefault() Color { return Current().StatusDefault }

// AvatarChange avatars change.
func AvatarChange() Color { return Current().AvatarChange }

// MemberJoin members join.
func MemberJoin() Color { return Current().MemberJoin }

// MemberLeave members leave.
func MemberLeave() Color { return Current().MemberLeave }

// MessageEdit messages edit.
func MessageEdit() Color { return Current().MessageEdit }

// MessageDelete messages delete.
func MessageDelete() Color { return Current().MessageDelete }

// AutomodAction automods action.
func AutomodAction() Color { return Current().AutomodAction }

// MemberRoleUpdate members role update.
func MemberRoleUpdate() Color { return Current().MemberRoleUpdate }

// Loading loadings.
func Loading() Color { return Current().Loading }

```

// === FILE: pkg/theme/theme_test.go ===
```go
package theme

import (
	"strings"
	"sync"
	"testing"
)

var testMu sync.Mutex

func setupThemeTest(t *testing.T) {
	testMu.Lock()

	// Backup registry
	mu.Lock()
	origRegistry := make(map[string]*Theme, len(registry))
	for k, v := range registry {
		origRegistry[k] = v
	}
	mu.Unlock()

	// Backup activeTheme
	origActive := activeTheme.Load()

	t.Cleanup(func() {
		// Restore activeTheme
		activeTheme.Store(origActive)

		// Restore registry
		mu.Lock()
		registry = make(map[string]*Theme, len(origRegistry))
		for k, v := range origRegistry {
			registry[k] = v
		}
		mu.Unlock()

		testMu.Unlock()
	})
}

func TestTheme_Default(t *testing.T) {
	t.Parallel()

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
	t.Parallel()
	setupThemeTest(t)

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
	t.Parallel()
	setupThemeTest(t)

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
	t.Parallel()
	setupThemeTest(t)

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
	t.Parallel()
	setupThemeTest(t)

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

```

