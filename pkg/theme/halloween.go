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
		AvatarChange:  0xFFBB33, // Violet
		MemberJoin:    0xFFBB33, // Pumpkin (amber)
		MemberLeave:   0xB91C1C, // Deep blood red
		MessageEdit:   0xFB923C, // Softer orange
		MessageDelete: 0xDC2626, // Strong red
		AutomodAction: 0xFFBB33, // Orange (alerting but less harsh than pure red)
	})
}
