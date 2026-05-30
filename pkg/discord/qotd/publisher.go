package qotd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

const (
	// defaultThreadAutoArchiveMinutes mirrors the QOTD answer window (48h).
	// Discord auto-archives the thread at this point — equivalent to a mod
	// hitting "Close" in the UI — so we don't need to call SetThreadState
	// with Archived=true at ArchiveAt anymore. The grooming reconcile still
	// keeps the thread unlocked during the active window; the explicit
	// archive transition only flips Locked=true to prevent reply-driven
	// unarchive after the window closes.
	defaultThreadAutoArchiveMinutes = 2880
	// fallbackThreadAutoArchiveMinutes is the closest officially-documented
	// auto_archive_duration value above 48h. Discord's API historically
	// validated this field against the discrete set {60, 1440, 4320, 10080};
	// 2880 works on most guilds but a strict-mode rejection (HTTP 400) has
	// been observed. When the primary value is refused we retry once with
	// this fallback so the QOTD still publishes — a 24h tail past the
	// answer window is acceptable; failing the publish entirely is not.
	fallbackThreadAutoArchiveMinutes = 4320
	officialQuestionEmbedColor       = 0xF48FB1
)

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
	// Nonce, when set, is forwarded to Discord with enforce_nonce=true so that
	// a retry after a crash that already accepted the message at Discord
	// returns the existing message ID instead of creating a duplicate. Empty
	// string falls back to the legacy non-idempotent send.
	Nonce string
}

type PublishedOfficialPost struct {
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	ThreadID                   string
	StarterMessageID           string
	AnswerChannelID            string
	PublishedAt                time.Time
	PostURL                    string
}

type ThreadState struct {
	Pinned   bool
	Locked   bool
	Archived bool
}

// Publisher wraps Discord publishing and archive/state transitions for QOTD.
type Publisher struct{}

func NewPublisher() *Publisher {
	return &Publisher{}
}

func (p *Publisher) PublishOfficialPost(ctx context.Context, session *discordgo.Session, params PublishOfficialPostParams) (*PublishedOfficialPost, error) {
	if session == nil {
		return nil, fmt.Errorf("publish official qotd post: discord session is required")
	}

	normalized, err := normalizePublishOfficialPostParams(params)
	if err != nil {
		return nil, fmt.Errorf("publish official qotd post: %w", err)
	}

	questionEmbed := buildOfficialQuestionEmbed(normalized.DeckName, normalized.AvailableQuestions, normalized.QuestionText, normalized.DisplayID)
	result := &PublishedOfficialPost{
		QuestionListThreadID:       normalized.QuestionListThreadID,
		QuestionListEntryMessageID: normalized.QuestionListEntryMessageID,
		ThreadID:                   normalized.OfficialThreadID,
		StarterMessageID:           normalized.OfficialStarterMessageID,
		AnswerChannelID:            normalized.OfficialAnswerChannelID,
		PublishedAt:                normalized.ExistingPublishedAt,
	}

	if result.StarterMessageID == "" {
		message, err := sendOfficialStarterMessage(session, normalized.ChannelID, questionEmbed, normalized.Nonce)
		if err != nil {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd starter message: %w", err)
		}
		if message == nil || strings.TrimSpace(message.ID) == "" {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd starter message: missing message id")
		}
		result.StarterMessageID = strings.TrimSpace(message.ID)
		if result.PublishedAt.IsZero() {
			result.PublishedAt = time.Now().UTC()
		}
	}

	if result.ThreadID == "" {
		threadID, err := startOrAdoptOfficialThread(session, normalized, result.StarterMessageID)
		if err != nil {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd daily thread: %w", err)
		}
		result.ThreadID = strings.TrimSpace(threadID)
		if result.ThreadID == "" {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd daily thread: missing thread id")
		}
		if result.PublishedAt.IsZero() {
			result.PublishedAt = time.Now().UTC()
		}
	}

	if result.AnswerChannelID == "" && result.ThreadID != "" {
		result.AnswerChannelID = result.ThreadID
	}

	return result.withPostURL(normalized.GuildID, normalized.ChannelID), nil
}

// sendOfficialStarterMessage is ChannelMessageSendComplex augmented with
// nonce + enforce_nonce server-side dedup. discordgo's MessageSend does not
// expose those fields yet, so when a nonce is supplied we drop down to the
// generic RequestWithBucketID path and post a JSON payload that includes the
// idempotency hint. Discord returns the SAME message ID across retries with
// the same nonce, so a crash between Discord acknowledging the send and our
// DB saving the message ID cannot duplicate the QOTD post on resume.
//
// When nonce is empty the legacy ChannelMessageSendComplex path is used so
// records created before the nonce column existed keep working.
func sendOfficialStarterMessage(session *discordgo.Session, channelID string, embed *discordgo.MessageEmbed, nonce string) (*discordgo.Message, error) {
	send := buildOfficialPostStarterMessage(embed)
	nonce = strings.TrimSpace(nonce)
	if nonce == "" {
		return session.ChannelMessageSendComplex(channelID, send)
	}

	for _, embed := range send.Embeds {
		if embed.Type == "" {
			embed.Type = "rich"
		}
	}

	payload := struct {
		*discordgo.MessageSend
		Nonce        string `json:"nonce,omitempty"`
		EnforceNonce bool   `json:"enforce_nonce,omitempty"`
	}{
		MessageSend:  send,
		Nonce:        nonce,
		EnforceNonce: true,
	}
	endpoint := discordgo.EndpointChannelMessages(channelID)
	body, err := session.RequestWithBucketID(http.MethodPost, endpoint, payload, endpoint)
	if err != nil {
		return nil, err
	}
	var message discordgo.Message
	if err := json.Unmarshal(body, &message); err != nil {
		return nil, fmt.Errorf("decode qotd starter message: %w", err)
	}
	return &message, nil
}

