package theme

import (
	"fmt"
	"sync"
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
		t.AutomodAction = 0xF7768E
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
		AutomodAction:    0xF7768E,
		MemberRoleUpdate: 0x7AA2F7,
	}
	th.ensureDefaults()
	return th
}

var (
	mu        sync.RWMutex
	registry  = map[string]*Theme{}
	currentTh = defaultTheme()
)

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

// MustRegister is like Register but panics on error.
func MustRegister(t *Theme) {
	if err := Register(t); err != nil {
		panic(err)
	}
}

// SetCurrent switches the active theme by name.
func SetCurrent(name string) error {
	mu.Lock()
	defer mu.Unlock()
	if name == "" {
		currentTh = defaultTheme()
		return nil
	}
	th, ok := registry[name]
	if !ok {
		return fmt.Errorf("theme: theme %q not found", name)
	}
	currentTh = th.Clone()
	currentTh.ensureDefaults()
	return nil
}

// Current returns a copy of the current theme.
// Modifying the returned value does not affect the global theme.
func Current() *Theme {
	mu.RLock()
	defer mu.RUnlock()
	return currentTh.Clone()
}

// Default returns a copy of the built-in default theme.
func Default() *Theme {
	return defaultTheme()
}

// Helper getters to use directly in code (avoid exposing globals).
// These read from the current theme and simplify adoption throughout the codebase.

func Primary() Color          { return Current().Primary }
func Accent() Color           { return Current().Accent }
func Info() Color             { return Current().Info }
func Success() Color          { return Current().Success }
func Warning() Color          { return Current().Warning }
func Error() Color            { return Current().Error }
func Danger() Color           { return Current().Danger }
func Muted() Color            { return Current().Muted }
func ServiceList() Color      { return Current().ServiceList }
func SystemInfo() Color       { return Current().SystemInfo }
func StatusOK() Color         { return Current().StatusOK }
func StatusDegraded() Color   { return Current().StatusDegraded }
func StatusError() Color      { return Current().StatusError }
func StatusDefault() Color    { return Current().StatusDefault }
func AvatarChange() Color     { return Current().AvatarChange }
func MemberJoin() Color       { return Current().MemberJoin }
func MemberLeave() Color      { return Current().MemberLeave }
func MessageEdit() Color      { return Current().MessageEdit }
func MessageDelete() Color    { return Current().MessageDelete }
func AutomodAction() Color    { return Current().AutomodAction }
func MemberRoleUpdate() Color { return Current().MemberRoleUpdate }
func Loading() Color          { return Current().Loading }
