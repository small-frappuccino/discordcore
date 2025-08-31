package store

// ## Constants
// This section consolidates all constants into a single declaration for better organization and readability.
const (
	// ## Path Constants
	ConfigFilePath = "configs/config.json"
	CacheFilePath  = "configs/cache.json"

	// ## Error Constants
	// Avatar cache errors
	ErrReadCacheFile        = "error reading cache file: %w"
	ErrUnmarshalCache       = "error unmarshalling cache: %w"
	ErrCreateCacheDirectory = "error creating cache directory: %w"
	ErrMarshalCache         = "error marshalling cache: %w"
	ErrSaveCacheFile        = "error saving cache file: %w"
	WarnNoGuildCache        = "ClearForGuild called, but guild has no cache"

	// Configuration and File System errors
	ErrFailedLoadConfig           = "failed to load config: %w"
	ErrCreateConfigDir            = "error creating config directory: %w"
	ErrCreateLogsDir              = "error creating logs directory: %w"
	ErrFailedCheckPerms           = "failed to check permissions: %w"
	ErrCreateConfigFile           = "error creating config file: %w"
	ErrCreateCacheFile            = "error creating cache file: %w"
	ErrFailedResolveConfigPath    = "failed to resolve config path: %w"
	ErrCannotSaveNilConfig        = "cannot save nil config"
	ErrFailedMarshalConfig        = "failed to marshal config: %w"
	ErrProjectRootPathNotFoundMsg = "project root path not found"
	ErrInvalidPath                = "invalid path: %w"
	ErrCreateCacheDir             = "error creating cache directory: %w"

	// Discord API and Guild-related errors
	ErrGuildsNotAccessible  = "%d configured guild(s) could not be accessed"
	ErrGuildInfoFetchMsg    = "error fetching guild info %s: %w"
	ErrNoSuitableChannelMsg = "no suitable channel found in guild %s"
	ErrChannelNotFound      = "channel not found"
	ErrChannelWrongGuild    = "channel does not belong to this guild"
	ErrChannelWrongType     = "channel must be a text channel"
	ErrChannelNoPermissions = "bot lacks permissions to send messages in channel"
	ErrWriteAvatarCache     = "error writing avatar cache file: %w"
	ErrMarshalAvatarCache   = "error marshalling avatar cache: %w"
	ErrRemoveAvatarCache    = "error removing avatar cache file: %w"

	// General errors
	ErrValidationFailed           = "validation failed"
	ErrConfigOperationFailed      = "configuration operation failed"
	ErrDiscordOperationFailed     = "discord operation failed"
	ErrNonRetryable               = "non-retryable error encountered"
	ErrOperationFailed            = "operation failed"
	ErrGlobalLoggerNotInitialized = "global logger not initialized for error handler"
	ErrOnAttempt                  = "error on attempt %d for %s"
	ErrOperationAttemptsFailed    = "operation %s failed after %d attempts. Last error: %w"

	// Error format strings
	ErrFmtNonRetryable                 = "non-retryable error in %s: %w"
	ErrFmtOperationCancelled           = "operation %s cancelled: %w"
	ErrFmtOperationFailedAfterRetries  = "operation %s failed after %d attempts: %w"
	ErrFmtOperationFailed              = "%s failed: %w"
	ErrFmtPanicCriticalOperationFailed = "critical operation %s failed: %v"

	// ## Log Constants
	// Configuration and startup logs
	LogCouldNotFetchGuild     = "Could not fetch guild details: %v"
	LogNoSuitableChannel      = "No suitable channel found in guild %s"
	LogGuildAdded             = "Guild added"
	LogGuildAlreadyConfigured = "Guild already configured, skipping"
	LogMonitorGuild           = "Will monitor this guild"
	LogConfigFileNotFound     = "Config file not found, creating: %s"
	LogCacheFileNotFound      = "Cache file not found, creating: %s"
	LogNoConfiguredGuilds     = "No configured guilds. Use /setup to configure."
	LogGuildNotAccessible     = "Guild not accessible; skipping"
	LogFoundConfiguredGuilds  = "%d configured guild(s) found"

	// Specific loading and saving logs
	LogLoadConfigFailedJoinPaths   = "Failed to join paths: %s, error: %v"
	LogLoadConfigFileNotFound      = "Config file not found at path: %s, initializing default config"
	LogLoadConfigFailedReadFile    = "Failed to read config file at path: %s, error: %v"
	LogLoadConfigFailedUnmarshal   = "Failed to unmarshal config data from path: %s, error: %v"
	LogLoadConfigNoGuilds          = "Loaded config has no guilds defined, path: %s"
	LogLoadConfigSuccess           = "Successfully loaded config from path: %s"
	LogSaveConfigNilConfig         = "Attempted to save nil config"
	LogSaveConfigFailedResolvePath = "Failed to resolve config path: %s, error: %v"
	LogSaveConfigFailedMarshal     = "Failed to marshal config, error: %v"
	LogSaveConfigFailedWriteFile   = "Failed to write config to path: %s, error: %v"
	LogSaveConfigSuccess           = "Successfully saved config to path: %s"

	// General log messages
	MsgOperationRetrying            = "operation failed, retrying"
	MsgOperationSucceededAfterRetry = "operation succeeded after retry"
	MsgOperationFailedAllRetries    = "operation failed after all retries"
	MsgOperationFailedCleanup       = "operation failed, running cleanup"
	MsgCriticalOperationFailed      = "critical operation failed"
)
