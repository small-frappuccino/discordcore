package qotd

import "time"

// PublishOfficialPostParams is the full set of inputs needed to publish (or
// idempotently re-publish) an official QOTD post to Discord.
type PublishOfficialPostParams struct {
	GuildID                    string
	OfficialPostID             int64
	DisplayID                  int64
	DeckName                   string
	AvailableQuestions         int
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	OfficialThreadID           string
	OfficialStarterMessageID   string
	OfficialAnswerChannelID    string
	ExistingPublishedAt        time.Time
	QuestionText               string
	PublishDateUTC             time.Time
	ThreadName                 string
	Nonce                      string
}

// PublishedOfficialPost reports the Discord-side identifiers produced by a
// successful publish, which the caller persists back onto the official-post
// record.
type PublishedOfficialPost struct {
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	ThreadID                   string
	StarterMessageID           string
	AnswerChannelID            string
	PublishedAt                time.Time
	PostURL                    string
}

// ThreadState is the desired pin/lock/archive state applied to a QOTD thread.
type ThreadState struct {
	Pinned   bool
	Locked   bool
	Archived bool
}

// DeleteOfficialPostParams carries the parameters to best-effort delete a QOTD official post from Discord.
type DeleteOfficialPostParams struct {
	GuildID                    string
	DiscordThreadID            string
	DiscordStarterMessageID    string
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
}
