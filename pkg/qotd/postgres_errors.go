package qotd

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	postgresUniqueViolationCode            = "23505"
	qotdScheduledPublishConstraint         = "idx_qotd_official_posts_scheduled_publish_date"
	qotdLegacyPublishDateConstraint        = "idx_qotd_official_posts_publish_date"
	qotdThreadArchiveConstraint            = "idx_qotd_thread_archives_thread"
	qotdAnswerMessagesUniqueUserConstraint = "idx_qotd_answer_messages_unique_user"
)

func isQOTDScheduledPublishConflict(err error) bool {
	return isQOTDUniqueConstraint(err,
		qotdScheduledPublishConstraint,
		qotdLegacyPublishDateConstraint,
	)
}

func isQOTDThreadArchiveConflict(err error) bool {
	return isQOTDUniqueConstraint(err, qotdThreadArchiveConstraint)
}

func isQOTDAnswerMessageConflict(err error) bool {
	return isQOTDUniqueConstraint(err, qotdAnswerMessagesUniqueUserConstraint)
}

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

func asPostgresError(err error) *pgconn.PgError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr == nil {
		return nil
	}
	return pgErr
}
