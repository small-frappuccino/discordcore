package qotd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const (
	groupName                             = "qotd"
	publishSubCommandName                 = "publish"
	questionsGroupName                    = "questions"
	questionsAddSubCommand                = "add"
	questionsListSubCommand               = "list"
	questionsQueueSubCommand              = "queue"
	questionsMarkPublishedSubCommand      = "mark_published"
	publishConsumeAutomaticSlotOptionName = "consume_automatic_slot"
	questionsRecoverSubCommand            = "recover"
	questionsRemoveSubCommand             = "remove"
	questionsBodyOptionName               = "question"
	questionsDeckOptionName               = "deck"
	questionsIDOptionName                 = "id"
	questionsPageSize                     = 10
	questionsListRouteFirst               = "qotd:questions:list:first"
	questionsListRoutePrev                = "qotd:questions:list:prev"
	questionsListRouteNext                = "qotd:questions:list:next"
	questionsListRouteLast                = "qotd:questions:list:last"
	questionsListDeniedText               = "Only the user who opened this list can change pages."
	questionsListUnknownDeck              = "QOTD deck not found"
	questionsListMissingGuild             = "This command can only be used in a server"
	questionsListIdleTimeout              = 60 * time.Second
	questionsListPageJumpSize             = 10
)

// QuestionCatalogService is the composed dependency the slash-command
// package needs as a wiring convenience. Each individual command struct
// below holds the narrower role it actually exercises so tests can mock
// the smaller surface; the top-level Commands constructor still takes one
// argument so callers don't have to wire two interfaces by hand. Anything
// that satisfies applicationqotd.QuestionCatalog AND
// applicationqotd.PublishCoordinator (which the monolithic *Service does)
// fits.
type QuestionCatalogService interface {
	applicationqotd.QuestionCatalog
	applicationqotd.PublishCoordinator
}

type Commands struct {
	service QuestionCatalogService
}

func NewCommands(service QuestionCatalogService) *Commands {
	return &Commands{service: service}
}

func (c *Commands) RegisterCommands(router *core.CommandRouter) {
	if router == nil || c == nil || c.service == nil {
		return
	}

	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	// The composed c.service satisfies both narrow roles; each command
	// receives only the role it uses so the test surface stays minimal.
	catalog := applicationqotd.QuestionCatalog(c.service)
	publisher := applicationqotd.PublishCoordinator(c.service)
	addCommand := &questionsAddCommand{service: catalog}
	listCommand := &questionsListCommand{service: catalog}
	queueCommand := &questionsQueueCommand{catalog: catalog, publish: publisher}
	markPublishedCommand := &questionsMarkPublishedCommand{service: catalog}
	publishCommand := &qotdPublishCommand{publishCoordinator: publisher, catalog: catalog}
	recoverCommand := &questionsRecoverCommand{service: catalog}
	removeCommand := &questionsRemoveCommand{service: catalog}
	questionsGroup := core.NewGroupCommand(questionsGroupName, "Browse QOTD deck questions", checker)
	questionsGroup.AddSubCommand(addCommand)
	questionsGroup.AddSubCommand(listCommand)
	questionsGroup.AddSubCommand(queueCommand)
	questionsGroup.AddSubCommand(markPublishedCommand)
	questionsGroup.AddSubCommand(recoverCommand)
	questionsGroup.AddSubCommand(removeCommand)

	var group *core.GroupCommand
	if existing, ok := router.GetRegistry().GetCommand(groupName); ok {
		if existingGroup, ok := existing.(*core.GroupCommand); ok {
			group = existingGroup
		}
	}
	if group == nil {
		group = core.NewGroupCommand(groupName, "Manage QOTD decks and questions", checker)
	}
	group.AddSubCommand(publishCommand)
	group.AddSubCommand(questionsGroup)
	router.RegisterSlashCommandForDomain(files.BotDomainQOTD, group)

	handler := core.ComponentHandlerFunc(func(ctx *core.Context) error {
		return listCommand.HandleComponent(ctx)
	})
	router.RegisterInteractionRoutesForDomain(
		files.BotDomainQOTD,
		core.InteractionRouteBinding{Path: questionsListRouteFirst, Component: handler},
		core.InteractionRouteBinding{Path: questionsListRoutePrev, Component: handler},
		core.InteractionRouteBinding{Path: questionsListRouteNext, Component: handler},
		core.InteractionRouteBinding{Path: questionsListRouteLast, Component: handler},
	)
}

