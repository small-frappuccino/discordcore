package core

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// ResponseType defines standard response types
type ResponseType int

const (
	ResponseSuccess ResponseType = iota
	ResponseError
	ResponseWarning
	ResponseInfo
	ResponseLoading
)

// ResponseConfig sets options for responses
type ResponseConfig struct {
	Ephemeral   bool
	Title       string
	Color       int
	WithEmbed   bool
	Footer      string
	Timestamp   bool
	Components  []discordgo.MessageComponent
	Attachments []*discordgo.File
}

// ResponseManager manages all interaction responses
type ResponseManager struct {
	session *discordgo.Session
	config  ResponseConfig
}

// NewResponseManager creates a new response manager
func NewResponseManager(session *discordgo.Session) *ResponseManager {
	return &ResponseManager{
		session: session,
		config:  ResponseConfig{},
	}
}

// WithConfig sets configuration for the next response
func (rm *ResponseManager) WithConfig(config ResponseConfig) *ResponseManager {
	return &ResponseManager{
		session: rm.session,
		config:  config,
	}
}

// Success sends a success response
func (rm *ResponseManager) Success(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseSuccess)
}

// Error sends an error response
func (rm *ResponseManager) Error(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseError)
}

// Warning sends a warning response
func (rm *ResponseManager) Warning(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseWarning)
}

// Info sends an informational response
func (rm *ResponseManager) Info(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseInfo)
}

// Loading sends a loading response
func (rm *ResponseManager) Loading(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseLoading)
}

// Ephemeral sends a simple ephemeral response
func (rm *ResponseManager) Ephemeral(i *discordgo.InteractionCreate, message string) error {
	config := rm.config
	config.Ephemeral = true
	return rm.WithConfig(config).Info(i, message)
}

// Custom sends a custom response using a centralized builder
func (rm *ResponseManager) Custom(i *discordgo.InteractionCreate, content string, embeds []*discordgo.MessageEmbed) error {
	data := rm.buildResponseData(content, embeds)
	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}

// buildResponseData builds InteractionResponseData honoring ResponseConfig
func (rm *ResponseManager) buildResponseData(content string, embeds []*discordgo.MessageEmbed) *discordgo.InteractionResponseData {
	var flags discordgo.MessageFlags
	if rm.config.Ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	return &discordgo.InteractionResponseData{
		Content:    content,
		Embeds:     embeds,
		Flags:      flags,
		Components: rm.config.Components,
		Files:      rm.config.Attachments,
	}
}

// buildFlags returns ephemeral flag when requested
func (rm *ResponseManager) buildFlags(ephemeral bool) discordgo.MessageFlags {
	if ephemeral {
		return discordgo.MessageFlagsEphemeral
	}
	return 0
}

// sendResponse sends a response based on the type
func (rm *ResponseManager) sendResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	if rm.config.WithEmbed {
		return rm.sendEmbedResponse(i, message, responseType)
	}
	return rm.sendTextResponse(i, message, responseType)
}

// sendTextResponse sends a simple text response
func (rm *ResponseManager) sendTextResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	content := rm.formatTextMessage(message, responseType)
	return rm.Custom(i, content, nil)
}

// sendEmbedResponse sends a response with an embed
func (rm *ResponseManager) sendEmbedResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	embed := rm.createEmbed(message, responseType)
	return rm.Custom(i, "", []*discordgo.MessageEmbed{embed})
}

// formatTextMessage formats a text message based on the type
func (rm *ResponseManager) formatTextMessage(message string, responseType ResponseType) string {
	switch responseType {
	case ResponseSuccess:
		return "✅ " + message
	case ResponseError:
		return "❌ " + message
	case ResponseWarning:
		return "⚠️ " + message
	case ResponseInfo:
		return "ℹ️ " + message
	case ResponseLoading:
		return "⏳ " + message
	default:
		return message
	}
}

// createEmbed creates an embed based on the response type
func (rm *ResponseManager) createEmbed(message string, responseType ResponseType) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Description: message,
		Color:       rm.getColorForType(responseType),
	}

	if rm.config.Title != "" {
		embed.Title = rm.config.Title
	} else {
		embed.Title = rm.getTitleForType(responseType)
	}

	if rm.config.Footer != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: rm.config.Footer,
		}
	}

	if rm.config.Timestamp {
		embed.Timestamp = time.Now().Format(time.RFC3339)
	}

	return embed
}