// startOrAdoptOfficialThread creates the daily thread or adopts the existing
// one when Discord reports ALREADY_HAS_A_THREAD on the starter message.
// That second branch is the recovery path after a crash: the message already
// has a thread on the Discord side from a prior attempt, but our DB never
// recorded its ID, so we read the message back to get the thread reference
// instead of creating a duplicate (which Discord would refuse anyway).
func startOrAdoptOfficialThread(session *discordgo.Session, params PublishOfficialPostParams, starterMessageID string) (string, error) {
	threadName := buildOfficialPostName(params.PublishDateUTC, params.DisplayID, params.ThreadName)
	thread, err := startOfficialThreadWithFallback(session, params.ChannelID, starterMessageID, threadName)
	if err == nil {
		if thread == nil {
			return "", nil
		}
		return strings.TrimSpace(thread.ID), nil
	}
	if !isThreadAlreadyCreatedError(err) {
		return "", err
	}
	existing, lookupErr := session.ChannelMessage(params.ChannelID, starterMessageID)
	if lookupErr != nil {
		return "", fmt.Errorf("lookup existing qotd thread after retry: %w", lookupErr)
	}
	if existing == nil || existing.Thread == nil || strings.TrimSpace(existing.Thread.ID) == "" {
		return "", err
	}
	return strings.TrimSpace(existing.Thread.ID), nil
}

// startOfficialThreadWithFallback attempts thread creation with the preferred
// auto-archive duration (matching the QOTD answer window) and retries once
// with the canonical 3-day value if Discord rejects the request as a
// validation error. The fallback only fires on a clean validation rejection
// — ALREADY_HAS_A_THREAD, permission errors, network failures, and other
// non-validation 400s all bubble up unchanged so they keep their existing
// recovery paths (adopt-existing-thread, abandon-on-permission, retry-later).
func startOfficialThreadWithFallback(session *discordgo.Session, channelID, starterMessageID, threadName string) (*discordgo.Channel, error) {
	thread, err := session.MessageThreadStartComplex(
		channelID,
		starterMessageID,
		&discordgo.ThreadStart{
			Name:                threadName,
			AutoArchiveDuration: defaultThreadAutoArchiveMinutes,
		},
	)
	if err == nil || !isAutoArchiveDurationRejection(err) {
		return thread, err
	}
	return session.MessageThreadStartComplex(
		channelID,
		starterMessageID,
		&discordgo.ThreadStart{
			Name:                threadName,
			AutoArchiveDuration: fallbackThreadAutoArchiveMinutes,
		},
	)
}

// isAutoArchiveDurationRejection narrows a Discord error down to "the
// auto_archive_duration value I just sent is not accepted on this guild". We
// require both a 400 status and an indication that the offending field is
// auto_archive_duration: matching on a bare 400 would swallow unrelated
// validation issues (bad thread name, missing permissions surfaced as 400,
// rate-limit edge cases) and silently change the auto-archive contract.
func isAutoArchiveDurationRejection(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil {
		return false
	}
	if restErr.Response == nil || restErr.Response.StatusCode != http.StatusBadRequest {
		return false
	}
	payload := strings.ToLower(string(restErr.ResponseBody))
	if strings.Contains(payload, "auto_archive_duration") {
		return true
	}
	if restErr.Message != nil {
		msg := strings.ToLower(restErr.Message.Message)
		if strings.Contains(msg, "auto_archive_duration") || strings.Contains(msg, "auto archive duration") {
			return true
		}
	}
	return false
}

func isThreadAlreadyCreatedError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil {
		return false
	}
	return restErr.Message != nil && restErr.Message.Code == discordgo.ErrCodeThreadAlreadyCreatedForThisMessage
}

func (post *PublishedOfficialPost) withPostURL(guildID, channelID string) *PublishedOfficialPost {
	if post == nil {
		return nil
	}
	if strings.TrimSpace(post.PostURL) == "" {
		if channelID = strings.TrimSpace(channelID); channelID != "" {
			if starterMessageID := strings.TrimSpace(post.StarterMessageID); starterMessageID != "" {
				post.PostURL = BuildMessageJumpURL(guildID, channelID, starterMessageID)
				return post
			}
		}
		if threadID := strings.TrimSpace(post.ThreadID); threadID != "" {
			post.PostURL = BuildThreadJumpURL(guildID, threadID)
		}
	}
	return post
}

