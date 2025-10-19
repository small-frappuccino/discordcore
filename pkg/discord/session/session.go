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
		log.ErrorLoggerRaw().Error("❌ Discord bot token is empty. Please set the token before starting the bot.")
		return nil, fmt.Errorf("discord bot token is empty")
	}

	// Add detailed logging for session creation
	log.DiscordLogger().Info("🔑 Creating Discord session (token redacted)")

	if err := errutil.HandleDiscordError("create_session", func() error {
		var sessionErr error
		s, sessionErr = discordgo.New("Bot " + token)
		if sessionErr != nil {
			log.ErrorLoggerRaw().Error(fmt.Sprintf("❌ Failed to create Discord session: %v", sessionErr))
		}
		return sessionErr
	}); err != nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("❌ Error during session creation: %v", err))
		return nil, fmt.Errorf(ErrSessionCreationFailed, err)
	}

	log.DiscordLogger().Info("✅ Discord session created successfully")
	s.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildPresences |
		discordgo.IntentsGuildMessages |
		discordgo.IntentAutoModerationConfiguration |
		discordgo.IntentAutoModerationExecution |
		discordgo.IntentMessageContent

	// Add logging for connection
	log.DiscordLogger().Info("🔗 Connecting to Discord...")
	if err := errutil.HandleDiscordError("connect", func() error {
		connectErr := s.Open()
		if connectErr != nil {
			log.ErrorLoggerRaw().Error(fmt.Sprintf("❌ Failed to connect to Discord: %v", connectErr))
		}
		return connectErr
	}); err != nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("❌ Error during connection: %v", err))
		// Clean up session if connection failed
		if s != nil {
			_ = s.Close()
		}
		return nil, fmt.Errorf(ErrSessionConnectionFailed, err)
	}

	log.DiscordLogger().Info("✅ Connected to Discord successfully")
	return s, nil
}
