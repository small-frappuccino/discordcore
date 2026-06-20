package legacycore

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

// ErrAlreadyAcknowledged is returned by handlers that have already sent a response to Discord
// and do not want the router to log a failure.
var ErrAlreadyAcknowledged = errors.New("interaction has already been acknowledged")

// ArikawaCommandRouter manages routing and execution of Arikawa commands.
type ArikawaCommandRouter struct {
	commands map[string]ArikawaCommand
	client   *api.Client
	config   *files.ConfigManager
}

// NewArikawaCommandRouter creates a new Arikawa router.
func NewArikawaCommandRouter(token string, config *files.ConfigManager) *ArikawaCommandRouter {
	router := &ArikawaCommandRouter{
		commands: make(map[string]ArikawaCommand),
		client:   api.NewClient(token),
		config:   config,
	}
	log.ApplicationLogger().Info("Initialized Arikawa command router")
	return router
}

// Register registers an Arikawa command.
func (r *ArikawaCommandRouter) Register(cmd ArikawaCommand) {
	r.commands[cmd.Name()] = cmd
}

// HandleRawEvent intercepts raw Discord events and routes INTERACTION_CREATE natively to Arikawa.
func (r *ArikawaCommandRouter) HandleRawEvent(s *discordgo.Session, e *discordgo.Event) {
	if e.Type != "INTERACTION_CREATE" {
		return
	}

	var interactionEvent discord.InteractionEvent
	if err := json.Unmarshal(e.RawData, &interactionEvent); err != nil {
		log.ErrorLoggerRaw().Error("Failed to unmarshal raw interaction to Arikawa type", slog.Any("error", err))
		return
	}

	data, ok := interactionEvent.Data.(*discord.CommandInteraction)
	if !ok {
		return
	}

	cmd, exists := r.commands[data.Name]
	if !exists {
		return
	}

	log.DiscordLogger().Debug("Received Arikawa interaction event",
		slog.String("interaction_id", interactionEvent.ID.String()),
		slog.String("command_name", data.Name),
	)

	ctx := &ArikawaContext{
		Client:      r.client,
		Interaction: &interactionEvent,
		Config:      r.config,
		Logger:      log.DiscordLogger(),
		GuildID:     interactionEvent.GuildID,
	}

	if interactionEvent.Member != nil {
		ctx.UserID = interactionEvent.Member.User.ID
	} else if interactionEvent.User != nil {
		ctx.UserID = interactionEvent.User.ID
	}

	if r.config != nil {
		ctx.GuildConfig = r.config.GuildConfig(interactionEvent.GuildID.String())
	}

	if err := cmd.Handle(ctx); err != nil && !errors.Is(err, ErrAlreadyAcknowledged) {
		log.ErrorLoggerRaw().Error("Arikawa command handler failed",
			slog.String("cmd", cmd.Name()),
			slog.String("request_id", interactionEvent.ID.String()),
			slog.Int("http_status", 500),
			slog.Any("error", err),
		)
	}
}

// GetAllCommands returns all registered Arikawa commands.
func (r *ArikawaCommandRouter) GetAllCommands() map[string]ArikawaCommand {
	return r.commands
}

// ConvertArikawaOptions converts Arikawa command options to Discordgo options via JSON bridge.
func ConvertArikawaOptions(opts []discord.CommandOption) []*discordgo.ApplicationCommandOption {
	if len(opts) == 0 {
		return nil
	}
	b, err := json.Marshal(opts)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to marshal arikawa options", slog.Any("error", err))
		return nil
	}

	var dgoOpts []*discordgo.ApplicationCommandOption
	if err := json.Unmarshal(b, &dgoOpts); err != nil {
		log.ErrorLoggerRaw().Error("Failed to unmarshal to discordgo options", slog.Any("error", err))
		return nil
	}
	return dgoOpts
}

// NewArikawaMissingConfigErrorData constructs an actionable error response data for Arikawa
func NewArikawaMissingConfigErrorData(guildID, featureName, dashboardPath string) api.InteractionResponseData {
	url := "https://discordcore.app/manage/" + guildID + dashboardPath
	return api.InteractionResponseData{
		Content: option.NewNullableString("The **" + featureName + "** feature has not been fully configured on the dashboard. Please configure it to use this command."),
		Flags:   discord.EphemeralMessage,
		Components: discord.ComponentsPtr(
			&discord.ButtonComponent{
				Label: "Configure Feature",
				Style: discord.LinkButtonStyle(discord.URL(url)),
			},
		),
	}
}
