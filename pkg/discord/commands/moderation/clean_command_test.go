package moderation

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestCleanCommandDeletesMatchingMessagesAndLogsAction(t *testing.T) {
	h := newCleanCommandHarness(t, cleanHarnessConfig{
		guildID:      "g-clean",
		channelID:    "c-main",
		logChannelID: "c-clean-log",
		ownerID:      "owner",
		actorID:      "owner",
		botID:        "bot",
		channelPerms: discordgo.PermissionViewChannel |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionSendMessages |
			discordgo.PermissionEmbedLinks |
			discordgo.PermissionManageMessages,
		messages: []*discordgo.Message{
			newCleanTestMessage("m6", "c-main", "g-clean", "u-other", "spam but wrong user", time.Now().Add(-2*time.Minute), false),
			newCleanTestMessage("m5", "c-main", "g-clean", "u-target", "noise", time.Now().Add(-3*time.Minute), false),
			newCleanTestMessage("m4", "c-main", "g-clean", "u-target", "spam wave", time.Now().Add(-4*time.Minute), false),
			newCleanTestMessage("m3", "c-main", "g-clean", "u-target", "spam burst", time.Now().Add(-5*time.Minute), false),
			newCleanTestMessage("m2", "c-main", "g-clean", "u-target", "spam pinned", time.Now().Add(-6*time.Minute), true),
			newCleanTestMessage("m1", "c-main", "g-clean", "u-target", "spam archive", time.Now().Add(-15*24*time.Hour), false),
		},
	})

	h.run(t,
		cleanIntOption(cleanCountOptionName, 3),
		cleanUserOption(cleanUserOptionName, "u-target"),
		cleanStringOption(cleanContainsOptionName, "spam"),
	)

	if !h.lastAckWasDeferredEphemeral(t) {
		t.Fatal("expected clean command to defer ephemerally")
	}
	if got := h.lastEditedContent(t); !strings.Contains(got, "Cleaned 3 message(s) in this channel.") {
		t.Fatalf("unexpected clean response: %q", got)
	}
	if got := h.lastEditedContent(t); !strings.Contains(got, "from <@u-target>") || !strings.Contains(got, "containing `spam`") {
		t.Fatalf("expected filter summary in response, got %q", got)
	}
	if !slices.Equal(h.bulkDeletedIDs(), []string{"m4", "m3"}) {
		t.Fatalf("unexpected bulk deleted ids: %v", h.bulkDeletedIDs())
	}
	if !slices.Equal(h.singleDeletedIDs(), []string{"m1"}) {
		t.Fatalf("unexpected single deleted ids: %v", h.singleDeletedIDs())
	}
	if posted := h.loggedChannelIDs(); !slices.Equal(posted, []string{"c-clean-log"}) {
		t.Fatalf("expected clean log sent to clean log channel, got %v", posted)
	}
	if got := h.lastPostedEmbedFieldValue("c-clean-log", "Reason"); !strings.Contains(got, "Recent channel cleanup") {
		t.Fatalf("expected clean log reason field to mention cleanup, got %q", got)
	}
	if got := h.lastPostedEmbedDescription("c-clean-log"); !strings.Contains(got, "Moderation action executed") {
		t.Fatalf("expected moderation-style clean embed, got %q", got)
	}
	if !strings.Contains(h.lastPostedEmbedFieldValue("c-clean-log", "Details"), "Single delete: 1") {
		t.Fatalf("expected clean log details to mention single delete, got %q", h.lastPostedEmbedFieldValue("c-clean-log", "Details"))
	}
}

func TestCleanCommandRejectsWhenBotLacksChannelPermissions(t *testing.T) {
	h := newCleanCommandHarness(t, cleanHarnessConfig{
		guildID:   "g-clean-no-perm",
		channelID: "c-main",
		ownerID:   "owner",
		actorID:   "owner",
		botID:     "bot",
		channelPerms: discordgo.PermissionViewChannel |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionSendMessages |
			discordgo.PermissionEmbedLinks,
		messages: []*discordgo.Message{
			newCleanTestMessage("m1", "c-main", "g-clean-no-perm", "u-target", "hello", time.Now().Add(-time.Minute), false),
		},
	})

	h.run(t, cleanIntOption(cleanCountOptionName, 1))

	if got := h.lastEditedContent(t); !strings.Contains(got, "I need View Channel, Read Message History, and Manage Messages in this channel") {
		t.Fatalf("unexpected permission error: %q", got)
	}
	if len(h.bulkDeletedIDs()) != 0 || len(h.singleDeletedIDs()) != 0 {
		t.Fatalf("did not expect any deletions, bulk=%v single=%v", h.bulkDeletedIDs(), h.singleDeletedIDs())
	}
}

