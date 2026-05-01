package qotd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	groupName                 = "qotd"
	publishSubCommandName     = "publish"
	questionsGroupName        = "questions"
	questionsAddSubCommand    = "add"
	questionsListSubCommand   = "list"
	questionsQueueSubCommand  = "queue"
	questionsNextSubCommand   = "next"
	questionsResetSubCommand  = "reset"
	questionsRecoverSubCommand = "recover"
	questionsRemoveSubCommand = "remove"
	questionsImportSubCommand = "import"
	questionsBodyOptionName   = "question"
	questionsDeckOptionName   = "deck"
	questionsIDOptionName     = "id"
	questionsImportUsersName  = "user_ids"
	questionsImportChannel    = "channel"
	questionsImportStartDate  = "start_date"
	questionsPageSize         = 10
	questionsListRouteFirst   = "qotd:questions:list:first"
	questionsListRoutePrev    = "qotd:questions:list:prev"
	questionsListRouteNext    = "qotd:questions:list:next"
	questionsListRouteLast    = "qotd:questions:list:last"
	questionsListDeniedText   = "Only the user who opened this list can change pages."
	questionsListUnknownDeck  = "QOTD deck not found"
	questionsListMissingGuild = "This command can only be used in a server"
	questionsListIdleTimeout  = 60 * time.Second
	questionsListPageJumpSize = 10
)

type QuestionCatalogService interface {
	Settings(guildID string) (files.QOTDConfig, error)
	ListQuestions(ctx context.Context, guildID, deckID string) ([]storage.QOTDQuestionRecord, error)
	CreateQuestion(ctx context.Context, guildID, actorID string, mutation applicationqotd.QuestionMutation) (*storage.QOTDQuestionRecord, error)
	DeleteQuestion(ctx context.Context, guildID string, questionID int64) error
	SetNextQuestion(ctx context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error)
	RestoreUsedQuestion(ctx context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error)
	ResetDeckState(ctx context.Context, guildID, deckID string) (applicationqotd.ResetDeckResult, error)
	GetAutomaticQueueState(ctx context.Context, guildID, deckID string) (applicationqotd.AutomaticQueueState, error)
	ImportArchivedQuestions(ctx context.Context, guildID, actorID string, session *discordgo.Session, params applicationqotd.ImportArchivedQuestionsParams) (applicationqotd.ImportArchivedQuestionsResult, error)
	PublishNow(ctx context.Context, guildID string, session *discordgo.Session) (*applicationqotd.PublishResult, error)
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
	addCommand := &questionsAddCommand{service: c.service}
	listCommand := &questionsListCommand{service: c.service}
	queueCommand := &questionsQueueCommand{service: c.service}
	nextCommand := &questionsNextCommand{service: c.service}
	importCommand := &questionsImportCommand{service: c.service}
	publishCommand := &qotdPublishCommand{service: c.service}
	resetCommand := &questionsResetCommand{service: c.service}
	recoverCommand := &questionsRecoverCommand{service: c.service}
	removeCommand := &questionsRemoveCommand{service: c.service}
	questionsGroup := core.NewGroupCommand(questionsGroupName, "Browse QOTD deck questions", checker)
	questionsGroup.AddSubCommand(addCommand)
	questionsGroup.AddSubCommand(listCommand)
	questionsGroup.AddSubCommand(queueCommand)
	questionsGroup.AddSubCommand(nextCommand)
	questionsGroup.AddSubCommand(importCommand)
	questionsGroup.AddSubCommand(resetCommand)
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
	router.RegisterSlashCommand(group)

	handler := core.ComponentHandlerFunc(func(ctx *core.Context) error {
		return listCommand.HandleComponent(ctx)
	})
	router.RegisterInteractionRoutes(
		core.InteractionRouteBinding{Path: questionsListRouteFirst, Component: handler},
		core.InteractionRouteBinding{Path: questionsListRoutePrev, Component: handler},
		core.InteractionRouteBinding{Path: questionsListRouteNext, Component: handler},
		core.InteractionRouteBinding{Path: questionsListRouteLast, Component: handler},
	)
}

type questionsListCommand struct {
	service        QuestionCatalogService
	controlsMu     sync.Mutex
	controlTimers  map[string]questionsListControlTimer
	idleTimeout    time.Duration
	editComponents questionsListMessageEditor
}

