package qotd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

func TestBuildOfficialQuestionEmbedCarriesPromptMetadata(t *testing.T) {
	t.Parallel()

	embed := buildOfficialQuestionEmbed(
		"Final Mix",
		62,
		"What song best represents the current mood you are in?",
		345,
	)

	if embed.Title != "☆ question!! ☆" {
		t.Fatalf("unexpected title: %+v", embed)
	}
	if embed.Color != officialQuestionEmbedColor {
		t.Fatalf("expected qotd embed color %x, got %x", officialQuestionEmbedColor, embed.Color)
	}
	if embed.Footer == nil || embed.Footer.Text != "Question ID 345 from Final Mix -- 62 questions remaining" {
		t.Fatalf("expected qotd footer metadata, got %+v", embed.Footer)
	}
	if embed.Timestamp != "" {
		t.Fatalf("expected publish timestamp to be omitted, got %q", embed.Timestamp)
	}
	if len(embed.Fields) != 0 {
		t.Fatalf("expected prompt metadata fields to be removed, got %+v", embed.Fields)
	}
	if !strings.Contains(embed.Description, "what song best represents") {
		t.Fatalf("expected question text in description, got %q", embed.Description)
	}
}

func TestBuildOfficialPostNameMatchesDailyForumFormat(t *testing.T) {
	t.Parallel()

	got := buildOfficialPostName(
		time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
		1,
		"",
	)

	if got != "Question of the Day" {
		t.Fatalf("unexpected official post name: %q", got)
	}
}

func TestTruncateEmbedTextPreservesUTF8Boundaries(t *testing.T) {
	t.Parallel()

	got := truncateEmbedText(strings.Repeat("á", 5), 4)
	if got != "á..." {
		t.Fatalf("unexpected truncated embed text: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8 after truncation, got %q", got)
	}
}

func TestTruncateThreadNamePreservesUTF8Boundaries(t *testing.T) {
	t.Parallel()

	got := truncateThreadName(strings.Repeat("😀", 100) + "!")
	want := strings.Repeat("😀", 97) + "..."
	if got != want {
		t.Fatalf("unexpected truncated thread name: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8 after truncation, got %q", got)
	}
}

func TestBuildOfficialPostStarterMessageOmitsAnswerButton(t *testing.T) {
	t.Parallel()

	embed := buildOfficialQuestionEmbed(
		"Final Mix",
		62,
		"What song best represents the current mood you are in?",
		345,
	)
	message := buildOfficialPostStarterMessage(embed)

	if message == nil || len(message.Embeds) != 1 {
		t.Fatalf("expected one embed starter message, got %+v", message)
	}
	if len(message.Components) != 0 {
		t.Fatalf("expected no message components on official post starter message, got %+v", message.Components)
	}
}

func TestBuildThreadStateChannelEditOmitsFlagsWhenPinIsFalse(t *testing.T) {
	t.Parallel()

	edit := buildThreadStateChannelEdit(ThreadState{
		Locked:   true,
		Archived: false,
	})

	if edit == nil {
		t.Fatal("expected channel edit")
	}
	if edit.Flags != nil {
		t.Fatalf("expected flags to be omitted, got %+v", *edit.Flags)
	}
	if edit.Locked == nil || !*edit.Locked {
		t.Fatalf("expected locked=true, got %+v", edit.Locked)
	}
	if edit.Archived == nil || *edit.Archived {
		t.Fatalf("expected archived=false, got %+v", edit.Archived)
	}
}

type discordTestRoute struct {
	method string
	path   string
	handle func(t *testing.T, r *http.Request, w http.ResponseWriter)
}

