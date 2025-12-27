package app

import "github.com/small-frappuccino/discordcore/pkg/util"

// Version is the current version of the discordcore package.
const Version = util.DiscordCoreVersion

// AppVersion is the version of the application using discordcore.
func AppVersion() string {
	return util.AppVersion
}

// SetAppVersion sets the version of the application using discordcore.
func SetAppVersion(v string) {
	util.SetAppVersion(v)
}