func TestCleanCommandDeletesMessagesWithinMessageIDRange(t *testing.T) {
	h := newCleanCommandHarness(t, cleanHarnessConfig{
		guildID:      "g-clean-range",
		channelID:    "c-main",
		logChannelID: "c-clean-log",
		ownerID:      "owner",
		actorID:      "owner",
		botID:        "bot",
		channelPerms: discordgo.PermissionViewChannel |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionSendMessages |
			discordgo.PermissionEmbedLinks |
			discordgo.PermissionManageMessages,
		messages: []*discordgo.Message{
			newCleanTestMessage("100000000000000005", "c-main", "g-clean-range", "u-target", "too new", time.Now().Add(-2*time.Minute), false),
			newCleanTestMessage("100000000000000004", "c-main", "g-clean-range", "u-target", "upper bound", time.Now().Add(-3*time.Minute), false),
			newCleanTestMessage("100000000000000003", "c-main", "g-clean-range", "u-target", "inside newer", time.Now().Add(-4*time.Minute), false),
			newCleanTestMessage("100000000000000002", "c-main", "g-clean-range", "u-target", "inside older", time.Now().Add(-5*time.Minute), false),
			newCleanTestMessage("100000000000000001", "c-main", "g-clean-range", "u-target", "lower bound", time.Now().Add(-6*time.Minute), false),
			newCleanTestMessage("100000000000000000", "c-main", "g-clean-range", "u-target", "too old", time.Now().Add(-7*time.Minute), false),
		},
	})

	h.run(t,
		cleanIntOption(cleanCountOptionName, 10),
		cleanStringOption(cleanFromOptionName, "100000000000000001"),
		cleanStringOption(cleanToOptionName, "100000000000000004"),
	)

	if !h.lastAckWasDeferredEphemeral(t) {
		t.Fatal("expected clean command to defer ephemerally")
	}
	if got := h.lastEditedContent(t); !strings.Contains(got, "Cleaned 2 message(s) in this channel.") {
		t.Fatalf("unexpected clean response: %q", got)
	}
	if got := h.lastEditedContent(t); !strings.Contains(got, "between message IDs `100000000000000001` and `100000000000000004`") {
		t.Fatalf("expected range summary in response, got %q", got)
	}
	if !slices.Equal(h.bulkDeletedIDs(), []string{"100000000000000003", "100000000000000002"}) {
		t.Fatalf("unexpected bulk deleted ids: %v", h.bulkDeletedIDs())
	}
	if len(h.singleDeletedIDs()) != 0 {
		t.Fatalf("did not expect single deletes, got %v", h.singleDeletedIDs())
	}
	if got := h.lastPostedEmbedFieldValue("c-clean-log", "Reason"); !strings.Contains(got, "between message IDs `100000000000000001` and `100000000000000004`") {
		t.Fatalf("expected clean log reason to include range, got %q", got)
	}
}

func TestCleanCommandRejectsInvalidMessageIDRange(t *testing.T) {
	h := newCleanCommandHarness(t, cleanHarnessConfig{
		guildID:      "g-clean-range-invalid",
		channelID:    "c-main",
		logChannelID: "c-clean-log",
		ownerID:      "owner",
		actorID:      "owner",
		botID:        "bot",
		channelPerms: discordgo.PermissionViewChannel |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionSendMessages |
			discordgo.PermissionEmbedLinks |
			discordgo.PermissionManageMessages,
	})

	h.run(t,
		cleanIntOption(cleanCountOptionName, 10),
		cleanStringOption(cleanFromOptionName, "100000000000000004"),
		cleanStringOption(cleanToOptionName, "100000000000000001"),
	)

	if got := h.lastEditedContent(t); !strings.Contains(got, "The `from` message ID must be older than the `to` message ID.") {
		t.Fatalf("unexpected invalid range error: %q", got)
	}
	if len(h.bulkDeletedIDs()) != 0 || len(h.singleDeletedIDs()) != 0 {
		t.Fatalf("did not expect any deletions, bulk=%v single=%v", h.bulkDeletedIDs(), h.singleDeletedIDs())
	}
	if len(h.loggedChannelIDs()) != 0 {
		t.Fatalf("did not expect clean logs for invalid range, got %v", h.loggedChannelIDs())
	}
}