// getColorForType returns the appropriate color for each response type
func (rm *ResponseManager) getColorForType(responseType ResponseType) int {
	if rm.config.Color != 0 {
		return rm.config.Color
	}

	switch responseType {
	case ResponseSuccess:
		return theme.Success()
	case ResponseError:
		return theme.Error()
	case ResponseWarning:
		return theme.Warning()
	case ResponseInfo:
		return theme.Info()
	case ResponseLoading:
		return theme.Loading()
	default:
		return theme.Muted()
	}
}

// getTitleForType returns the default title for each response type
func (rm *ResponseManager) getTitleForType(responseType ResponseType) string {
	switch responseType {
	case ResponseSuccess:
		return "Success"
	case ResponseError:
		return "Error"
	case ResponseWarning:
		return "Warning"
	case ResponseInfo:
		return "Information"
	case ResponseLoading:
		return "Loading..."
	default:
		return ""
	}
}

// Autocomplete sends an autocomplete response
func (rm *ResponseManager) Autocomplete(i *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) error {
	if len(choices) > 25 {
		choices = choices[:25]
	}

	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// DeferResponse defers the response (for long processing)
func (rm *ResponseManager) DeferResponse(i *discordgo.InteractionCreate, ephemeral bool) error {
	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: rm.buildFlags(ephemeral),
		},
	})
}

// EditResponse edits an already sent response
func (rm *ResponseManager) EditResponse(i *discordgo.InteractionCreate, content string) error {
	_, err := rm.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	return err
}

// EditResponseWithEmbed edits a response with an embed
func (rm *ResponseManager) EditResponseWithEmbed(i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) error {
	_, err := rm.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
	return err
}

// FollowUp sends a follow-up message
func (rm *ResponseManager) FollowUp(i *discordgo.InteractionCreate, content string, ephemeral bool) error {
	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	_, err := rm.session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   flags,
	})
	return err
}

// FollowUpWithEmbed sends a follow-up message with an embed
func (rm *ResponseManager) FollowUpWithEmbed(i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, ephemeral bool) error {
	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	_, err := rm.session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Flags:  flags,
	})
	return err
}

// DeleteResponse deletes the original response
func (rm *ResponseManager) DeleteResponse(i *discordgo.InteractionCreate) error {
	return rm.session.InteractionResponseDelete(i.Interaction)
}

// Builder pattern for fluent response construction

// ResponseBuilder builds responses fluently
type ResponseBuilder struct {
	manager *ResponseManager
	config  ResponseConfig
}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder(session *discordgo.Session) *ResponseBuilder {
	return &ResponseBuilder{
		manager: NewResponseManager(session),
		config:  ResponseConfig{},
	}
}

// Ephemeral makes the response ephemeral
func (rb *ResponseBuilder) Ephemeral() *ResponseBuilder {
	rb.config.Ephemeral = true
	return rb
}

// WithEmbed enables embed responses
func (rb *ResponseBuilder) WithEmbed() *ResponseBuilder {
	rb.config.WithEmbed = true
	return rb
}

// WithTitle sets a custom title
func (rb *ResponseBuilder) WithTitle(title string) *ResponseBuilder {
	rb.config.Title = title
	return rb
}

// WithColor sets a custom color
func (rb *ResponseBuilder) WithColor(color int) *ResponseBuilder {
	rb.config.Color = color
	return rb
}

// WithFooter adds a footer
func (rb *ResponseBuilder) WithFooter(footer string) *ResponseBuilder {
	rb.config.Footer = footer
	return rb
}

// WithTimestamp adds a timestamp
func (rb *ResponseBuilder) WithTimestamp() *ResponseBuilder {
	rb.config.Timestamp = true
	return rb
}

// WithComponents adds components (buttons, etc.)
func (rb *ResponseBuilder) WithComponents(components ...discordgo.MessageComponent) *ResponseBuilder {
	rb.config.Components = components
	return rb
}

// WithAttachments adds attachments
func (rb *ResponseBuilder) WithAttachments(files ...*discordgo.File) *ResponseBuilder {
	rb.config.Attachments = files
	return rb
}

// Build constructs the ResponseManager with the configuration
func (rb *ResponseBuilder) Build() *ResponseManager {
	return rb.manager.WithConfig(rb.config)
}

// Success sends a success response (convenience method)
func (rb *ResponseBuilder) Success(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Success(i, message)
}

// Error sends an error response (convenience method)
func (rb *ResponseBuilder) Error(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Error(i, message)
}

// Info sends an informational response (convenience method)
func (rb *ResponseBuilder) Info(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Info(i, message)
}

// Warning sends a warning response (convenience method)
func (rb *ResponseBuilder) Warning(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Warning(i, message)
}