// Each command struct declares only the role it actually exercises:
// catalog-only commands take QuestionCatalog, publish commands take
// PublishCoordinator. The wiring constructor (RegisterCommands) still
// receives one composed value and hands the right slice of behavior to
// each. This is the role-segregation half of the Service split — the
// concrete *qotd.Service stays whole on the implementation side, but
// callers no longer carry the union dependency.

type questionsListCommand struct {
	service        applicationqotd.QuestionCatalog
	controlsMu     sync.Mutex
	controlTimers  map[string]questionsListControlTimer
	idleTimeout    time.Duration
	editComponents questionsListMessageEditor
}

type questionsAddCommand struct {
	service applicationqotd.QuestionCatalog
}

type questionsMarkPublishedCommand struct {
	service applicationqotd.QuestionCatalog
}

type questionsQueueCommand struct {
	// queue needs both roles: Settings() (catalog) to resolve the deck
	// the user named, and GetAutomaticQueueState (publish coordinator)
	// to inspect the scheduler.
	catalog applicationqotd.QuestionCatalog
	publish applicationqotd.PublishCoordinator
}

type questionsRecoverCommand struct {
	service applicationqotd.QuestionCatalog
}

type questionsRemoveCommand struct {
	service applicationqotd.QuestionCatalog
}

type qotdPublishCommand struct {
	// publishCoordinator runs the manual publish. catalog is held
	// separately because the command needs to resolve the active deck
	// (Settings) before deciding whether the publish is allowed.
	publishCoordinator applicationqotd.PublishCoordinator
	catalog            applicationqotd.QuestionCatalog
}

type questionsListState struct {
	UserID string
	DeckID string
	Page   int
}

type questionsListControlTimer struct {
	generation uint64
	timer      *time.Timer
}

type questionsListMessageEditor func(session *discordgo.Session, channelID, messageID string, components []discordgo.MessageComponent) error

func (c *questionsListCommand) Name() string { return questionsListSubCommand }

func (c *questionsListCommand) Description() string {
	return "Show all questions in a QOTD deck"
}

func (c *questionsListCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        questionsDeckOptionName,
		Description: "Deck ID or exact deck name. Defaults to the active deck.",
		Required:    false,
	}}
}

func (c *questionsListCommand) RequiresGuild() bool       { return true }
func (c *questionsListCommand) RequiresPermissions() bool { return true }

func (c *questionsQueueCommand) Name() string { return questionsQueueSubCommand }

func (c *questionsQueueCommand) Description() string {
	return "Show the real automatic QOTD queue state"
}

func (c *questionsQueueCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        questionsDeckOptionName,
		Description: "Deck ID or exact deck name. Defaults to the active deck.",
		Required:    false,
	}}
}

func (c *questionsQueueCommand) RequiresGuild() bool       { return true }
func (c *questionsQueueCommand) RequiresPermissions() bool { return true }

func (c *questionsAddCommand) Name() string { return questionsAddSubCommand }

func (c *questionsAddCommand) Description() string {
	return "Add a question to a QOTD deck"
}

func (c *questionsAddCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        questionsBodyOptionName,
			Description: "Question text to add to the deck",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        questionsDeckOptionName,
			Description: "Deck ID or exact deck name. Defaults to the active deck.",
			Required:    false,
		},
	}
}

func (c *questionsAddCommand) RequiresGuild() bool       { return true }
func (c *questionsAddCommand) RequiresPermissions() bool { return true }

func (c *questionsAddCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	body, err := extractor.StringRequired(questionsBodyOptionName)
	if err != nil {
		return err
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}

	created, err := c.service.CreateQuestion(context.Background(), ctx.GuildID, ctx.UserID, applicationqotd.QuestionMutation{
		DeckID: deck.ID,
		Body:   body,
	})
	if err != nil {
		return translateQuestionsMutationError(err)
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("Added QOTD question ID %d to deck `%s`.", visibleQuestionID(*created), deck.Name))
}