func TestCleanCommandSurfacesClassifiedFetchErrors(t *testing.T) {
	// End-to-end coverage for the wiring between ClassifyFetchError and the
	// command response. Only 403 and 404 are exercised here because
	// discordgo handles 429 and 5xx through its bucket Ratelimiter, which
	// is not gated by MaxRestRetries — those paths would deadlock the
	// harness. The full FailureClass mapping (including 429/5xx) is
	// covered by TestClassifyFetchError in the cleanup package.
	cases := []struct {
		name        string
		fetchErr    cleanFetchErrorSpec
		wantSubstr  string
		mustNotHave string
	}{
		{
			name: "forbidden message-history",
			fetchErr: cleanFetchErrorSpec{
				statusCode: http.StatusForbidden,
				body:       `{"code":50001,"message":"Missing Access"}`,
			},
			wantSubstr:  "I lost permission to read message history in this channel.",
			mustNotHave: "Make sure I can read message history",
		},
		{
			name: "channel deleted mid-flight",
			fetchErr: cleanFetchErrorSpec{
				statusCode: http.StatusNotFound,
				body:       `{"code":10003,"message":"Unknown Channel"}`,
			},
			wantSubstr:  "I couldn't reach this channel anymore",
			mustNotHave: "Make sure I can read message history",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := newCleanCommandHarness(t, cleanHarnessConfig{
				guildID:   "g-fetch-" + tc.name,
				channelID: "c-main",
				ownerID:   "owner",
				actorID:   "owner",
				botID:     "bot",
				channelPerms: discordgo.PermissionViewChannel |
					discordgo.PermissionReadMessageHistory |
					discordgo.PermissionSendMessages |
					discordgo.PermissionEmbedLinks |
					discordgo.PermissionManageMessages,
				messagesFetchError: &tc.fetchErr,
			})

			h.run(t, cleanIntOption(cleanCountOptionName, 5))

			got := h.lastEditedContent(t)
			if !strings.Contains(got, tc.wantSubstr) {
				t.Fatalf("expected response to contain %q, got %q", tc.wantSubstr, got)
			}
			if tc.mustNotHave != "" && strings.Contains(got, tc.mustNotHave) {
				t.Fatalf("response should not include the generic fallback %q (it leaked classification): %q", tc.mustNotHave, got)
			}
			if len(h.bulkDeletedIDs()) != 0 || len(h.singleDeletedIDs()) != 0 {
				t.Fatalf("did not expect any deletions when fetch fails, bulk=%v single=%v", h.bulkDeletedIDs(), h.singleDeletedIDs())
			}
		})
	}
}

