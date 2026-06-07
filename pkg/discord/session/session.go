package session

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// Injectable seams to allow testing without real network calls.
var (
	newSession   = discordgo.New
	openSession  = func(s *discordgo.Session) error { return s.Open() }
	closeSession = func(s *discordgo.Session) error { return s.Close() }
)

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