type questionsAddCommand struct {
	service QuestionCatalogService
}

type questionsNextCommand struct {
	service QuestionCatalogService
}

type questionsQueueCommand struct {
	service QuestionCatalogService
}

type questionsRecoverCommand struct {
	service QuestionCatalogService
}

type questionsRemoveCommand struct {
	service QuestionCatalogService
}

type questionsImportCommand struct {
	service QuestionCatalogService
}

type questionsResetCommand struct {
	service QuestionCatalogService
}

type qotdPublishCommand struct {
	service QuestionCatalogService
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
	return "Exceptionally move a used QOTD question back to ready by visible ID"
}

func (c *questionsNextCommand) Name() string { return questionsNextSubCommand }

func (c *questionsNextCommand) Description() string {
	return "Set which ready QOTD question publishes next"
}

func (c *questionsNextCommand) Options() []*discordgo.ApplicationCommandOption {
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

func (c *questionsNextCommand) RequiresGuild() bool       { return true }
func (c *questionsNextCommand) RequiresPermissions() bool { return true }

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

func (c *questionsResetCommand) Name() string { return questionsResetSubCommand }

func (c *questionsImportCommand) Name() string { return questionsImportSubCommand }

func (c *questionsImportCommand) Description() string {
	return "Import historical QOTD posts from another bot into the current deck as used questions"
}

func (c *questionsImportCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        questionsImportUsersName,
			Description: "One user ID or a comma/space-separated list of bot user IDs to import from",
			Required:    true,
		},
		{
			Type:         discordgo.ApplicationCommandOptionChannel,
			Name:         questionsImportChannel,
			Description:  "Text channel to backread for historical QOTD posts",
			Required:     true,
			ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        questionsImportStartDate,
			Description: "Import only messages sent on or after this UTC date (YYYY-MM-DD)",
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

func (c *questionsImportCommand) RequiresGuild() bool       { return true }
func (c *questionsImportCommand) RequiresPermissions() bool { return true }

func (c *questionsResetCommand) Description() string {
	return "Reset question states and clear automatic/manual QOTD publish state"
}

func (c *questionsResetCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        questionsDeckOptionName,
		Description: "Deck ID or exact deck name. Defaults to the active deck.",
		Required:    false,
	}}
}

func (c *questionsResetCommand) RequiresGuild() bool       { return true }
func (c *questionsResetCommand) RequiresPermissions() bool { return true }

func (c *questionsResetCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}

	result, err := c.service.ResetDeckState(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return translateQuestionsMutationError(err)
	}
	if result.QuestionsReset == 0 && result.OfficialPostsCleared == 0 {
		return core.NewResponseBuilder(ctx.Session).
			Info(ctx.Interaction, fmt.Sprintf("No QOTD question states or publish history needed reset in deck `%s`. Question order was unchanged.", deck.Name))
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, describeResetDeckResult(result, deck.Name))
}

func (c *questionsImportCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	rawUserIDs, err := extractor.StringRequired(questionsImportUsersName)
	if err != nil {
		return err
	}
	authorIDs, err := parseQuestionsImportAuthorIDs(rawUserIDs)
	if err != nil {
		return err
	}
	channelID := questionsChannelOptionID(ctx.Session, core.GetSubCommandOptions(ctx.Interaction), questionsImportChannel)
	if channelID == "" {
		return core.NewCommandError("Channel is required.", false)
	}
	startDate, err := extractor.StringRequired(questionsImportStartDate)
	if err != nil {
		return err
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}

	rm := core.NewResponseManager(ctx.Session)
	if err := rm.DeferResponse(ctx.Interaction, false); err != nil {
		return err
	}

	result, err := c.service.ImportArchivedQuestions(context.Background(), ctx.GuildID, ctx.UserID, ctx.Session, applicationqotd.ImportArchivedQuestionsParams{
		DeckID:          deck.ID,
		SourceChannelID: channelID,
		AuthorIDs:       authorIDs,
		StartDate:       startDate,
		BackupDir:       defaultQuestionsImportBackupDir(),
	})
	if err != nil {
		return rm.EditResponse(ctx.Interaction, describeQuestionsImportError(translateQuestionsImportError(err)))
	}
	if result.MatchedMessages == 0 {
		return rm.EditResponse(ctx.Interaction, fmt.Sprintf("No historical QOTD questions matched in <#%s> since %s for deck `%s`.", channelID, startDate, deck.Name))
	}

	return rm.EditResponse(ctx.Interaction, describeQuestionsImportResult(deck.Name, channelID, result))
}