func (c *questionsRemoveCommand) Name() string { return questionsRemoveSubCommand }

func (c *questionsRemoveCommand) Description() string {
	return "Remove a question from QOTD by visible ID"
}

func (c *questionsRecoverCommand) Name() string { return questionsRecoverSubCommand }

func (c *questionsRecoverCommand) Description() string {
	return "Move a used QOTD question back to ready so it can be published again"
}

func (c *questionsMarkPublishedCommand) Name() string { return questionsMarkPublishedSubCommand }

func (c *questionsMarkPublishedCommand) Description() string {
	return "Mark a ready QOTD question as already published"
}

func (c *questionsMarkPublishedCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        questionsIDOptionName,
			Description: "Question ID from the questions list embed",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        questionsDeckOptionName,
			Description: "Deck ID or exact deck name. Defaults to the active deck.",
			Required:    false,
		},
	}
}

func (c *questionsMarkPublishedCommand) RequiresGuild() bool       { return true }
func (c *questionsMarkPublishedCommand) RequiresPermissions() bool { return true }

func (c *questionsRemoveCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        questionsIDOptionName,
			Description: "Question ID from the questions list embed",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        questionsDeckOptionName,
			Description: "Deck ID or exact deck name. Defaults to the active deck.",
			Required:    false,
		},
	}
}

func (c *questionsRemoveCommand) RequiresGuild() bool       { return true }
func (c *questionsRemoveCommand) RequiresPermissions() bool { return true }

func (c *questionsRecoverCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        questionsIDOptionName,
			Description: "Question ID from the questions list embed",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        questionsDeckOptionName,
			Description: "Deck ID or exact deck name. Defaults to the active deck.",
			Required:    false,
		},
	}
}

func (c *questionsRecoverCommand) RequiresGuild() bool       { return true }
func (c *questionsRecoverCommand) RequiresPermissions() bool { return true }

func (c *questionsQueueCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	deck, err := loadCommandDeck(ctx, c.catalog, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	state, err := c.publish.GetAutomaticQueueState(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return translateQuestionsMutationError(err)
	}

	return core.NewResponseBuilder(ctx.Session).
		Info(ctx.Interaction, formatAutomaticQueueState(state))
}

func (c *qotdPublishCommand) Name() string { return publishSubCommandName }

func (c *qotdPublishCommand) Description() string {
	return "Publish the next ready QOTD question immediately"
}

func (c *qotdPublishCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionBoolean,
		Name:        publishConsumeAutomaticSlotOptionName,
		Description: "Whether this manual publish should occupy the current automatic slot",
		Required:    false,
	}}
}

func (c *qotdPublishCommand) RequiresGuild() bool       { return true }
func (c *qotdPublishCommand) RequiresPermissions() bool { return true }

// InteractionAckPolicy defers the slash response so the publish flow has the
// 15-minute follow-up window. Without this, the synchronous Discord/DB calls
// inside PublishNowWithParams routinely exceed the 3-second interaction
// timeout, producing 10062 ("Unknown interaction") on success.
func (c *qotdPublishCommand) InteractionAckPolicy() core.InteractionAckPolicy {
	return core.InteractionAckPolicy{Mode: core.InteractionAckModeDefer}
}

func (c *qotdPublishCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	deck, err := loadCommandDeck(ctx, c.catalog, "")
	if err != nil {
		return err
	}
	if !deck.Enabled {
		return core.NewCommandError("Enable QOTD publishing for the active deck before publishing manually.", false)
	}
	if strings.TrimSpace(deck.ChannelID) == "" {
		return core.NewCommandError("Set a QOTD channel for the active deck before publishing manually.", false)
	}
	consumeAutomaticSlot := true
	options := core.GetSubCommandOptions(ctx.Interaction)
	for _, option := range options {
		if option == nil || option.Name != publishConsumeAutomaticSlotOptionName {
			continue
		}
		consumeAutomaticSlot = core.NewOptionExtractor(options).Bool(publishConsumeAutomaticSlotOptionName)
		break
	}

	result, err := c.publishCoordinator.PublishNowWithParams(context.Background(), ctx.GuildID, ctx.Session, applicationqotd.PublishNowParams{
		ConsumeAutomaticSlot: &consumeAutomaticSlot,
	})
	if err != nil {
		return translatePublishNowError(err)
	}

	message := fmt.Sprintf("Published QOTD question ID %d manually from deck `%s`.", visibleQuestionID(result.Question), deck.Name)
	if !consumeAutomaticSlot {
		message = fmt.Sprintf("Published QOTD question ID %d manually from deck `%s` without consuming the automatic slot.", visibleQuestionID(result.Question), deck.Name)
	}
	if postURL := strings.TrimSpace(result.PostURL); postURL != "" {
		message = fmt.Sprintf("%s %s", message, postURL)
	}
	return core.NewResponseBuilder(ctx.Session).
		WithContext(ctx).
		Success(ctx.Interaction, message)
}

