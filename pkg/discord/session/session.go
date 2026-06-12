package session

import (
	"context"
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

// Injectable seams to allow testing without real network calls.
var (
	newSession     = discordgo.New
	openSession    = func(s *discordgo.Session) error { return s.Open() }
	closeSession   = func(s *discordgo.Session) error { return s.Close() }
	addHandlerOnce = func(s *discordgo.Session, h interface{}) func() { return s.AddHandlerOnce(h) }
)

// OpenSession formally connects the discordgo.Session to the gateway,
// waiting for the READY payload asynchronously before returning.
func OpenSession(ctx context.Context, s *discordgo.Session) error {
	if s == nil {
		return fmt.Errorf("session is nil")
	}

	readyCh := make(chan struct{})
	removeHandler := addHandlerOnce(s, func(s *discordgo.Session, r *discordgo.Ready) {
		close(readyCh)
	})

	if err := openSession(s); err != nil {
		removeHandler()
		return fmt.Errorf(ErrSessionConnectionFailed, err)
	}

	select {
	case <-ctx.Done():
		removeHandler()
		closeSession(s)
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

	s, err := newSession("Bot " + token)
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
