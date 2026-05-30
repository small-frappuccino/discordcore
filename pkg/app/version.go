package app

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Version is the current version of the discordcore package.
const Version = files.DiscordCoreVersion

// AppVersion is the version of the application using discordcore.
func AppVersion() string {
	return files.AppVersion
}

// SetAppVersion sets the version of the application using discordcore.
func SetAppVersion(v string) {
	files.SetAppVersion(v)
}
