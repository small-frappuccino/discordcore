package files

// DiscordCoreVersion is the current version of the discordcore package.
// This value is automatically updated by the release CLI tool.
const DiscordCoreVersion = "v0.840.0-rc.2"

// AppVersion is the version of the application using discordcore.
var AppVersion string

// SetAppVersion sets the version of the application using discordcore.
func SetAppVersion(v string) {
	AppVersion = v
}
