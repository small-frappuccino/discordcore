package session

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

// LegacySession is a boundary-crossing alias to isolate discordgo dependency
// and eradicate redundant allocation layers for external controllers.
type LegacySession = discordgo.Session

// Injectable seams to allow testing without real network calls.
// Context keys for session testing stubs.
type sessionContextKey string

const (
	openSessionKey    sessionContextKey = "openSession"
	closeSessionKey   sessionContextKey = "closeSession"
	addHandlerOnceKey sessionContextKey = "addHandlerOnce"
)

// Injectable seams to allow testing without real network calls.
var (
	newSession          = discordgo.New
	newSessionOverrides sync.Map
	openSession         = func(s *discordgo.Session) error { return s.Open() }
	closeSession        = func(s *discordgo.Session) error { return s.Close() }
	addHandlerOnce      = func(s *discordgo.Session, h interface{}) func() { return s.AddHandlerOnce(h) }
)

// OpenSession formally connects the discordgo.Session to the gateway,
// waiting for the READY payload asynchronously before returning.
func OpenSession(ctx context.Context, s *discordgo.Session) error {
	if s == nil {
		return fmt.Errorf("session is nil")
	}

	openFn := openSession
	if val := ctx.Value(openSessionKey); val != nil {
		openFn = val.(func(*discordgo.Session) error)
	}

	closeFn := closeSession
	if val := ctx.Value(closeSessionKey); val != nil {
		closeFn = val.(func(*discordgo.Session) error)
	}

	addHandlerFn := addHandlerOnce
	if val := ctx.Value(addHandlerOnceKey); val != nil {
		addHandlerFn = val.(func(*discordgo.Session, interface{}) func())
	}

	readyCh := make(chan struct{})
	removeHandler := addHandlerFn(s, func(s *discordgo.Session, r *discordgo.Ready) {
		close(readyCh)
	})

	if err := openFn(s); err != nil {
		removeHandler()
		return fmt.Errorf(ErrSessionConnectionFailed, err)
	}

	select {
	case <-ctx.Done():
		removeHandler()
		closeFn(s)
		return fmt.Errorf("handshake timed out or canceled: %w", ctx.Err())
	case <-readyCh:
		return nil
	}
}

const defaultSessionIntents = discordgo.IntentsGuilds |
	discordgo.IntentsGuildMembers |
	discordgo.IntentsGuildPresences |
	discordgo.IntentsGuildMessages |
	discordgo.IntentAutoModerationConfiguration |
	discordgo.IntentAutoModerationExecution |
	discordgo.IntentMessageContent

// Error messages
const (
	ErrSessionCreationFailed   = "failed to create Discord session: %w"
	ErrSessionConnectionFailed = "failed to connect to Discord: %w"
)

// NewEmptySessionForCompat creates a dummy session specifically to satisfy
// downstream struct constructors that still expect *discordgo.Session without
// initiating any gateway or REST connections.
func NewEmptySessionForCompat(token string) *LegacySession {
	var s *discordgo.Session
	if val, ok := newSessionOverrides.Load(token); ok {
		s, _ = val.(func(string) (*discordgo.Session, error))(token)
	} else {
		s, _ = newSession(token)
	}
	if s != nil {
		s.StateEnabled = false
	}
	return s
}

// NewDiscordSession creates a new Discord session
func NewDiscordSession(token string) (*discordgo.Session, error) {
	return NewDiscordSessionWithIntents(token, defaultSessionIntents)
}

// NewDiscordSessionWithIntents creates a new Discord session with an explicit gateway intents mask.
func NewDiscordSessionWithIntents(token string, intents discordgo.Intent) (*discordgo.Session, error) {
	var s *discordgo.Session

	// Validate token before creating session
	if token == "" {
		log.ErrorLoggerRaw().Error("Discord bot token is empty. Please set the token before starting the bot.")
		return nil, fmt.Errorf("discord bot token is empty")
	}

	// Add detailed logging for session creation
	log.DiscordLogger().Info("Creating Discord session (token redacted)")

	tokenStr := strings.TrimSpace(token)
	tokenStr = strings.Trim(tokenStr, `"'`)
	for strings.HasPrefix(strings.ToLower(tokenStr), "bot ") {
		tokenStr = strings.TrimSpace(tokenStr[4:])
	}

	var err error
	if val, ok := newSessionOverrides.Load("Bot " + tokenStr); ok {
		s, err = val.(func(string) (*discordgo.Session, error))("Bot " + tokenStr)
	} else {
		s, err = newSession("Bot " + tokenStr)
	}
	if err != nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("Failed to create Discord session: %v", err))
		return nil, fmt.Errorf(ErrSessionCreationFailed, err)
	}

	log.DiscordLogger().Info("Discord session created successfully")
	if intents == 0 {
		intents = defaultSessionIntents
	}
	s.Identify.Intents = intents

	return s, nil
}
