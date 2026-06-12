package qotd

import "errors"

// Sentinel errors representing Discord-side failures that the QOTD domain
// must handle for its state machine transitions (e.g., abandoning a post
// when permissions are revoked).
// The Publisher adapter is responsible for mapping the underlying Discord
// SDK errors (e.g., arikawa or discordgo) to these sentinels.
var (
	ErrDiscordUnknownChannel                     = errors.New("discord: unknown channel")
	ErrDiscordUnknownGuild                       = errors.New("discord: unknown guild")
	ErrDiscordUnknownMessage                     = errors.New("discord: unknown message")
	ErrDiscordMissingAccess                      = errors.New("discord: missing access")
	ErrDiscordMissingPermissions                 = errors.New("discord: missing permissions")
	ErrDiscordCannotSendMessagesInVoice          = errors.New("discord: cannot send messages in voice channel")
	ErrDiscordCannotSendMessagesToUser           = errors.New("discord: cannot send messages to this user")
	ErrDiscordUnauthorized                       = errors.New("discord: unauthorized")
	ErrDiscordThreadAlreadyCreatedForThisMessage = errors.New("discord: thread already created for this message")
)