// TestCleanCommandRecordsObservabilityMetrics pins the contract between the
// /clean command and the moderation Metrics seam: a happy path increments
// attempts + success + deleted_messages and records a duration; a fetch
// failure increments the classified fetch_* failure cause; a permission
// failure increments the permission_denied cause. Without this test the
// metrics interface and its call sites can drift silently — operators
// would still see counters on /v1/health/moderation, but the buckets
// would no longer match reality.
func TestCleanCommandRecordsObservabilityMetrics(t *testing.T) {
	// Subtests cannot run in parallel because newCleanCommandHarness
	// mutates discordgo package-level endpoint globals (EndpointAPI etc.)
	// to route through httptest. The existing clean tests follow the
	// same constraint — see TestCleanCommandDeletesMatchingMessagesAndLogsAction.

	t.Run("success path records attempt+success+deleted", func(t *testing.T) {
		metrics := &InMemoryMetrics{}
		h := newCleanCommandHarness(t, cleanHarnessConfig{
			guildID:   "g-metrics-ok",
			channelID: "c-main",
			ownerID:   "owner",
			actorID:   "owner",
			botID:     "bot",
			channelPerms: discordgo.PermissionViewChannel |
				discordgo.PermissionReadMessageHistory |
				discordgo.PermissionSendMessages |
				discordgo.PermissionManageMessages,
			messages: []*discordgo.Message{
				newCleanTestMessage("m2", "c-main", "g-metrics-ok", "u-1", "hi", time.Now().Add(-2*time.Minute), false),
				newCleanTestMessage("m1", "c-main", "g-metrics-ok", "u-1", "yo", time.Now().Add(-3*time.Minute), false),
			},
			metrics: metrics,
		})

		h.run(t, cleanIntOption(cleanCountOptionName, 2))

		snap := metrics.Snapshot().Clean
		if snap.AttemptsTotal != 1 {
			t.Fatalf("AttemptsTotal=%d want 1", snap.AttemptsTotal)
		}
		if snap.SuccessTotal != 1 {
			t.Fatalf("SuccessTotal=%d want 1 (%+v)", snap.SuccessTotal, snap)
		}
		if snap.FailureTotal != 0 {
			t.Fatalf("FailureTotal=%d want 0", snap.FailureTotal)
		}
		if snap.DeletedMessagesTotal != 2 {
			t.Fatalf("DeletedMessagesTotal=%d want 2", snap.DeletedMessagesTotal)
		}
		if snap.Duration.Count != 1 {
			t.Fatalf("Duration.Count=%d want 1", snap.Duration.Count)
		}
	})

	t.Run("permission failure records permission_denied cause", func(t *testing.T) {
		metrics := &InMemoryMetrics{}
		h := newCleanCommandHarness(t, cleanHarnessConfig{
			guildID:   "g-metrics-perm",
			channelID: "c-main",
			ownerID:   "owner",
			actorID:   "owner",
			botID:     "bot",
			// Bot is missing PermissionManageMessages so the perm gate trips.
			channelPerms: discordgo.PermissionViewChannel |
				discordgo.PermissionReadMessageHistory,
			messages: []*discordgo.Message{},
			metrics:  metrics,
		})

		h.run(t, cleanIntOption(cleanCountOptionName, 1))

		snap := metrics.Snapshot().Clean
		if snap.AttemptsTotal != 1 || snap.SuccessTotal != 0 || snap.FailureTotal != 1 {
			t.Fatalf("expected one attempt + one failure, got %+v", snap)
		}
		if snap.FailureByCause[CleanFailureCausePermissionDenied] != 1 {
			t.Fatalf("expected permission_denied=1, got %+v", snap.FailureByCause)
		}
	})

	t.Run("audit-log channel failure records RecordCleanAuditLogFailure", func(t *testing.T) {
		metrics := &InMemoryMetrics{}
		h := newCleanCommandHarness(t, cleanHarnessConfig{
			guildID:      "g-metrics-audit",
			channelID:    "c-main",
			logChannelID: "c-clean-log",
			ownerID:      "owner",
			actorID:      "owner",
			botID:        "bot",
			// EmbedLinks is required for ShouldEmitLogEvent to pass the
			// channel-permission gate on LogEventCleanAction; without it
			// the audit-log POST never happens and the failure metric is
			// not exercised.
			channelPerms: discordgo.PermissionViewChannel |
				discordgo.PermissionReadMessageHistory |
				discordgo.PermissionSendMessages |
				discordgo.PermissionEmbedLinks |
				discordgo.PermissionManageMessages,
			messages: []*discordgo.Message{
				newCleanTestMessage("m2", "c-main", "g-metrics-audit", "u-1", "hi", time.Now().Add(-2*time.Minute), false),
				newCleanTestMessage("m1", "c-main", "g-metrics-audit", "u-1", "yo", time.Now().Add(-3*time.Minute), false),
			},
			auditLogPostError: &cleanFetchErrorSpec{
				statusCode: http.StatusForbidden,
				body:       `{"code":50001,"message":"Missing Access"}`,
			},
			metrics: metrics,
		})

		h.run(t, cleanIntOption(cleanCountOptionName, 2))

		snap := metrics.Snapshot().Clean
		if snap.SuccessTotal != 1 {
			t.Fatalf("clean itself should succeed; SuccessTotal=%d (%+v)", snap.SuccessTotal, snap)
		}
		if snap.DeletedMessagesTotal != 2 {
			t.Fatalf("expected 2 deleted messages, got %+v", snap)
		}
		if snap.AuditLogFailureTotal != 1 {
			t.Fatalf("expected AuditLogFailureTotal=1, got %+v", snap)
		}
	})

	t.Run("fetch failure records classified fetch cause", func(t *testing.T) {
		metrics := &InMemoryMetrics{}
		h := newCleanCommandHarness(t, cleanHarnessConfig{
			guildID:   "g-metrics-fetch",
			channelID: "c-main",
			ownerID:   "owner",
			actorID:   "owner",
			botID:     "bot",
			channelPerms: discordgo.PermissionViewChannel |
				discordgo.PermissionReadMessageHistory |
				discordgo.PermissionSendMessages |
				discordgo.PermissionManageMessages,
			// 429 and 5xx are intentionally avoided here — discordgo's bucket
			// rate limiter is not gated by MaxRestRetries and would deadlock
			// the harness (same constraint as TestCleanCommandSurfacesClassifiedFetchErrors).
			messagesFetchError: &cleanFetchErrorSpec{
				statusCode: http.StatusForbidden,
				body:       `{"code":50001,"message":"Missing Access"}`,
			},
			metrics: metrics,
		})

		h.run(t, cleanIntOption(cleanCountOptionName, 5))

		snap := metrics.Snapshot().Clean
		if snap.FailureByCause[CleanFailureCauseFetchForbidden] != 1 {
			t.Fatalf("expected fetch_forbidden=1, got %+v", snap.FailureByCause)
		}
	})
}