func (c *questionsRemoveCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	displayID := extractor.Int(questionsIDOptionName)
	if displayID <= 0 {
		return core.NewCommandError("Question ID must be greater than zero.", false)
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return err
	}
	question := findQuestionByDisplayID(questions, displayID)
	if question == nil {
		return translateQuestionsDeleteError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	if err := c.service.DeleteQuestion(context.Background(), ctx.GuildID, question.ID); err != nil {
		return translateQuestionsDeleteError(displayID, err)
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("Removed QOTD question ID %d from deck `%s`.", displayID, deck.Name))
}

func (c *questionsRecoverCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	displayID := extractor.Int(questionsIDOptionName)
	if displayID <= 0 {
		return core.NewCommandError("Question ID must be greater than zero.", false)
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return err
	}
	question := findQuestionByDisplayID(questions, displayID)
	if question == nil {
		return translateQuestionsRecoverError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	updated, err := c.service.RestoreUsedQuestion(context.Background(), ctx.GuildID, deck.ID, question.ID)
	if err != nil {
		return translateQuestionsRecoverError(displayID, err)
	}
	if updated == nil {
		return translateQuestionsRecoverError(displayID, applicationqotd.ErrQuestionNotFound)
	}
	if visibleQuestionID(*updated) == displayID {
		return core.NewResponseBuilder(ctx.Session).
			Success(ctx.Interaction, fmt.Sprintf("Recovered QOTD question ID %d from used to ready in deck `%s`.", displayID, deck.Name))
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("Recovered QOTD question ID %d from used to ready in deck `%s` and it is now listed as ID %d.", displayID, deck.Name, visibleQuestionID(*updated)))
}

func (c *questionsMarkPublishedCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	displayID := extractor.Int(questionsIDOptionName)
	if displayID <= 0 {
		return core.NewCommandError("Question ID must be greater than zero.", false)
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return err
	}
	question := findQuestionByDisplayID(questions, displayID)
	if question == nil {
		return translateQuestionsMarkPublishedError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	updated, err := c.service.MarkQuestionPublished(context.Background(), ctx.GuildID, deck.ID, question.ID)
	if err != nil {
		return translateQuestionsMarkPublishedError(displayID, err)
	}
	if updated == nil {
		return translateQuestionsMarkPublishedError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("Marked QOTD question ID %d as already published in deck `%s` without changing the day state.", displayID, deck.Name))
}

func (c *questionsListCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	requestedDeck := extractor.String(questionsDeckOptionName)
	view, err := c.loadView(ctx, requestedDeck)
	if err != nil {
		return err
	}

	state := questionsListState{
		UserID: strings.TrimSpace(ctx.UserID),
		DeckID: view.deck.ID,
		Page:   0,
	}
	if err := respondQuestionsList(ctx, view, state, false, true); err != nil {
		return err
	}
	c.armQuestionsListIdleTimeoutForOriginalResponse(ctx)
	return nil
}

func (c *questionsListCommand) HandleComponent(ctx *core.Context) error {
	if ctx == nil || ctx.Interaction == nil {
		return nil
	}
	action, state, err := parseQuestionsListState(ctx.RouteKey.CustomID)
	if err != nil {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, "Invalid questions list action.")
	}
	if strings.TrimSpace(ctx.UserID) != state.UserID {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, questionsListDeniedText)
	}

	view, err := c.loadView(ctx, state.DeckID)
	if err != nil {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, err.Error())
	}

	totalPages := discordqotdBuildPageCount(len(view.questions))
	state.Page = nextQuestionsListPage(action, state.Page, totalPages)
	if err := respondQuestionsList(ctx, view, state, false, false); err != nil {
		return err
	}
	c.armQuestionsListIdleTimeoutForMessage(ctx)
	return nil
}

