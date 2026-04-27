package qotd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const (
	groupName                 = "qotd"
	questionsGroupName        = "questions"
	questionsListSubCommand   = "list"
	questionsDeckOptionName   = "deck"
	questionsPageSize         = 10
	questionsListRouteFirst   = "qotd:questions:list:first"
	questionsListRoutePrev    = "qotd:questions:list:prev"
	questionsListRouteNext    = "qotd:questions:list:next"
	questionsListRouteLast    = "qotd:questions:list:last"
	questionsListDeniedText   = "Only the user who opened this list can change pages."
	questionsListUnknownDeck  = "QOTD deck not found"
	questionsListMissingGuild = "This command can only be used in a server"
)

type QuestionCatalogService interface {
	Settings(guildID string) (files.QOTDConfig, error)
	ListQuestions(ctx context.Context, guildID, deckID string) ([]storage.QOTDQuestionRecord, error)
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
	listCommand := &questionsListCommand{service: c.service}
	questionsGroup := core.NewGroupCommand(questionsGroupName, "Browse QOTD deck questions", checker)
	questionsGroup.AddSubCommand(listCommand)

	var group *core.GroupCommand
	if existing, ok := router.GetRegistry().GetCommand(groupName); ok {
		if existingGroup, ok := existing.(*core.GroupCommand); ok {
			group = existingGroup
		}
	}
	if group == nil {
		group = core.NewGroupCommand(groupName, "Manage QOTD decks and questions", checker)
	}
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
	service QuestionCatalogService
}

type questionsListState struct {
	UserID string
	DeckID string
	Page   int
}

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

func (c *questionsListCommand) Handle(ctx *core.Context) error {
	if ctx == nil || ctx.Interaction == nil {
		return nil
	}
	if strings.TrimSpace(ctx.GuildID) == "" {
		return core.NewCommandError(questionsListMissingGuild, true)
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
	return respondQuestionsList(ctx, view, state, true, questionsListRouteFirst)
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
	return respondQuestionsList(ctx, view, state, false, action)
}

type questionsListView struct {
	deck      files.QOTDDeckConfig
	questions []storage.QOTDQuestionRecord
}

func (c *questionsListCommand) loadView(ctx *core.Context, requestedDeck string) (questionsListView, error) {
	settings, err := c.service.Settings(ctx.GuildID)
	if err != nil {
		return questionsListView{}, err
	}
	deck, err := resolveDeck(settings, requestedDeck)
	if err != nil {
		return questionsListView{}, err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return questionsListView{}, err
	}
	return questionsListView{deck: deck, questions: questions}, nil
}

func resolveDeck(settings files.QOTDConfig, requestedDeck string) (files.QOTDDeckConfig, error) {
	settings = files.DashboardQOTDConfig(settings)
	requestedDeck = strings.TrimSpace(requestedDeck)
	if requestedDeck == "" {
		if deck, ok := settings.ActiveDeck(); ok {
			return deck, nil
		}
		return files.QOTDDeckConfig{}, core.NewCommandError(questionsListUnknownDeck, true)
	}

	if deck, ok := settings.DeckByID(requestedDeck); ok {
		return deck, nil
	}
	for _, deck := range settings.Decks {
		if strings.EqualFold(strings.TrimSpace(deck.Name), requestedDeck) {
			return deck, nil
		}
	}
	return files.QOTDDeckConfig{}, core.NewCommandError(fmt.Sprintf("%s: %s", questionsListUnknownDeck, requestedDeck), true)
}

func respondQuestionsList(
	ctx *core.Context,
	view questionsListView,
	state questionsListState,
	ephemeral bool,
	action string,
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
	return sendQuestionsListResponse(ctx, embed, components, ephemeral, action == questionsListRouteFirst)
}

func buildQuestionsListComponents(state questionsListState, totalPages int) []discordgo.MessageComponent {
	page := normalizeQuestionsListPage(state.Page, totalPages)
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: encodeQuestionsListState(questionsListRouteFirst, state.withPage(page)), Label: "<<", Style: discordgo.SecondaryButton, Disabled: page == 0 || totalPages <= 1},
			discordgo.Button{CustomID: encodeQuestionsListState(questionsListRoutePrev, state.withPage(page)), Label: "<", Style: discordgo.SecondaryButton, Disabled: page == 0 || totalPages <= 1},
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
		return 0
	case questionsListRoutePrev:
		return normalizeQuestionsListPage(currentPage-1, totalPages)
	case questionsListRouteNext:
		return normalizeQuestionsListPage(currentPage+1, totalPages)
	case questionsListRouteLast:
		return normalizeQuestionsListPage(totalPages-1, totalPages)
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

var _ core.SubCommand = (*questionsListCommand)(nil)

var _ QuestionCatalogService = (*applicationqotd.Service)(nil)