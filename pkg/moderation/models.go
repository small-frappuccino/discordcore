package moderation

import "time"

type Warning struct {
	ID          int64
	GuildID     string
	UserID      string
	CaseNumber  int64
	ModeratorID string
	Reason      string
	CreatedAt   time.Time
}
