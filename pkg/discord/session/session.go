package session

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/errutil"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// Error messages
const (
	ErrSessionCreationFailed   = "failed to create Discord session: %w"
	ErrSessionConnectionFailed = "failed to connect to Discord: %w"
)

// NewDiscordSession creates a new Discord session
func NewDiscordSession(token string) (*discordgo.Session, error) {
	var s *discordgo.Session

	// Validate token before creating session
	if token == "" {
		log.Error().Errorf("❌ Discord bot token is empty. Please set the token before starting the bot.")
		return nil, fmt.Errorf("discord bot token is empty")
	}

	// Add detailed logging for session creation
	log.Info().Discordf("🔑 Creating Discord session with token: %s", token)

	if err := errutil.HandleDiscordError("create_session", func() error {
		var sessionErr error
		s, sessionErr = discordgo.New("Bot " + token)
		if sessionErr != nil {
			log.Error().Errorf("❌ Failed to create Discord session: %v", sessionErr)
		}
		return sessionErr
	}); err != nil {
		log.Error().Errorf("❌ Error during session creation: %v", err)
		return nil, fmt.Errorf(ErrSessionCreationFailed, err)
	}

	log.Info().Discordf("✅ Discord session created successfully")
	s.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildPresences |
		discordgo.IntentAutoModerationConfiguration |
		discordgo.IntentAutoModerationExecution |
		discordgo.IntentMessageContent

	// Add logging for connection
	log.Info().Discordf("🔗 Connecting to Discord...")
	if err := errutil.HandleDiscordError("connect", func() error {
		connectErr := s.Open()
		if connectErr != nil {
			log.Error().Errorf("❌ Failed to connect to Discord: %v", connectErr)
		}
		return connectErr
	}); err != nil {
		log.Error().Errorf("❌ Error during connection: %v", err)
		return nil, fmt.Errorf(ErrSessionConnectionFailed, err)
	}

	log.Info().Discordf("✅ Connected to Discord successfully")
	return s, nil
}