// TestCleanCommandExecuteCleanUsesInjectedClock pins the wiring between
// executeClean and the cleanCommand.now seam. Without it, a regression that
// re-introduces a direct time.Now() call inside executeClean would silently
// strand the bulk-vs-single routing on the wall clock, making the boundary
// untestable. The check works by setting cleanCommand.now ~20 years ahead
// of the message timestamps: with c.now() the messages look ancient and
// must route to single-delete; with wall-clock time.Now() they would look
// fresh and route to bulk-delete, flipping the assertions below.
func TestCleanCommandExecuteCleanUsesInjectedClock(t *testing.T) {
	referenceTime := time.Now().UTC()
	injectedNow := referenceTime.Add(20 * 365 * 24 * time.Hour)

	h := newCleanCommandHarness(t, cleanHarnessConfig{
		guildID:   "g-clean-clock",
		channelID: "c-main",
		ownerID:   "owner",
		actorID:   "owner",
		botID:     "bot",
		channelPerms: discordgo.PermissionViewChannel |
			discordgo.PermissionReadMessageHistory |
			discordgo.PermissionSendMessages |
			discordgo.PermissionManageMessages,
		messages: []*discordgo.Message{
			newCleanTestMessage("m-newer", "c-main", "g-clean-clock", "u-target", "spam", referenceTime.Add(-time.Minute), false),
			newCleanTestMessage("m-older", "c-main", "g-clean-clock", "u-target", "spam", referenceTime.Add(-2*time.Minute), false),
		},
	})

	cmd := newCleanCommand(nil)
	cmd.now = func() time.Time { return injectedNow }

	ctx := &core.Context{
		Session: h.session,
		GuildID: h.guildID,
	}
	result, err := cmd.executeClean(ctx, cleanRequest{
		channelID: h.channelID,
		count:     5,
	}, injectedNow)
	if err != nil {
		t.Fatalf("executeClean returned unexpected error: %v", err)
	}

	if result.deletedBulk != 0 {
		t.Fatalf("expected zero bulk deletes when injected clock makes messages ancient, got %+v", result)
	}
	if result.deletedSingle != 2 {
		t.Fatalf("expected both messages to route to single-delete, got %+v", result)
	}
	if got := h.bulkDeletedIDs(); len(got) != 0 {
		t.Fatalf("did not expect any bulk-delete calls, got %v", got)
	}
	if got := h.singleDeletedIDs(); !slices.Equal(got, []string{"m-newer", "m-older"}) {
		t.Fatalf("unexpected single-delete order or contents: %v", got)
	}
}

// TestShouldSingleDeleteCleanMessageBoundary pins the safety margin used to
// route messages near Discord's 14-day bulk-delete cutoff. The margin must
// be wide enough that normal request latency between local classification
// and Discord receiving the bulk-delete cannot push a borderline message
// across the 14-day line. The cleanup package still has a fallback for the
// race, but this margin keeps the race rare in the first place; if it ever
// shrinks back near a minute, this test catches it.
func TestShouldSingleDeleteCleanMessageBoundary(t *testing.T) {
	t.Parallel()

	if cleanBulkDeleteMaxAge > 14*24*time.Hour-30*time.Minute {
		t.Fatalf("cleanBulkDeleteMaxAge must keep at least a 30-minute buffer under the 14-day limit, got %s", cleanBulkDeleteMaxAge)
	}
	if cleanBulkDeleteMaxAge >= 14*24*time.Hour {
		t.Fatalf("cleanBulkDeleteMaxAge must stay below 14 days, got %s", cleanBulkDeleteMaxAge)
	}

	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		timestamp time.Time
		want      bool
	}{
		{
			name:      "fresh message stays on bulk path",
			timestamp: now.Add(-time.Minute),
			want:      false,
		},
		{
			name:      "comfortably under the margin stays on bulk path",
			timestamp: now.Add(-(13 * 24 * time.Hour)),
			want:      false,
		},
		{
			name:      "one minute inside the margin still routes to single delete",
			timestamp: now.Add(-(cleanBulkDeleteMaxAge + time.Minute)),
			want:      true,
		},
		{
			name:      "well past the 14-day line routes to single delete",
			timestamp: now.Add(-(15 * 24 * time.Hour)),
			want:      true,
		},
		{
			name:      "exactly at the margin routes to single delete",
			timestamp: now.Add(-cleanBulkDeleteMaxAge),
			want:      true,
		},
		{
			name:      "nil timestamp stays on bulk path",
			timestamp: time.Time{},
			want:      false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			message := &discordgo.Message{ID: "m", Timestamp: tc.timestamp}
			if got := shouldSingleDeleteCleanMessage(message, now); got != tc.want {
				t.Fatalf("shouldSingleDeleteCleanMessage(age=%s) = %v, want %v", now.Sub(tc.timestamp), got, tc.want)
			}
		})
	}

	if shouldSingleDeleteCleanMessage(nil, now) {
		t.Fatalf("shouldSingleDeleteCleanMessage(nil) must be false")
	}
}