type questionsListView struct {
	deck      files.QOTDDeckConfig
	questions []storage.QOTDQuestionRecord
}

func (c *questionsListCommand) loadView(ctx *core.Context, requestedDeck string) (questionsListView, error) {
	deck, err := loadCommandDeck(ctx, c.service, requestedDeck)
	if err != nil {
		return questionsListView{}, err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return questionsListView{}, err
	}
	return questionsListView{deck: deck, questions: questions}, nil
}

func requireQuestionsGuild(ctx *core.Context) error {
	if ctx == nil || ctx.Interaction == nil {
		return nil
	}
	if strings.TrimSpace(ctx.GuildID) == "" {
		return core.NewCommandError(questionsListMissingGuild, false)
	}
	return nil
}

// loadCommandDeck only needs Settings(), so it takes the narrow catalog
// role. Both PublishCoordinator-only and QuestionCatalog-only command
// structs can pass their dependency in without widening to the union.
func loadCommandDeck(ctx *core.Context, catalog applicationqotd.QuestionCatalog, requestedDeck string) (files.QOTDDeckConfig, error) {
	settings, err := catalog.Settings(ctx.GuildID)
	if err != nil {
		return files.QOTDDeckConfig{}, err
	}
	return resolveDeck(settings, requestedDeck)
}

func resolveDeck(settings files.QOTDConfig, requestedDeck string) (files.QOTDDeckConfig, error) {
	settings = files.DashboardQOTDConfig(settings)
	requestedDeck = strings.TrimSpace(requestedDeck)
	if requestedDeck == "" {
		if deck, ok := settings.ActiveDeck(); ok {
			return deck, nil
		}
		return files.QOTDDeckConfig{}, core.NewCommandError(questionsListUnknownDeck, false)
	}

	if deck, ok := settings.DeckByID(requestedDeck); ok {
		return deck, nil
	}
	for _, deck := range settings.Decks {
		if strings.EqualFold(strings.TrimSpace(deck.Name), requestedDeck) {
			return deck, nil
		}
	}
	return files.QOTDDeckConfig{}, core.NewCommandError(fmt.Sprintf("%s: %s", questionsListUnknownDeck, requestedDeck), false)
}

func respondQuestionsList(
	ctx *core.Context,
	view questionsListView,
	state questionsListState,
	ephemeral bool,
	initial bool,
) error {
	totalQuestions := len(view.questions)
	totalPages := discordqotdBuildPageCount(totalQuestions)
	state.Page = normalizeQuestionsListPage(state.Page, totalPages)
	embed := discordqotd.BuildQuestionsListEmbed(discordqotd.QuestionsListEmbedParams{
		DeckName:       view.deck.Name,
		Questions:      view.questions,
		Page:           state.Page,
		PageSize:       questionsPageSize,
		TotalQuestions: totalQuestions,
	})
	components := buildQuestionsListComponents(state, totalPages)
	return sendQuestionsListResponse(ctx, embed, components, ephemeral, initial)
}

func buildQuestionsListComponents(state questionsListState, totalPages int) []discordgo.MessageComponent {
	page := normalizeQuestionsListPage(state.Page, totalPages)
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: encodeQuestionsListState(questionsListRouteFirst, state.withPage(page)), Label: "<<", Style: discordgo.SecondaryButton, Disabled: page == 0 || totalPages <= 1},
			discordgo.Button{CustomID: encodeQuestionsListState(questionsListRoutePrev, state.withPage(page)), Label: "<", Style: discordgo.PrimaryButton, Disabled: page == 0 || totalPages <= 1},
			discordgo.Button{CustomID: encodeQuestionsListState(questionsListRouteNext, state.withPage(page)), Label: ">", Style: discordgo.PrimaryButton, Disabled: page >= totalPages-1 || totalPages <= 1},
			discordgo.Button{CustomID: encodeQuestionsListState(questionsListRouteLast, state.withPage(page)), Label: ">>", Style: discordgo.SecondaryButton, Disabled: page >= totalPages-1 || totalPages <= 1},
		}},
	}
}

