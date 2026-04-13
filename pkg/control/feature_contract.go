package control

type featureAreaID string

const (
	featureAreaCommands    featureAreaID = "commands"
	featureAreaModeration  featureAreaID = "moderation"
	featureAreaLogging     featureAreaID = "logging"
	featureAreaRoles       featureAreaID = "roles"
	featureAreaMaintenance featureAreaID = "maintenance"
	featureAreaStats       featureAreaID = "stats"
)

const (
	featureTagCommandsPrimary        = "commands.primary"
	featureTagCommandsAdmin          = "commands.admin"
	featureTagModerationAutomod      = "moderation.automod"
	featureTagModerationMuteRole     = "moderation.mute_role"
	featureTagModerationCommand      = "moderation.command"
	featureTagModerationRoute        = "moderation.route"
	featureTagRolesAutoAssign        = "roles.auto_assignment"
	featureTagRolesAdvanced          = "roles.advanced"
	featureTagRolesPresenceWatchBot  = "roles.presence_watch.bot"
	featureTagRolesPresenceWatchUser = "roles.presence_watch.user"
	featureTagRolesPermissionMirror  = "roles.permission_mirror"
	featureTagStatsPrimary           = "stats.primary"
	featureTagHomeCommands           = "home.commands"
	featureTagHomeAdminCommands      = "home.admin_commands"
	featureTagHomeStats              = "home.stats"
	featureTagHomeAutoRole           = "home.auto_role"
)