type cleanHarnessConfig struct {
	guildID      string
	channelID    string
	logChannelID string
	ownerID      string
	actorID      string
	botID        string
	channelPerms int64
	messages     []*discordgo.Message
	// messagesFetchError, when non-nil, makes the mock Discord server
	// reject GET /channels/{id}/messages with the supplied status and
	// JSON body. Used to drive ClassifyFetchError code paths from the
	// command end-to-end.
	messagesFetchError *cleanFetchErrorSpec
	// metrics, when non-nil, is wired into RegisterModerationCommandsWithMetrics
	// so tests can assert that /clean records attempts/outcomes through the
	// Metrics interface end-to-end.
	metrics Metrics
	// auditLogPostError, when set, makes the mock Discord server reject
	// POST /channels/{logChannelID}/messages with the supplied status and
	// JSON body. Lets tests drive the audit-log POST failure path through
	// the command end-to-end so RecordCleanAuditLogFailure is observable
	// without rebuilding the harness state machine in each test.
	auditLogPostError *cleanFetchErrorSpec
}

type cleanFetchErrorSpec struct {
	statusCode int
	body       string
}

type cleanRecordedPost struct {
	channelID string
	content   string
	embeds    []*discordgo.MessageEmbed
}

type cleanHarness struct {
	t         *testing.T
	session   *discordgo.Session
	router    *core.CommandRouter
	config    *files.ConfigManager
	guildID   string
	channelID string
	actorID   string
	botID     string

	mu             sync.Mutex
	callbackResp   []discordgo.InteractionResponse
	editedContent  []string
	bulkDeletes    [][]string
	singleDeletes  []string
	postedMessages []cleanRecordedPost
	requests       []string
	channelData    map[string][]*discordgo.Message
}

