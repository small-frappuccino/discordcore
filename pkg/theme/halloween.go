package theme

// halloween.go
//
// Built-in Halloween theme tailored for an "Alice mains (Zenless Zone Zero)"
// vibe. It keeps core sentiment colors (success, warning, loading, error, etc.)
// from the default theme, but provides notification-focused overrides to give
// a seasonal, spooky flair without breaking semantic color expectations.
//
// To use:
//   util.SetTheme("halloween")
// or
//   ALICE_BOT_THEME=halloween
//
// Only notification/feature roles are overridden here; all other roles inherit
// from the default theme via ensureDefaults().

func init() {
	MustRegister(&Theme{
		Name: "halloween",

		// Notification-focused overrides
		// Purple & Pumpkin palette for a seasonal look
		AvatarChange:     0xEB6123, // Pumpkin
		MemberJoin:       0xEB6123, // Pumpkin
		MemberLeave:      0xEB6123, // Pumpkin
		MessageEdit:      0xEB6123, // Pumpkin
		MessageDelete:    0xF28B82, // Pastel red
		AutomodAction:    0xF28B82, // Pastel red
		MemberRoleUpdate: 0xEB6123, // Pumpkin
	})
}