func sendQuestionsListResponse(
	ctx *core.Context,
	embed *discordgo.MessageEmbed,
	components []discordgo.MessageComponent,
	ephemeral bool,
	initial bool,
) error {
	if initial {
		builder := core.NewResponseBuilder(ctx.Session).WithComponents(components...)
		if ephemeral {
			builder = builder.Ephemeral()
		}
		return builder.Build().Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
	}

	return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})
}

func encodeQuestionsListState(routeID string, state questionsListState) string {
	return fmt.Sprintf("%s|%s|%s|%d", routeID, state.UserID, state.DeckID, state.Page)
}

func parseQuestionsListState(customID string) (string, questionsListState, error) {
	routeID, rawState, found := strings.Cut(strings.TrimSpace(customID), "|")
	if !found {
		return "", questionsListState{}, fmt.Errorf("missing questions list state")
	}
	parts := strings.Split(rawState, "|")
	if len(parts) != 3 {
		return "", questionsListState{}, fmt.Errorf("invalid questions list state")
	}
	page, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", questionsListState{}, fmt.Errorf("invalid page state: %w", err)
	}
	return routeID, questionsListState{
		UserID: strings.TrimSpace(parts[0]),
		DeckID: strings.TrimSpace(parts[1]),
		Page:   page,
	}, nil
}

func nextQuestionsListPage(action string, currentPage int, totalPages int) int {
	currentPage = normalizeQuestionsListPage(currentPage, totalPages)
	switch action {
	case questionsListRouteFirst:
		return normalizeQuestionsListPage(currentPage-questionsListPageJumpSize, totalPages)
	case questionsListRoutePrev:
		return normalizeQuestionsListPage(currentPage-1, totalPages)
	case questionsListRouteNext:
		return normalizeQuestionsListPage(currentPage+1, totalPages)
	case questionsListRouteLast:
		return normalizeQuestionsListPage(currentPage+questionsListPageJumpSize, totalPages)
	default:
		return currentPage
	}
}

func normalizeQuestionsListPage(page int, totalPages int) int {
	if totalPages <= 0 {
		return 0
	}
	if page < 0 {
		return 0
	}
	if page >= totalPages {
		return totalPages - 1
	}
	return page
}

func discordqotdBuildPageCount(totalQuestions int) int {
	if totalQuestions <= 0 {
		return 1
	}
	pages := totalQuestions / questionsPageSize
	if totalQuestions%questionsPageSize != 0 {
		pages++
	}
	if pages <= 0 {
		return 1
	}
	return pages
}

func (state questionsListState) withPage(page int) questionsListState {
	state.Page = page
	return state
}

func (c *questionsListCommand) currentQuestionsListIdleTimeout() time.Duration {
	if c != nil && c.idleTimeout > 0 {
		return c.idleTimeout
	}
	return questionsListIdleTimeout
}

func (c *questionsListCommand) currentQuestionsListMessageEditor() questionsListMessageEditor {
	if c != nil && c.editComponents != nil {
		return c.editComponents
	}
	return editQuestionsListMessageComponents
}

func (c *questionsListCommand) armQuestionsListIdleTimeoutForOriginalResponse(ctx *core.Context) {
	if c == nil || ctx == nil || ctx.Session == nil || ctx.Interaction == nil || ctx.Interaction.Interaction == nil {
		return
	}
	message, err := ctx.Session.InteractionResponse(ctx.Interaction.Interaction)
	if err != nil || message == nil {
		return
	}
	c.armQuestionsListIdleTimeout(ctx.Session, message.ChannelID, message.ID)
}

func (c *questionsListCommand) armQuestionsListIdleTimeoutForMessage(ctx *core.Context) {
	if c == nil || ctx == nil || ctx.Session == nil || ctx.Interaction == nil || ctx.Interaction.Message == nil {
		return
	}
	c.armQuestionsListIdleTimeout(ctx.Session, ctx.Interaction.Message.ChannelID, ctx.Interaction.Message.ID)
}

