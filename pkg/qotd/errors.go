package qotd

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrAlreadyPublished defines err already published.
var ErrAlreadyPublished = errors.New("qotd already published")

// ErrNoCurrentPublish defines err no current publish.
var ErrNoCurrentPublish = errors.New("no current qotd publish found to replace")

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

const (
	postgresUniqueViolationCode            = "23505"
	qotdScheduledPublishConstraint         = "idx_qotd_official_posts_scheduled_publish_date"
	qotdLegacyPublishDateConstraint        = "idx_qotd_official_posts_publish_date"
	qotdAnswerMessagesUniqueUserConstraint = "idx_qotd_answer_messages_unique_user"
)

// isQOTDScheduledPublishConflict reports if the error is a Postgres unique constraint
// violation on the scheduled publish date.
func isQOTDScheduledPublishConflict(err error) bool {
	return isQOTDUniqueConstraint(err,
		qotdScheduledPublishConstraint,
		qotdLegacyPublishDateConstraint,
	)
}

// isQOTDAnswerMessageConflict reports if the error is a Postgres unique constraint
// violation on the answer message.
func isQOTDAnswerMessageConflict(err error) bool {
	return isQOTDUniqueConstraint(err, qotdAnswerMessagesUniqueUserConstraint)
}

// isQOTDUniqueConstraint reports if the error is a Postgres unique constraint
// violation for the given constraint names.
func isQOTDUniqueConstraint(err error, constraintNames ...string) bool {
	pgErr := asPostgresError(err)
	if pgErr == nil || pgErr.Code != postgresUniqueViolationCode {
		return false
	}
	for _, constraintName := range constraintNames {
		if pgErr.ConstraintName == constraintName {
			return true
		}
	}
	return false
}

// asPostgresError unmarshals the error into a *pgconn.PgError if possible.
func asPostgresError(err error) *pgconn.PgError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr == nil {
		return nil
	}
	return pgErr
}
