package session

import (
	"fmt"

	"github.com/alice-bnuy/errutil"
	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
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
		logutil.Fatal("❌ Discord bot token is empty. Please set the token before starting the bot.")
		return nil, fmt.Errorf("discord bot token is empty")
	}

	// Add detailed logging for session creation
	logutil.Infof("🔑 Creating Discord session with token: %s", token)

	if err := errutil.HandleDiscordError("create_session", func() error {
		var sessionErr error
		s, sessionErr = discordgo.New("Bot " + token)
		if sessionErr != nil {
			logutil.Errorf("❌ Failed to create Discord session: %v", sessionErr)
		}
		return sessionErr
	}); err != nil {
		logutil.Fatalf("❌ Error during session creation: %v", err)
		return nil, fmt.Errorf(ErrSessionCreationFailed, err)
	}

	logutil.Info("✅ Discord session created successfully")
	s.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildPresences |
		discordgo.IntentAutoModerationConfiguration |
		discordgo.IntentAutoModerationExecution |
		discordgo.IntentMessageContent

	// Add logging for connection
	logutil.Info("🔗 Connecting to Discord...")
	if err := errutil.HandleDiscordError("connect", func() error {
		connectErr := s.Open()
		if connectErr != nil {
			logutil.Errorf("❌ Failed to connect to Discord: %v", connectErr)
		}
		return connectErr
	}); err != nil {
		logutil.Fatalf("❌ Error during connection: %v", err)
		return nil, fmt.Errorf(ErrSessionConnectionFailed, err)
	}

	logutil.Info("✅ Connected to Discord successfully")
	return s, nil
}