func (p *Publisher) SetThreadState(ctx context.Context, session *discordgo.Session, threadID string, state ThreadState) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("set qotd thread state: %w", err)
		}
	}()
	if session == nil {
		return errors.New("discord session is required")
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return errors.New("thread id is required")
	}

	if _, err := session.ChannelEditComplex(
		threadID,
		buildThreadStateChannelEdit(state),
	); err != nil {
		return err
	}
	return nil
}

func buildThreadStateChannelEdit(state ThreadState) *discordgo.ChannelEdit {
	locked := state.Locked
	archived := state.Archived
	edit := &discordgo.ChannelEdit{
		Locked:   &locked,
		Archived: &archived,
	}
	if state.Pinned {
		flags := discordgo.ChannelFlagPinned
		edit.Flags = &flags
	}
	return edit
}

func BuildThreadJumpURL(guildID, threadID string) string {
	guildID = strings.TrimSpace(guildID)
	threadID = strings.TrimSpace(threadID)
	if guildID == "" || threadID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s", guildID, threadID)
}

func BuildChannelJumpURL(guildID, channelID string) string {
	return BuildThreadJumpURL(guildID, channelID)
}

func BuildMessageJumpURL(guildID, channelID, messageID string) string {
	guildID = strings.TrimSpace(guildID)
	channelID = strings.TrimSpace(channelID)
	messageID = strings.TrimSpace(messageID)
	if guildID == "" || channelID == "" || messageID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}

func buildOfficialQuestionEmbed(deckName string, availableQuestions int, questionText string, displayID int64) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "☆ question!! ☆",
		Description: normalizeOfficialQuestionText(questionText),
		Color:       officialQuestionEmbedColor,
		Footer: &discordgo.MessageEmbedFooter{
			Text: buildOfficialQuestionFooter(deckName, availableQuestions, displayID),
		},
	}
}

func buildOfficialPostName(_ time.Time, _ int64, explicitName string) string {
	explicitName = strings.TrimSpace(explicitName)
	if explicitName != "" {
		return truncateThreadName(explicitName)
	}
	return "Question of the Day"
}

func buildOfficialPostStarterMessage(embed *discordgo.MessageEmbed) *discordgo.MessageSend {
	return &discordgo.MessageSend{
		Embeds:          []*discordgo.MessageEmbed{embed},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func normalizePublishOfficialPostParams(params PublishOfficialPostParams) (PublishOfficialPostParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.DeckName = strings.TrimSpace(params.DeckName)
	params.ChannelID = strings.TrimSpace(params.ChannelID)
	params.QuestionListThreadID = strings.TrimSpace(params.QuestionListThreadID)
	params.QuestionListEntryMessageID = strings.TrimSpace(params.QuestionListEntryMessageID)
	params.OfficialThreadID = strings.TrimSpace(params.OfficialThreadID)
	params.OfficialStarterMessageID = strings.TrimSpace(params.OfficialStarterMessageID)
	params.OfficialAnswerChannelID = strings.TrimSpace(params.OfficialAnswerChannelID)
	params.QuestionText = strings.TrimSpace(params.QuestionText)
	params.ThreadName = strings.TrimSpace(params.ThreadName)
	if params.AvailableQuestions < 0 {
		params.AvailableQuestions = 0
	}
	if !params.ExistingPublishedAt.IsZero() {
		params.ExistingPublishedAt = params.ExistingPublishedAt.UTC()
	}
	params.PublishDateUTC = params.PublishDateUTC.UTC()

	switch {
	case params.GuildID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("guild id is required")
	case params.OfficialPostID <= 0:
		return PublishOfficialPostParams{}, fmt.Errorf("official post id is required")
	case params.QuestionText == "":
		return PublishOfficialPostParams{}, fmt.Errorf("question text is required")
	case params.PublishDateUTC.IsZero():
		return PublishOfficialPostParams{}, fmt.Errorf("publish date is required")
	case params.ChannelID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("channel id is required")
	default:
		return params, nil
	}
}

func quoteEmbedText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "> -"
	}
	lines := strings.Split(text, "\n")
	for idx := range lines {
		line := strings.TrimSpace(lines[idx])
		if line == "" {
			lines[idx] = ">"
			continue
		}
		lines[idx] = "> " + line
	}
	return truncateEmbedText(strings.Join(lines, "\n"), limit)
}

func truncateEmbedText(text string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
}

func buildOfficialQuestionFooter(deckName string, availableQuestions int, displayID int64) string {
	deckName = strings.TrimSpace(deckName)
	if deckName == "" {
		deckName = "Default"
	}
	if availableQuestions < 0 {
		availableQuestions = 0
	}
	if displayID > 0 {
		return fmt.Sprintf("Question ID %d from %s -- %d questions remaining", displayID, deckName, availableQuestions)
	}
	return fmt.Sprintf("%s -- %d questions remaining", deckName, availableQuestions)
}

func normalizeOfficialQuestionText(questionText string) string {
	return strings.ToLower(strings.TrimSpace(questionText))
}

func truncateThreadName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) <= 100 {
		return name
	}
	return strings.TrimSpace(string([]rune(name)[:97])) + "..."
}