func (c *questionsListCommand) armQuestionsListIdleTimeout(session *discordgo.Session, channelID, messageID string) {
	if c == nil {
		return
	}
	channelID = strings.TrimSpace(channelID)
	messageID = strings.TrimSpace(messageID)
	if channelID == "" || messageID == "" {
		return
	}
	timeout := c.currentQuestionsListIdleTimeout()
	if timeout <= 0 {
		return
	}

	c.controlsMu.Lock()
	if c.controlTimers == nil {
		c.controlTimers = make(map[string]questionsListControlTimer)
	}
	entry := c.controlTimers[messageID]
	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.generation++
	generation := entry.generation
	entry.timer = time.AfterFunc(timeout, func() {
		c.hideQuestionsListControls(session, channelID, messageID, generation)
	})
	c.controlTimers[messageID] = entry
	c.controlsMu.Unlock()
}

func (c *questionsListCommand) hideQuestionsListControls(session *discordgo.Session, channelID, messageID string, generation uint64) {
	if c == nil {
		return
	}

	c.controlsMu.Lock()
	entry, ok := c.controlTimers[messageID]
	if !ok || entry.generation != generation {
		c.controlsMu.Unlock()
		return
	}
	delete(c.controlTimers, messageID)
	c.controlsMu.Unlock()

	_ = c.currentQuestionsListMessageEditor()(session, channelID, messageID, []discordgo.MessageComponent{})
}

func editQuestionsListMessageComponents(session *discordgo.Session, channelID, messageID string, components []discordgo.MessageComponent) error {
	if session == nil {
		return fmt.Errorf("discord session is required")
	}
	edit := &discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Components: &components,
	}
	_, err := session.ChannelMessageEditComplex(edit)
	return err
}

func visibleQuestionID(question storage.QOTDQuestionRecord) int64 {
	if question.DisplayID > 0 {
		return question.DisplayID
	}
	return question.ID
}

func formatAutomaticQueueState(state applicationqotd.AutomaticQueueState) string {
	deckName := strings.TrimSpace(state.Deck.Name)
	if deckName == "" {
		deckName = "Default"
	}
	lines := []string{fmt.Sprintf("Automatic QOTD queue for deck `%s`.", deckName)}

	if !state.ScheduleConfigured {
		lines = append(lines, "Automatic publish schedule is not configured.")
	} else {
		lines = append(lines, fmt.Sprintf("Automatic schedule: %s UTC.", formatAutomaticQueueSchedule(state.Schedule)))
		lines = append(lines, fmt.Sprintf("Next automatic slot: %s (%s).", formatAutomaticQueueSlotInstant(state.SlotPublishAtUTC), formatAutomaticQueueSlotStatus(state.SlotStatus)))
	}

	if !state.Deck.Enabled {
		lines = append(lines, "Publishing is disabled for this deck.")
	} else if strings.TrimSpace(state.Deck.ChannelID) == "" {
		lines = append(lines, "Set a QOTD channel before automatic publishing can run.")
	}

	if state.SlotQuestion != nil {
		lines = append(lines, fmt.Sprintf("Next automatic slot question: %s.", formatAutomaticQueueQuestion(*state.SlotQuestion)))
	}

	if state.NextReadyQuestion != nil {
		label := "Next automatic question"
		if state.SlotQuestion != nil || state.SlotStatus == applicationqotd.AutomaticQueueSlotStatusPublished {
			label = "After that"
		}
		lines = append(lines, fmt.Sprintf("%s: %s.", label, formatAutomaticQueueQuestion(*state.NextReadyQuestion)))
	} else if state.SlotQuestion == nil {
		lines = append(lines, "No ready QOTD questions are available for the automatic queue.")
	}

	return strings.Join(lines, "\n")
}

func formatAutomaticQueueSchedule(schedule applicationqotd.PublishSchedule) string {
	hourUTC, minuteUTC, ok := schedule.Values()
	if !ok {
		return "unavailable"
	}
	return fmt.Sprintf("%02d:%02d", hourUTC, minuteUTC)
}

// formatAutomaticQueueSlotInstant renders the next slot as an absolute UTC
// stamp plus a Discord timestamp anchor that Discord renders in the viewer's
// local timezone, so a moderator does not have to do the UTC arithmetic
// themselves.
func formatAutomaticQueueSlotInstant(value time.Time) string {
	if value.IsZero() {
		return "unavailable"
	}
	utc := value.UTC()
	return fmt.Sprintf("%s — <t:%d:F> in your timezone (<t:%d:R>)", utc.Format("2006-01-02 15:04 UTC"), utc.Unix(), utc.Unix())
}

