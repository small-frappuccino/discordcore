package core

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// ResponseType define tipos de resposta padronizados
type ResponseType int

const (
	ResponseSuccess ResponseType = iota
	ResponseError
	ResponseWarning
	ResponseInfo
	ResponseLoading
)

// ResponseConfig configura opções de resposta
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

// ResponseManager gerencia todas as respostas de interação
type ResponseManager struct {
	session *discordgo.Session
	config  ResponseConfig
}

// NewResponseManager cria um novo gerenciador de respostas
func NewResponseManager(session *discordgo.Session) *ResponseManager {
	return &ResponseManager{
		session: session,
		config:  ResponseConfig{},
	}
}

// WithConfig define configurações para a próxima resposta
func (rm *ResponseManager) WithConfig(config ResponseConfig) *ResponseManager {
	return &ResponseManager{
		session: rm.session,
		config:  config,
	}
}

// Success envia uma resposta de sucesso
func (rm *ResponseManager) Success(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseSuccess)
}

// Error envia uma resposta de erro
func (rm *ResponseManager) Error(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseError)
}

// Warning envia uma resposta de aviso
func (rm *ResponseManager) Warning(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseWarning)
}

// Info envia uma resposta informativa
func (rm *ResponseManager) Info(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseInfo)
}

// Loading envia uma resposta de carregamento
func (rm *ResponseManager) Loading(i *discordgo.InteractionCreate, message string) error {
	return rm.sendResponse(i, message, ResponseLoading)
}

// Ephemeral envia uma resposta ephemeral simples
func (rm *ResponseManager) Ephemeral(i *discordgo.InteractionCreate, message string) error {
	config := rm.config
	config.Ephemeral = true
	return rm.WithConfig(config).Info(i, message)
}

// Custom envia uma resposta personalizada
func (rm *ResponseManager) Custom(i *discordgo.InteractionCreate, content string, embeds []*discordgo.MessageEmbed) error {
	var flags discordgo.MessageFlags
	if rm.config.Ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    content,
			Embeds:     embeds,
			Flags:      flags,
			Components: rm.config.Components,
			Files:      rm.config.Attachments,
		},
	})
}

// sendResponse envia uma resposta baseada no tipo
func (rm *ResponseManager) sendResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	if rm.config.WithEmbed {
		return rm.sendEmbedResponse(i, message, responseType)
	}
	return rm.sendTextResponse(i, message, responseType)
}

// sendTextResponse envia uma resposta de texto simples
func (rm *ResponseManager) sendTextResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	content := rm.formatTextMessage(message, responseType)

	var flags discordgo.MessageFlags
	if rm.config.Ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    content,
			Flags:      flags,
			Components: rm.config.Components,
			Files:      rm.config.Attachments,
		},
	})
}

// sendEmbedResponse envia uma resposta com embed
func (rm *ResponseManager) sendEmbedResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	embed := rm.createEmbed(message, responseType)

	var flags discordgo.MessageFlags
	if rm.config.Ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Flags:      flags,
			Components: rm.config.Components,
			Files:      rm.config.Attachments,
		},
	})
}

// formatTextMessage formata mensagem de texto baseada no tipo
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

// createEmbed cria um embed baseado no tipo de resposta
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

// getColorForType retorna a cor apropriada para cada tipo de resposta
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

// getTitleForType retorna o título padrão para cada tipo de resposta
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

// Autocomplete envia uma resposta de autocomplete
func (rm *ResponseManager) Autocomplete(i *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) error {
	if len(choices) > 25 {
		choices = choices[:25]
	}

	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// DeferResponse adia a resposta (para processamento longo)
func (rm *ResponseManager) DeferResponse(i *discordgo.InteractionCreate, ephemeral bool) error {
	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: flags,
		},
	})
}

// EditResponse edita uma resposta já enviada
func (rm *ResponseManager) EditResponse(i *discordgo.InteractionCreate, content string) error {
	_, err := rm.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	return err
}

// EditResponseWithEmbed edita uma resposta com embed
func (rm *ResponseManager) EditResponseWithEmbed(i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) error {
	_, err := rm.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
	return err
}

// FollowUp envia uma mensagem de follow-up
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

// FollowUpWithEmbed envia uma mensagem de follow-up com embed
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

// DeleteResponse deleta a resposta original
func (rm *ResponseManager) DeleteResponse(i *discordgo.InteractionCreate) error {
	return rm.session.InteractionResponseDelete(i.Interaction)
}

// Builder pattern para construção fluente de respostas

// ResponseBuilder constrói respostas de forma fluente
type ResponseBuilder struct {
	manager *ResponseManager
	config  ResponseConfig
}

// NewResponseBuilder cria um novo construtor de respostas
func NewResponseBuilder(session *discordgo.Session) *ResponseBuilder {
	return &ResponseBuilder{
		manager: NewResponseManager(session),
		config:  ResponseConfig{},
	}
}

// Ephemeral torna a resposta ephemeral
func (rb *ResponseBuilder) Ephemeral() *ResponseBuilder {
	rb.config.Ephemeral = true
	return rb
}

// WithEmbed habilita respostas com embed
func (rb *ResponseBuilder) WithEmbed() *ResponseBuilder {
	rb.config.WithEmbed = true
	return rb
}

// WithTitle define um título personalizado
func (rb *ResponseBuilder) WithTitle(title string) *ResponseBuilder {
	rb.config.Title = title
	return rb
}

// WithColor define uma cor personalizada
func (rb *ResponseBuilder) WithColor(color int) *ResponseBuilder {
	rb.config.Color = color
	return rb
}

// WithFooter adiciona um footer
func (rb *ResponseBuilder) WithFooter(footer string) *ResponseBuilder {
	rb.config.Footer = footer
	return rb
}

// WithTimestamp adiciona timestamp
func (rb *ResponseBuilder) WithTimestamp() *ResponseBuilder {
	rb.config.Timestamp = true
	return rb
}

// WithComponents adiciona componentes (botões, etc.)
func (rb *ResponseBuilder) WithComponents(components ...discordgo.MessageComponent) *ResponseBuilder {
	rb.config.Components = components
	return rb
}

// WithAttachments adiciona anexos
func (rb *ResponseBuilder) WithAttachments(files ...*discordgo.File) *ResponseBuilder {
	rb.config.Attachments = files
	return rb
}

// Build constrói o ResponseManager com as configurações
func (rb *ResponseBuilder) Build() *ResponseManager {
	return rb.manager.WithConfig(rb.config)
}

// Success envia uma resposta de sucesso (método de conveniência)
func (rb *ResponseBuilder) Success(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Success(i, message)
}

// Error envia uma resposta de erro (método de conveniência)
func (rb *ResponseBuilder) Error(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Error(i, message)
}

// Info envia uma resposta informativa (método de conveniência)
func (rb *ResponseBuilder) Info(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Info(i, message)
}

// Warning envia uma resposta de aviso (método de conveniência)
func (rb *ResponseBuilder) Warning(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Warning(i, message)
}