func newCleanCommandHarness(t *testing.T, cfg cleanHarnessConfig) *cleanHarness {
	t.Helper()

	h := &cleanHarness{
		t:           t,
		guildID:     cfg.guildID,
		channelID:   cfg.channelID,
		actorID:     cfg.actorID,
		botID:       cfg.botID,
		channelData: map[string][]*discordgo.Message{cfg.channelID: cloneCleanMessages(cfg.messages)},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h.mu.Lock()
		h.requests = append(h.requests, req.Method+" "+req.URL.String())
		h.mu.Unlock()

		switch {
		case strings.Contains(req.URL.Path, "/callback"):
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			h.mu.Lock()
			h.callbackResp = append(h.callbackResp, resp)
			h.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case req.Method == http.MethodPatch && (strings.Contains(req.URL.Path, "/messages/@original") || strings.Contains(req.URL.Path, "/messages/%40original")):
			var edit discordgo.WebhookEdit
			_ = json.NewDecoder(req.Body).Decode(&edit)
			content := ""
			if edit.Content != nil {
				content = *edit.Content
			}
			h.mu.Lock()
			h.editedContent = append(h.editedContent, content)
			h.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"original"}`))
		case req.Method == http.MethodGet && strings.HasSuffix(strings.TrimRight(req.URL.Path, "/"), "/messages"):
			if cfg.messagesFetchError != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(cfg.messagesFetchError.statusCode)
				_, _ = w.Write([]byte(cfg.messagesFetchError.body))
				return
			}
			channelID := cleanChannelIDFromPath(req.URL.Path)
			before := strings.TrimSpace(req.URL.Query().Get("before"))
			limit := cleanParseLimit(req.URL.Query().Get("limit"))
			page := h.channelMessagesPage(channelID, before, limit)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(page)
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/bulk-delete"):
			var payload struct {
				Messages []string `json:"messages"`
			}
			_ = json.NewDecoder(req.Body).Decode(&payload)
			h.recordBulkDelete(payload.Messages)
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodDelete && strings.Contains(req.URL.Path, "/messages/"):
			messageID := cleanMessageIDFromDeletePath(req.URL.Path)
			h.recordSingleDelete(messageID)
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodPost && strings.HasSuffix(strings.TrimRight(req.URL.Path, "/"), "/messages"):
			channelID := cleanChannelIDFromPath(req.URL.Path)
			if cfg.auditLogPostError != nil && cfg.logChannelID != "" && channelID == cfg.logChannelID {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(cfg.auditLogPostError.statusCode)
				_, _ = w.Write([]byte(cfg.auditLogPostError.body))
				return
			}
			body, _ := io.ReadAll(req.Body)
			var payload struct {
				Content string                    `json:"content"`
				Embeds  []*discordgo.MessageEmbed `json:"embeds"`
			}
			_ = json.Unmarshal(body, &payload)
			h.mu.Lock()
			h.postedMessages = append(h.postedMessages, cleanRecordedPost{channelID: channelID, content: payload.Content, embeds: payload.Embeds})
			h.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"posted"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldChannels := discordgo.EndpointChannels
	oldWebhooks := discordgo.EndpointWebhooks
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	discordgo.EndpointWebhooks = discordgo.EndpointAPI + "webhooks/"
	t.Cleanup(func() { discordgo.EndpointAPI = oldAPI })
	t.Cleanup(func() { discordgo.EndpointChannels = oldChannels })
	t.Cleanup(func() { discordgo.EndpointWebhooks = oldWebhooks })

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New: %v", err)
	}
	// Disable REST retries so 5xx and rate-limit responses surface to the
	// caller immediately. The harness only ever simulates one outcome per
	// endpoint, so the production retry loop just hangs the test.
	session.MaxRestRetries = 0
	session.State.User = &discordgo.User{ID: cfg.botID}
	if err := session.State.GuildAdd(&discordgo.Guild{
		ID:      cfg.guildID,
		OwnerID: cfg.ownerID,
		Roles: []*discordgo.Role{
			{
				ID:          cfg.guildID,
				Permissions: cfg.channelPerms,
			},
			{
				ID:          "admin-role",
				Permissions: discordgo.PermissionManageGuild,
			},
		},
	}); err != nil {
		t.Fatalf("GuildAdd: %v", err)
	}
	if err := session.State.ChannelAdd(&discordgo.Channel{ID: cfg.channelID, GuildID: cfg.guildID, Type: discordgo.ChannelTypeGuildText}); err != nil {
		t.Fatalf("ChannelAdd main: %v", err)
	}
	if cfg.logChannelID != "" {
		if err := session.State.ChannelAdd(&discordgo.Channel{ID: cfg.logChannelID, GuildID: cfg.guildID, Type: discordgo.ChannelTypeGuildText}); err != nil {
			t.Fatalf("ChannelAdd log: %v", err)
		}
	}
	if err := session.State.MemberAdd(&discordgo.Member{GuildID: cfg.guildID, User: &discordgo.User{ID: cfg.botID}, Roles: []string{cfg.guildID}}); err != nil {
		t.Fatalf("MemberAdd bot: %v", err)
	}
	if err := session.State.MemberAdd(&discordgo.Member{GuildID: cfg.guildID, User: &discordgo.User{ID: cfg.actorID}, Roles: []string{cfg.guildID, "admin-role"}}); err != nil {
		t.Fatalf("MemberAdd actor: %v", err)
	}

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	guildCfg := files.GuildConfig{GuildID: cfg.guildID}
	if cfg.logChannelID != "" {
		guildCfg.Channels.CleanAction = cfg.logChannelID
	}
	if err := cm.AddGuildConfig(guildCfg); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	RegisterModerationCommandsWithMetrics(router, cfg.metrics)

	h.session = session
	h.router = router
	h.config = cm
	return h
}

func (h *cleanHarness) run(t *testing.T, options ...*discordgo.ApplicationCommandInteractionDataOption) {
	t.Helper()
	h.router.HandleInteraction(h.session, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:        "interaction-clean",
		AppID:     "app",
		Token:     "token",
		Type:      discordgo.InteractionApplicationCommand,
		GuildID:   h.guildID,
		ChannelID: h.channelID,
		Member:    &discordgo.Member{User: &discordgo.User{ID: h.actorID}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    cleanCommandName,
			Options: options,
		},
	}})
}

func (h *cleanHarness) channelMessagesPage(channelID, before string, limit int) []*discordgo.Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	messages := cloneCleanMessages(h.channelData[channelID])
	if before == "" {
		if limit > len(messages) {
			limit = len(messages)
		}
		return messages[:limit]
	}
	start := len(messages)
	for idx, message := range messages {
		if message != nil && message.ID == before {
			start = idx + 1
			break
		}
	}
	if start >= len(messages) {
		return []*discordgo.Message{}
	}
	end := start + limit
	if end > len(messages) {
		end = len(messages)
	}
	return messages[start:end]
}

func (h *cleanHarness) recordBulkDelete(ids []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.bulkDeletes = append(h.bulkDeletes, append([]string(nil), ids...))
	h.removeMessages(ids)
}

func (h *cleanHarness) recordSingleDelete(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.singleDeletes = append(h.singleDeletes, id)
	h.removeMessages([]string{id})
}

func (h *cleanHarness) removeMessages(ids []string) {
	if len(ids) == 0 {
		return
	}
	remove := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		remove[id] = struct{}{}
	}
	for channelID, messages := range h.channelData {
		filtered := messages[:0]
		for _, message := range messages {
			if message == nil {
				continue
			}
			if _, ok := remove[message.ID]; ok {
				continue
			}
			filtered = append(filtered, message)
		}
		h.channelData[channelID] = append([]*discordgo.Message(nil), filtered...)
	}
}

func (h *cleanHarness) lastAckWasDeferredEphemeral(t *testing.T) bool {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.callbackResp) == 0 {
		t.Fatal("expected at least one callback response")
	}
	last := h.callbackResp[len(h.callbackResp)-1]
	return last.Type == discordgo.InteractionResponseDeferredChannelMessageWithSource && last.Data.Flags&discordgo.MessageFlagsEphemeral != 0
}

func (h *cleanHarness) lastEditedContent(t *testing.T) string {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.editedContent) == 0 {
		t.Fatalf("expected at least one edited interaction response, requests=%v", h.requests)
	}
	return h.editedContent[len(h.editedContent)-1]
}

func (h *cleanHarness) bulkDeletedIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.bulkDeletes) == 0 {
		return nil
	}
	return append([]string(nil), h.bulkDeletes[len(h.bulkDeletes)-1]...)
}

func (h *cleanHarness) singleDeletedIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.singleDeletes...)
}

func (h *cleanHarness) loggedChannelIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids := make([]string, 0, len(h.postedMessages))
	for _, post := range h.postedMessages {
		ids = append(ids, post.channelID)
	}
	return ids
}

func (h *cleanHarness) lastPostedMessageContent(channelID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for idx := len(h.postedMessages) - 1; idx >= 0; idx-- {
		if h.postedMessages[idx].channelID == channelID {
			return h.postedMessages[idx].content
		}
	}
	return ""
}

func (h *cleanHarness) lastPostedEmbedDescription(channelID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for idx := len(h.postedMessages) - 1; idx >= 0; idx-- {
		post := h.postedMessages[idx]
		if post.channelID != channelID || len(post.embeds) == 0 || post.embeds[0] == nil {
			continue
		}
		return post.embeds[0].Description
	}
	return ""
}

func (h *cleanHarness) lastPostedEmbedFieldValue(channelID, fieldName string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for idx := len(h.postedMessages) - 1; idx >= 0; idx-- {
		post := h.postedMessages[idx]
		if post.channelID != channelID || len(post.embeds) == 0 || post.embeds[0] == nil {
			continue
		}
		for _, field := range post.embeds[0].Fields {
			if field != nil && field.Name == fieldName {
				return field.Value
			}
		}
	}
	return ""
}

func cleanIntOption(name string, value int64) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionInteger, Value: float64(value)}
}

func cleanStringOption(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionString, Value: value}
}

func cleanUserOption(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionUser, Value: value}
}

func newCleanTestMessage(id, channelID, guildID, authorID, content string, timestamp time.Time, pinned bool) *discordgo.Message {
	return &discordgo.Message{
		ID:        id,
		ChannelID: channelID,
		GuildID:   guildID,
		Author:    &discordgo.User{ID: authorID, Username: authorID},
		Content:   content,
		Timestamp: timestamp.UTC(),
		Pinned:    pinned,
	}
}

func cloneCleanMessages(messages []*discordgo.Message) []*discordgo.Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]*discordgo.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		copyMessage := *message
		if message.Author != nil {
			copyAuthor := *message.Author
			copyMessage.Author = &copyAuthor
		}
		cloned = append(cloned, &copyMessage)
	}
	return cloned
}

func cleanChannelIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for idx := 0; idx < len(parts)-1; idx++ {
		if parts[idx] == "channels" {
			return parts[idx+1]
		}
	}
	return ""
}

func cleanMessageIDFromDeletePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func cleanParseLimit(value string) int {
	if strings.TrimSpace(value) == "" {
		return cleanFetchPageSize
	}
	if value == "100" {
		return 100
	}
	if value == "1" {
		return 1
	}
	var limit int
	_, _ = fmt.Sscanf(value, "%d", &limit)
	if limit <= 0 {
		return cleanFetchPageSize
	}
	return limit
}