func describeQuestionsImportError(err error) string {
	if err == nil {
		return "An error occurred while importing historical QOTD questions."
	}
	var cmdErr *core.CommandError
	if errors.As(err, &cmdErr) && cmdErr != nil && strings.TrimSpace(cmdErr.Message) != "" {
		return cmdErr.Message
	}
	return err.Error()
}

func (c *questionsQueueCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	state, err := c.service.GetAutomaticQueueState(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return translateQuestionsMutationError(err)
	}

	return core.NewResponseBuilder(ctx.Session).
		Info(ctx.Interaction, formatAutomaticQueueState(state))
}

func (c *questionsNextCommand) Handle(ctx *core.Context) error {
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
		return translateQuestionsSetNextError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	updated, err := c.service.SetNextQuestion(context.Background(), ctx.GuildID, deck.ID, question.ID)
	if err != nil {
		return translateQuestionsSetNextError(displayID, err)
	}
	if updated == nil {
		return translateQuestionsSetNextError(displayID, applicationqotd.ErrQuestionNotFound)
	}
	if visibleQuestionID(*updated) == displayID {
		return core.NewResponseBuilder(ctx.Session).
			Info(ctx.Interaction, fmt.Sprintf("QOTD question ID %d is already the next ready question in deck `%s`.", displayID, deck.Name))
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD question ID %d is now the next ready question in deck `%s` and is now listed as ID %d.", displayID, deck.Name, visibleQuestionID(*updated)))
}

func (c *qotdPublishCommand) Name() string { return publishSubCommandName }

func (c *qotdPublishCommand) Description() string {
	return "Publish the next ready QOTD question immediately"
}

func (c *qotdPublishCommand) Options() []*discordgo.ApplicationCommandOption { return nil }

func (c *qotdPublishCommand) RequiresGuild() bool       { return true }
func (c *qotdPublishCommand) RequiresPermissions() bool { return true }

func (c *qotdPublishCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	deck, err := loadCommandDeck(ctx, c.service, "")
	if err != nil {
		return err
	}
	if !deck.Enabled {
		return core.NewCommandError("Enable QOTD publishing for the active deck before publishing manually.", false)
	}
	if strings.TrimSpace(deck.ChannelID) == "" {
		return core.NewCommandError("Set a QOTD channel for the active deck before publishing manually.", false)
	}

	result, err := c.service.PublishNow(context.Background(), ctx.GuildID, ctx.Session)
	if err != nil {
		return translatePublishNowError(err)
	}

	message := fmt.Sprintf("Published QOTD question ID %d manually.", visibleQuestionID(result.Question))
	if postURL := strings.TrimSpace(result.PostURL); postURL != "" {
		message = fmt.Sprintf("%s %s", message, postURL)
	}
	return core.NewResponseBuilder(ctx.Session).
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

func loadCommandDeck(ctx *core.Context, service QuestionCatalogService, requestedDeck string) (files.QOTDDeckConfig, error) {
	settings, err := service.Settings(ctx.GuildID)
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

func questionsChannelOptionID(session *discordgo.Session, options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option == nil || option.Name != name {
			continue
		}
		if channel := option.ChannelValue(session); channel != nil {
			return strings.TrimSpace(channel.ID)
		}
		if value, ok := option.Value.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseQuestionsImportAuthorIDs(value string) ([]string, error) {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return nil, core.NewCommandError("Provide one user ID or a comma/space-separated list of user IDs.", false)
	}

	ids := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "<@!")
		part = strings.TrimPrefix(part, "<@")
		part = strings.TrimSuffix(part, ">")
		if part == "" {
			continue
		}
		if !isCommandNumericID(part) {
			return nil, core.NewCommandError("User IDs must be numeric Discord IDs.", false)
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		ids = append(ids, part)
	}
	if len(ids) == 0 {
		return nil, core.NewCommandError("Provide at least one numeric Discord user ID.", false)
	}
	return ids, nil
}

func isCommandNumericID(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func defaultQuestionsImportBackupDir() string {
	return filepath.Join("D:", "backups", "qotd-imports")
}

func displayQuestionsImportBackupPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		if rel, err := filepath.Rel(wd, path); err == nil && rel != "" && rel != "." && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}

func describeQuestionsImportResult(deckName, channelID string, result applicationqotd.ImportArchivedQuestionsResult) string {
	parts := []string{fmt.Sprintf("Scanned %d messages in <#%s> and matched %d historical QOTD prompts for deck `%s`.", result.ScannedMessages, channelID, result.MatchedMessages, deckName)}
	parts = append(parts, fmt.Sprintf("Imported %s as used history.", formatCountNoun(result.ImportedQuestions, "historical QOTD question", "historical QOTD questions")))
	if result.DeletedQuestions > 0 {
		parts = append(parts, fmt.Sprintf("Removed %s duplicate questions from the current queue.", formatCountNoun(result.DeletedQuestions, "duplicate question", "duplicate questions")))
	} else if result.DuplicateQuestions > 0 {
		parts = append(parts, fmt.Sprintf("Found %s already locked in history.", formatCountNoun(result.DuplicateQuestions, "duplicate question", "duplicate questions")))
	}
	if result.StoredQuestions > 0 {
		parts = append(parts, fmt.Sprintf("Stored %s in local collector history.", formatCountNoun(result.StoredQuestions, "historical message", "historical messages")))
	}
	if backupPath := displayQuestionsImportBackupPath(result.BackupPath); backupPath != "" {
		parts = append(parts, fmt.Sprintf("Local backup: `%s`.", backupPath))
	}
	return strings.Join(parts, " ")
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
		lines = append(lines, fmt.Sprintf("Current automatic slot: %s (%s).", formatAutomaticQueueTimestamp(state.SlotPublishAtUTC), formatAutomaticQueueSlotStatus(state.SlotStatus)))
	}

	if !state.Deck.Enabled {
		lines = append(lines, "Publishing is disabled for this deck.")
	} else if strings.TrimSpace(state.Deck.ChannelID) == "" {
		lines = append(lines, "Set a QOTD channel before automatic publishing can run.")
	}

	if state.SlotQuestion != nil {
		lines = append(lines, fmt.Sprintf("Current automatic slot question: %s.", formatAutomaticQueueQuestion(*state.SlotQuestion)))
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

func formatAutomaticQueueTimestamp(value time.Time) string {
	if value.IsZero() {
		return "unavailable"
	}
	return value.UTC().Format("2006-01-02 15:04 UTC")
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

func describeResetDeckResult(result applicationqotd.ResetDeckResult, deckName string) string {
	parts := make([]string, 0, 2)
	if result.QuestionsReset > 0 {
		parts = append(parts, fmt.Sprintf("reset %s", formatCountNoun(result.QuestionsReset, "QOTD question state", "QOTD question states")))
	}
	if result.OfficialPostsCleared > 0 {
		parts = append(parts, fmt.Sprintf("cleared %s", formatCountNoun(result.OfficialPostsCleared, "QOTD publish record", "QOTD publish records")))
	}
	if len(parts) == 0 {
		message := fmt.Sprintf("No QOTD question states or publish history needed reset in deck `%s`. Question order was unchanged.", deckName)
		if result.SuppressedCurrentSlotAutomaticPublish {
			message += " Automatic publishing for the current slot is paused until you publish manually."
		}
		return message
	}
	message := fmt.Sprintf("%s in deck `%s`. Question order was preserved.", strings.Join(parts, " and "), deckName)
	if result.SuppressedCurrentSlotAutomaticPublish {
		message += " Automatic publishing for the current slot is paused until you publish manually."
	}
	return message
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

func translateQuestionsSetNextError(questionID int64, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotFound) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d was not found.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrImmutableQuestion) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d is already scheduled or used and cannot be set as next.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotReady) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d must be ready before it can be set as next.", questionID), false)
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

func translateQuestionsImportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrDiscordUnavailable) {
		return core.NewCommandError("Discord session unavailable for QOTD history import.", false)
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
var _ core.SubCommand = (*questionsNextCommand)(nil)
var _ core.SubCommand = (*questionsImportCommand)(nil)
var _ core.SubCommand = (*questionsResetCommand)(nil)
var _ core.SubCommand = (*questionsRecoverCommand)(nil)
var _ core.SubCommand = (*questionsRemoveCommand)(nil)
var _ core.SubCommand = (*qotdPublishCommand)(nil)

var _ QuestionCatalogService = (*applicationqotd.Service)(nil)