func newDiscordTestSession(t *testing.T, routes []discordTestRoute) *discordgo.Session {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, route := range routes {
			if r.Method == route.method && strings.HasSuffix(r.URL.Path, route.path) {
				route.handle(t, r, w)
				return
			}
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	t.Cleanup(server.Close)

	// Many discordgo.Endpoint* values are top-level vars resolved at package
	// load time from EndpointAPI, so overriding just EndpointAPI does NOT
	// retarget the channel/messages endpoint we exercise here. Patch the
	// downstream endpoints directly and restore them when the test ends.
	oldAPI := discordgo.EndpointAPI
	oldChannels := discordgo.EndpointChannels
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointChannels = server.URL + "/channels/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointChannels = oldChannels
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session
}

// TestSendOfficialStarterMessageWithNonceEnforcesServerSideDedup verifies that
// when a nonce is supplied the publisher posts a body that includes both
// `nonce` and `enforce_nonce: true`, so Discord can return the SAME message
// across crash-retries instead of creating a duplicate QOTD post.
func TestSendOfficialStarterMessageWithNonceEnforcesServerSideDedup(t *testing.T) {
	var bodies [][]byte
	var mu sync.Mutex

	session := newDiscordTestSession(t, []discordTestRoute{{
		method: http.MethodPost,
		path:   "/channels/channel-1/messages",
		handle: func(_ *testing.T, r *http.Request, w http.ResponseWriter) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			bodies = append(bodies, body)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"message-123","nonce":"abcdef0123456789"}`))
		},
	}})

	embed := buildOfficialQuestionEmbed("Default", 5, "Repeatable nonce", 1)
	message, err := sendOfficialStarterMessage(session, "channel-1", embed, "abcdef0123456789")
	if err != nil {
		t.Fatalf("sendOfficialStarterMessage() failed: %v", err)
	}
	if message == nil || message.ID != "message-123" {
		t.Fatalf("expected the discord message to be decoded, got %+v", message)
	}

	if len(bodies) != 1 {
		t.Fatalf("expected exactly one POST to discord, got %d", len(bodies))
	}
	var payload map[string]any
	if err := json.Unmarshal(bodies[0], &payload); err != nil {
		t.Fatalf("decode forwarded payload: %v", err)
	}
	if got, _ := payload["nonce"].(string); got != "abcdef0123456789" {
		t.Fatalf("expected nonce to be forwarded, got %q in %+v", got, payload)
	}
	if got, _ := payload["enforce_nonce"].(bool); !got {
		t.Fatalf("expected enforce_nonce=true to be forwarded, got %+v", payload)
	}
}

// TestSendOfficialStarterMessageWithoutNonceUsesLegacyPath confirms the
// non-idempotent send path still works for records created before the nonce
// column existed (the column is nullable).
func TestSendOfficialStarterMessageWithoutNonceUsesLegacyPath(t *testing.T) {
	var bodies [][]byte
	var mu sync.Mutex

	session := newDiscordTestSession(t, []discordTestRoute{{
		method: http.MethodPost,
		path:   "/channels/channel-1/messages",
		handle: func(_ *testing.T, r *http.Request, w http.ResponseWriter) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			bodies = append(bodies, body)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"message-legacy"}`))
		},
	}})

	embed := buildOfficialQuestionEmbed("Default", 5, "Legacy", 1)
	message, err := sendOfficialStarterMessage(session, "channel-1", embed, "")
	if err != nil {
		t.Fatalf("sendOfficialStarterMessage() failed: %v", err)
	}
	if message == nil || message.ID != "message-legacy" {
		t.Fatalf("expected legacy message id, got %+v", message)
	}

	var payload map[string]any
	if err := json.Unmarshal(bodies[0], &payload); err != nil {
		t.Fatalf("decode forwarded payload: %v", err)
	}
	if _, present := payload["nonce"]; present {
		t.Fatalf("expected nonce to be omitted on legacy path, got %+v", payload)
	}
	if _, present := payload["enforce_nonce"]; present {
		t.Fatalf("expected enforce_nonce to be omitted on legacy path, got %+v", payload)
	}
}

// TestStartOrAdoptOfficialThreadAdoptsExistingOnAlreadyCreatedError covers the
// crash-recovery path where Discord already created the thread on a prior
// attempt but our DB never saw the thread ID. A subsequent thread create
// returns 160004 (ALREADY_HAS_A_THREAD); we must read the message back and
// adopt the existing thread instead of failing.
func TestStartOrAdoptOfficialThreadAdoptsExistingOnAlreadyCreatedError(t *testing.T) {
	session := newDiscordTestSession(t, []discordTestRoute{
		{
			method: http.MethodPost,
			path:   "/channels/channel-1/messages/message-1/threads",
			handle: func(_ *testing.T, _ *http.Request, w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"code":%d,"message":"Thread is already created for this message"}`, discordgo.ErrCodeThreadAlreadyCreatedForThisMessage)
			},
		},
		{
			method: http.MethodGet,
			path:   "/channels/channel-1/messages/message-1",
			handle: func(_ *testing.T, _ *http.Request, w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"message-1","thread":{"id":"thread-existing"}}`))
			},
		},
	})

	threadID, err := startOrAdoptOfficialThread(session, PublishOfficialPostParams{
		ChannelID:      "channel-1",
		PublishDateUTC: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		ThreadName:     "Question of the Day",
	}, "message-1")
	if err != nil {
		t.Fatalf("startOrAdoptOfficialThread() failed: %v", err)
	}
	if threadID != "thread-existing" {
		t.Fatalf("expected publisher to adopt the existing thread id, got %q", threadID)
	}
}

func TestBuildThreadStateChannelEditIncludesPinnedFlagsWhenRequested(t *testing.T) {
	t.Parallel()

	edit := buildThreadStateChannelEdit(ThreadState{
		Pinned:   true,
		Locked:   true,
		Archived: true,
	})

	if edit == nil || edit.Flags == nil {
		t.Fatalf("expected pinned flags, got %+v", edit)
	}
	if *edit.Flags != discordgo.ChannelFlagPinned {
		t.Fatalf("expected pinned flag, got %+v", *edit.Flags)
	}
}
