package files

// DiscordCoreVersion is the current version of the discordcore package.
// This value is automatically updated by the release CLI tool.
const DiscordCoreVersion = "v0.839.0-rc.40"

// AppVersion is the version of the application using discordcore.
var AppVersion string

// SetAppVersion sets the version of the application using discordcore.
func SetAppVersion(v string) {
	AppVersion = v
}
