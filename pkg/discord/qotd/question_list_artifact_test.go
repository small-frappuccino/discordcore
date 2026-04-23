package qotd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

type fakeQuestionListArtifactTransport struct {
	threadID        string
	ensureErr       error
	sendErr         error
	lockErr         error
	writableErr     error
	messageID       string
	ensureCalls     []string
	stateCalls      []ThreadState
	sendCalls       []string
	lastSentMessage *discordgo.MessageSend
}

func (f *fakeQuestionListArtifactTransport) EnsureThread(_ context.Context, forumChannelID, preferredThreadID string) (string, error) {
	f.ensureCalls = append(f.ensureCalls, forumChannelID+"|"+preferredThreadID)
	if f.ensureErr != nil {
		return "", f.ensureErr
	}
	if f.threadID == "" {
		f.threadID = "questions-list-thread"
	}
	return f.threadID, nil
}

func (f *fakeQuestionListArtifactTransport) SetThreadState(_ context.Context, _ string, state ThreadState) error {
	f.stateCalls = append(f.stateCalls, state)
	if len(f.stateCalls) == 1 && f.writableErr != nil {
		return f.writableErr
	}
	if len(f.stateCalls) > 1 && f.lockErr != nil {
		return f.lockErr
	}
	return nil
}

func (f *fakeQuestionListArtifactTransport) SendEntry(_ context.Context, threadID string, message *discordgo.MessageSend) (*discordgo.Message, error) {
	f.sendCalls = append(f.sendCalls, threadID)
	f.lastSentMessage = message
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &discordgo.Message{ID: f.messageID}, nil
}

func TestQuestionListArtifactPublisherAppendsAndSealsThread(t *testing.T) {
	t.Parallel()

	transport := &fakeQuestionListArtifactTransport{messageID: "list-entry-1"}
	publisher := questionListArtifactPublisher{
		transport: transport,
		now: func() time.Time {
			return time.Date(2026, 4, 14, 18, 0, 0, 0, time.UTC)
		},
	}

	result, err := publisher.Publish(context.Background(), questionListArtifactPublishParams{
		ForumChannelID: "forum-1",
		OfficialPostID: 42,
		QuestionEmbed:  buildOfficialQuestionEmbed("Default", 3, "What is your answer?", 1),
	})
	if err != nil {
		t.Fatalf("Publish() failed: %v", err)
	}
	if result == nil || result.ThreadID != "questions-list-thread" || result.EntryMessageID != "list-entry-1" {
		t.Fatalf("expected publish result to include thread and entry ids, got %+v", result)
	}
	if result.PublishedAt.IsZero() {
		t.Fatalf("expected publish result to stamp published time, got %+v", result)
	}
	if len(transport.stateCalls) != 2 {
		t.Fatalf("expected writable and sealed state transitions, got %+v", transport.stateCalls)
	}
	if transport.stateCalls[0] != questionListThreadWritableState || transport.stateCalls[1] != questionListThreadSealedState {
		t.Fatalf("unexpected thread state sequence: %+v", transport.stateCalls)
	}
	if len(transport.sendCalls) != 1 || transport.sendCalls[0] != "questions-list-thread" {
		t.Fatalf("expected one entry append call, got %+v", transport.sendCalls)
	}
	if transport.lastSentMessage == nil || len(transport.lastSentMessage.Components) == 0 {
		t.Fatalf("expected answer button on list entry message, got %+v", transport.lastSentMessage)
	}
	if transport.lastSentMessage == nil || len(transport.lastSentMessage.Embeds) != 1 {
		t.Fatalf("expected one embed in list entry, got %+v", transport.lastSentMessage)
	}
	buttons := transport.lastSentMessage.Components
	if len(buttons) == 0 {
		t.Fatalf("expected answer button on list entry message, got %+v", transport.lastSentMessage)
	}
	row, ok := buttons[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) != 1 {
		t.Fatalf("expected one answer button row, got %+v", buttons)
	}
	button, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected button component, got %+v", row.Components[0])
	}
	if button.Label != "answer" || button.Style != discordgo.SecondaryButton {
		t.Fatalf("expected gray answer button, got %+v", button)
	}
}

func TestQuestionListArtifactPublisherSealsExistingEntryWithoutAppend(t *testing.T) {
	t.Parallel()

	transport := &fakeQuestionListArtifactTransport{}
	publisher := questionListArtifactPublisher{transport: transport}

	result, err := publisher.Publish(context.Background(), questionListArtifactPublishParams{
		ForumChannelID:    "forum-1",
		PreferredThreadID: "questions-list-thread",
		EntryMessageID:    "existing-entry",
		OfficialPostID:    42,
		QuestionEmbed:     buildOfficialQuestionEmbed("Default", 3, "What is your answer?", 1),
	})
	if err != nil {
		t.Fatalf("Publish() failed: %v", err)
	}
	if result == nil || result.EntryMessageID != "existing-entry" {
		t.Fatalf("expected existing entry id to be preserved, got %+v", result)
	}
	if len(transport.sendCalls) != 0 {
		t.Fatalf("expected no append for existing entry, got %+v", transport.sendCalls)
	}
	if len(transport.stateCalls) != 1 || transport.stateCalls[0] != questionListThreadSealedState {
		t.Fatalf("expected only final sealed state, got %+v", transport.stateCalls)
	}
}

func TestQuestionListArtifactPublisherRelocksAfterAppendFailure(t *testing.T) {
	t.Parallel()

	transport := &fakeQuestionListArtifactTransport{sendErr: errors.New("discord write failed")}
	publisher := questionListArtifactPublisher{transport: transport}

	result, err := publisher.Publish(context.Background(), questionListArtifactPublishParams{
		ForumChannelID: "forum-1",
		OfficialPostID: 42,
		QuestionEmbed:  buildOfficialQuestionEmbed("Default", 3, "What is your answer?", 1),
	})
	if err == nil {
		t.Fatal("expected append failure")
	}
	if result == nil || result.ThreadID == "" {
		t.Fatalf("expected partial result with resolved thread id, got %+v", result)
	}
	if len(transport.stateCalls) != 2 {
		t.Fatalf("expected relock attempt after failure, got %+v", transport.stateCalls)
	}
	if transport.stateCalls[0] != questionListThreadWritableState || transport.stateCalls[1] != questionListThreadSealedState {
		t.Fatalf("unexpected thread state sequence after failure: %+v", transport.stateCalls)
	}
}

func TestQuestionListArtifactPublisherReturnsJoinedAppendAndRelockErrors(t *testing.T) {
	t.Parallel()

	sendErr := errors.New("discord write failed")
	lockErr := errors.New("discord relock failed")
	transport := &fakeQuestionListArtifactTransport{
		sendErr: sendErr,
		lockErr: lockErr,
	}
	publisher := questionListArtifactPublisher{transport: transport}

	_, err := publisher.Publish(context.Background(), questionListArtifactPublishParams{
		ForumChannelID: "forum-1",
		OfficialPostID: 42,
		QuestionEmbed:  buildOfficialQuestionEmbed("Default", 3, "What is your answer?", 1),
	})
	if err == nil {
		t.Fatal("expected combined append and relock failure")
	}
	if !errors.Is(err, sendErr) {
		t.Fatalf("expected combined error to match send error, got %v", err)
	}
	if !errors.Is(err, lockErr) {
		t.Fatalf("expected combined error to match relock error, got %v", err)
	}
}