func formatAutomaticQueueSlotStatus(status applicationqotd.AutomaticQueueSlotStatus) string {
	switch status {
	case applicationqotd.AutomaticQueueSlotStatusWaiting:
		return "waiting for the scheduled publish"
	case applicationqotd.AutomaticQueueSlotStatusDue:
		return "ready to publish now"
	case applicationqotd.AutomaticQueueSlotStatusReserved:
		return "question reserved for the slot"
	case applicationqotd.AutomaticQueueSlotStatusRecovering:
		return "slot publish recovery pending"
	case applicationqotd.AutomaticQueueSlotStatusPublished:
		return "slot already published"
	case applicationqotd.AutomaticQueueSlotStatusDisabled:
		fallthrough
	default:
		return "automatic publishing unavailable"
	}
}

func formatAutomaticQueueQuestion(question storage.QOTDQuestionRecord) string {
	body := strings.Join(strings.Fields(strings.TrimSpace(question.Body)), " ")
	if len(body) > 72 {
		body = body[:69] + "..."
	}
	return fmt.Sprintf("QOTD question ID %d (%s)", visibleQuestionID(question), body)
}

func formatCountNoun(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func findQuestionByDisplayID(questions []storage.QOTDQuestionRecord, displayID int64) *storage.QOTDQuestionRecord {
	for idx := range questions {
		if visibleQuestionID(questions[idx]) == displayID {
			return &questions[idx]
		}
	}
	return nil
}

func translateQuestionsMutationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, files.ErrInvalidQOTDInput) {
		message := strings.TrimSpace(strings.TrimPrefix(err.Error(), files.ErrInvalidQOTDInput.Error()+":"))
		if message == "" {
			message = "Invalid QOTD question input"
		}
		return core.NewCommandError(message, false)
	}
	return err
}

func translateQuestionsDeleteError(questionID int64, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotFound) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d was not found.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrImmutableQuestion) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d is already scheduled or used and cannot be removed.", questionID), false)
	}
	return translateQuestionsMutationError(err)
}

func translateQuestionsRecoverError(questionID int64, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotFound) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d was not found.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotUsed) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d is not used and cannot be recovered.", questionID), false)
	}
	return translateQuestionsMutationError(err)
}

func translateQuestionsMarkPublishedError(questionID int64, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotFound) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d was not found.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrImmutableQuestion) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d is already scheduled or published and cannot be marked manually.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotReady) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d must be ready before it can be marked as published.", questionID), false)
	}
	return translateQuestionsMutationError(err)
}

func translatePublishNowError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrAlreadyPublished) {
		return core.NewCommandError("A QOTD question has already been published for the current slot.", false)
	}
	if errors.Is(err, applicationqotd.ErrPublishInProgress) {
		return core.NewCommandError("A QOTD publish is already in progress for the current slot.", false)
	}
	if errors.Is(err, applicationqotd.ErrNoQuestionsAvailable) {
		return core.NewCommandError("No ready QOTD questions are available in the active deck.", false)
	}
	if errors.Is(err, applicationqotd.ErrQOTDDisabled) {
		return core.NewCommandError("Enable QOTD publishing and set a channel before publishing manually.", false)
	}
	if errors.Is(err, applicationqotd.ErrDiscordUnavailable) {
		return core.NewCommandError("Discord session unavailable for manual publish.", false)
	}
	return err
}

var _ core.SubCommand = (*questionsAddCommand)(nil)
var _ core.SubCommand = (*questionsListCommand)(nil)
var _ core.SubCommand = (*questionsQueueCommand)(nil)
var _ core.SubCommand = (*questionsMarkPublishedCommand)(nil)
var _ core.SubCommand = (*questionsRecoverCommand)(nil)
var _ core.SubCommand = (*questionsRemoveCommand)(nil)
var _ core.SubCommand = (*qotdPublishCommand)(nil)

var _ QuestionCatalogService = (*applicationqotd.Service)(nil)
