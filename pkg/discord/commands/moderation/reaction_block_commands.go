package moderation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	emoji "github.com/kyokomi/emoji/v2"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	reactionBlockCommandName = "reaction_block"

	reactionBlockActionOptionName  = "action"
	reactionBlockReactorOptionName = "reactor"
	reactionBlockTargetOptionName  = "target"
	reactionBlockEmojisOptionName  = "emojis"

	reactionBlockActionSet    = "set"
	reactionBlockActionAdd    = "add"
	reactionBlockActionRemove = "remove"
	reactionBlockActionList   = "list"
	reactionBlockActionClear  = "clear"
)

var reactionBlockCustomEmojiRe = regexp.MustCompile(`^<(a?):([^:\s>]+):(\d+)>$`)

// reactionBlockCommand consolidates the reaction-block CRUD; the "action" choice is the discriminator so future CRUDs reuse this template instead of regrowing one top-level command per verb.
type reactionBlockCommand struct {
	configManager *files.ConfigManager
}

type reactionBlockRequest struct {
	reactorUserID string
	targetUserID  string
	emojis        []files.ReactionBlockEmojiConfig
}

func newReactionBlockCommand(configManager *files.ConfigManager) *reactionBlockCommand {
	return &reactionBlockCommand{configManager: configManager}
}

// Name names.
func (c *reactionBlockCommand) Name() string { return reactionBlockCommandName }

// Description descriptions.
func (c *reactionBlockCommand) Description() string {
	return "Manage blocked reaction emojis for a reactor/target pair"
}

// Options options.
func (c *reactionBlockCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        reactionBlockActionOptionName,
			Description: "CRUD action to perform on the blocked-reaction list",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "set", Value: reactionBlockActionSet},
				{Name: "add", Value: reactionBlockActionAdd},
				{Name: "remove", Value: reactionBlockActionRemove},
				{Name: "list", Value: reactionBlockActionList},
				{Name: "clear", Value: reactionBlockActionClear},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        reactionBlockReactorOptionName,
			Description: "User who adds the reaction",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        reactionBlockTargetOptionName,
			Description: "User whose messages are being reacted to",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        reactionBlockEmojisOptionName,
			Description: "Blocked emojis separated by spaces or commas (required for set, add, remove)",
			Required:    false,
		},
	}
}

// RequiresGuild requires guild.
func (c *reactionBlockCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *reactionBlockCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *reactionBlockCommand) DefaultMemberPermissions() int64 {
	return discordgo.PermissionManageMessages
}

// Handle handles.
func (c *reactionBlockCommand) Handle(ctx *core.Context) error {
	action, err := parseReactionBlockAction(ctx)
	if err != nil {
		return fmt.Errorf("reactionBlockCommand.Handle: %w", err)
	}
	request, err := parseReactionBlockRequest(ctx, reactionBlockActionRequiresEmojis(action))
	if err != nil {
		return fmt.Errorf("reactionBlockCommand.Handle: %w", err)
	}
	switch action {
	case reactionBlockActionSet:
		return c.handleSet(ctx, request)
	case reactionBlockActionAdd:
		return c.handleAdd(ctx, request)
	case reactionBlockActionRemove:
		return c.handleRemove(ctx, request)
	case reactionBlockActionList:
		return c.handleList(ctx, request)
	case reactionBlockActionClear:
		return c.handleClear(ctx, request)
	default:
		return core.NewCommandError("Unknown reaction_block action.", true)
	}
}

func (c *reactionBlockCommand) handleSet(ctx *core.Context, request reactionBlockRequest) error {
	updated, err := updateReactionBlockConfig(ctx, c.configManager, func(current files.ReactionBlockConfig) (files.ReactionBlockConfig, error) {
		return setReactionBlockPair(current, request.reactorUserID, request.targetUserID, request.emojis), nil
	})
	if err != nil {
		return fmt.Errorf("reactionBlockCommand.handleSet: %w", err)
	}
	return core.NewResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf(
			"Blocked reactions from <@%s> to <@%s> are now: %s.",
			request.reactorUserID,
			request.targetUserID,
			formatReactionBlockEmojiList(updated.EmojisForPair(request.reactorUserID, request.targetUserID)),
		),
	)
}

func (c *reactionBlockCommand) handleAdd(ctx *core.Context, request reactionBlockRequest) error {
	updated, err := updateReactionBlockConfig(ctx, c.configManager, func(current files.ReactionBlockConfig) (files.ReactionBlockConfig, error) {
		return addReactionBlockPairEmojis(current, request.reactorUserID, request.targetUserID, request.emojis), nil
	})
	if err != nil {
		return fmt.Errorf("reactionBlockCommand.handleAdd: %w", err)
	}
	return core.NewResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf(
			"Blocked reactions from <@%s> to <@%s> now include: %s.",
			request.reactorUserID,
			request.targetUserID,
			formatReactionBlockEmojiList(updated.EmojisForPair(request.reactorUserID, request.targetUserID)),
		),
	)
}

func (c *reactionBlockCommand) handleRemove(ctx *core.Context, request reactionBlockRequest) error {
	current, err := loadReactionBlockConfig(c.configManager, ctx.GuildID)
	if err != nil {
		return fmt.Errorf("reactionBlockCommand.handleRemove: %w", err)
	}
	updated := removeReactionBlockPairEmojis(current, request.reactorUserID, request.targetUserID, request.emojis)
	if sameReactionBlockEmojiList(current.EmojisForPair(request.reactorUserID, request.targetUserID), updated.EmojisForPair(request.reactorUserID, request.targetUserID)) {
		return core.NewResponseBuilder(ctx.Session).Success(
			ctx.Interaction,
			fmt.Sprintf("No matching blocked emojis were configured from <@%s> to <@%s>.", request.reactorUserID, request.targetUserID),
		)
	}
	if err := saveReactionBlockConfig(ctx, c.configManager, updated); err != nil {
		return fmt.Errorf("reactionBlockCommand.handleRemove: %w", err)
	}
	remaining := updated.EmojisForPair(request.reactorUserID, request.targetUserID)
	if len(remaining) == 0 {
		return core.NewResponseBuilder(ctx.Session).Success(
			ctx.Interaction,
			fmt.Sprintf("Blocked reactions from <@%s> to <@%s> were cleared.", request.reactorUserID, request.targetUserID),
		)
	}
	return core.NewResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf(
			"Blocked reactions from <@%s> to <@%s> now include: %s.",
			request.reactorUserID,
			request.targetUserID,
			formatReactionBlockEmojiList(remaining),
		),
	)
}

func (c *reactionBlockCommand) handleList(ctx *core.Context, request reactionBlockRequest) error {
	current, err := loadReactionBlockConfig(c.configManager, ctx.GuildID)
	if err != nil {
		return fmt.Errorf("reactionBlockCommand.handleList: %w", err)
	}
	emojis := current.EmojisForPair(request.reactorUserID, request.targetUserID)
	if len(emojis) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(
			ctx.Interaction,
			fmt.Sprintf("No blocked emojis are configured from <@%s> to <@%s>.", request.reactorUserID, request.targetUserID),
		)
	}
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(
		ctx.Interaction,
		fmt.Sprintf(
			"Blocked reactions from <@%s> to <@%s>: %s.",
			request.reactorUserID,
			request.targetUserID,
			formatReactionBlockEmojiList(emojis),
		),
	)
}

func (c *reactionBlockCommand) handleClear(ctx *core.Context, request reactionBlockRequest) error {
	current, err := loadReactionBlockConfig(c.configManager, ctx.GuildID)
	if err != nil {
		return fmt.Errorf("reactionBlockCommand.handleClear: %w", err)
	}
	updated := clearReactionBlockPair(current, request.reactorUserID, request.targetUserID)
	if sameReactionBlockEmojiList(current.EmojisForPair(request.reactorUserID, request.targetUserID), updated.EmojisForPair(request.reactorUserID, request.targetUserID)) {
		return core.NewResponseBuilder(ctx.Session).Success(
			ctx.Interaction,
			fmt.Sprintf("No blocked emojis were configured from <@%s> to <@%s>.", request.reactorUserID, request.targetUserID),
		)
	}
	if err := saveReactionBlockConfig(ctx, c.configManager, updated); err != nil {
		return fmt.Errorf("reactionBlockCommand.handleClear: %w", err)
	}
	return core.NewResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Cleared all blocked reactions from <@%s> to <@%s>.", request.reactorUserID, request.targetUserID),
	)
}

func parseReactionBlockAction(ctx *core.Context) (string, error) {
	options := core.GetSubCommandOptions(ctx.Interaction)
	action := strings.ToLower(strings.TrimSpace(core.OptionList(options).String(reactionBlockActionOptionName)))
	if action == "" {
		return "", core.NewCommandError("An action is required.", true)
	}
	return action, nil
}

func reactionBlockActionRequiresEmojis(action string) bool {
	switch action {
	case reactionBlockActionSet, reactionBlockActionAdd, reactionBlockActionRemove:
		return true
	default:
		return false
	}
}

func parseReactionBlockRequest(ctx *core.Context, requireEmojis bool) (reactionBlockRequest, error) {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return reactionBlockRequest{}, fmt.Errorf("parseReactionBlockRequest: %w", err)
	}
	options := core.GetSubCommandOptions(ctx.Interaction)
	reactorUserID := userOptionID(options, reactionBlockReactorOptionName)
	if reactorUserID == "" {
		return reactionBlockRequest{}, core.NewCommandError("A reactor user is required.", true)
	}
	targetUserID := userOptionID(options, reactionBlockTargetOptionName)
	if targetUserID == "" {
		return reactionBlockRequest{}, core.NewCommandError("A target user is required.", true)
	}
	request := reactionBlockRequest{
		reactorUserID: reactorUserID,
		targetUserID:  targetUserID,
	}
	if !requireEmojis {
		return request, nil
	}

	emojis, err := parseReactionBlockEmojiList(core.OptionList(options).String(reactionBlockEmojisOptionName))
	if err != nil {
		return reactionBlockRequest{}, core.NewCommandError(err.Error(), true)
	}
	request.emojis = emojis
	return request, nil
}

func parseReactionBlockEmojiList(input string) ([]files.ReactionBlockEmojiConfig, error) {
	tokens := splitReactionBlockEmojiTokens(input)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("at least one emoji is required")
	}

	emojis := make([]files.ReactionBlockEmojiConfig, 0, len(tokens))
	invalid := make([]string, 0)
	for _, token := range tokens {
		emojiConfig, err := parseReactionBlockEmojiToken(token)
		if err != nil {
			invalid = append(invalid, token)
			continue
		}
		emojis = append(emojis, emojiConfig)
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf(
			"invalid emoji list. Use custom emojis like <:name:id> or built-in shortcodes like :x:. Invalid: %s",
			strings.Join(invalid, ", "),
		)
	}
	return emojis, nil
}

func splitReactionBlockEmojiTokens(input string) []string {
	fields := strings.FieldsFunc(strings.TrimSpace(input), func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		out = append(out, field)
	}
	return out
}

func parseReactionBlockEmojiToken(token string) (files.ReactionBlockEmojiConfig, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return files.ReactionBlockEmojiConfig{}, fmt.Errorf("emoji token is empty")
	}
	if match := reactionBlockCustomEmojiRe.FindStringSubmatch(token); len(match) == 4 {
		return files.ReactionBlockEmojiConfig{
			Kind:     files.ReactionBlockEmojiKindCustom,
			Value:    match[3],
			Name:     match[2],
			Animated: match[1] == "a",
		}, nil
	}
	if isReactionBlockShortcodeToken(token) {
		alias := strings.ToLower(token)
		rendered := emoji.Sprint(alias)
		if rendered == alias || strings.TrimSpace(rendered) == "" {
			return files.ReactionBlockEmojiConfig{}, fmt.Errorf("unknown emoji shortcode")
		}
		return files.ReactionBlockEmojiConfig{
			Kind:  files.ReactionBlockEmojiKindUnicode,
			Value: rendered,
			Alias: alias,
		}, nil
	}
	if containsNonASCII(token) {
		return files.ReactionBlockEmojiConfig{
			Kind:  files.ReactionBlockEmojiKindUnicode,
			Value: token,
		}, nil
	}
	return files.ReactionBlockEmojiConfig{}, fmt.Errorf("unsupported emoji token")
}

func isReactionBlockShortcodeToken(token string) bool {
	token = strings.TrimSpace(token)
	return strings.Count(token, ":") >= 2 && strings.HasPrefix(token, ":") && strings.HasSuffix(token, ":")
}

func containsNonASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return true
		}
	}
	return false
}

func updateReactionBlockConfig(
	ctx *core.Context,
	configManager *files.ConfigManager,
	mutate func(files.ReactionBlockConfig) (files.ReactionBlockConfig, error),
) (files.ReactionBlockConfig, error) {
	current, err := loadReactionBlockConfig(configManager, ctx.GuildID)
	if err != nil {
		return files.ReactionBlockConfig{}, fmt.Errorf("updateReactionBlockConfig: %w", err)
	}
	next, err := mutate(current)
	if err != nil {
		return files.ReactionBlockConfig{}, fmt.Errorf("updateReactionBlockConfig: %w", err)
	}
	if err := saveReactionBlockConfig(ctx, configManager, next); err != nil {
		return files.ReactionBlockConfig{}, fmt.Errorf("updateReactionBlockConfig: %w", err)
	}
	updated, err := loadReactionBlockConfig(configManager, ctx.GuildID)
	if err != nil {
		return files.ReactionBlockConfig{}, fmt.Errorf("updateReactionBlockConfig: %w", err)
	}
	return updated, nil
}

func loadReactionBlockConfig(configManager *files.ConfigManager, guildID string) (files.ReactionBlockConfig, error) {
	if configManager == nil {
		return files.ReactionBlockConfig{}, core.NewCommandError("Configuration is not available right now.", true)
	}
	current, err := configManager.ReactionBlockConfig(guildID)
	if err != nil {
		return files.ReactionBlockConfig{}, core.NewCommandError("The reaction block list for this server couldn't be loaded. This reply stays private so it can be adjusted and retried without extra channel noise.", true)
	}
	return current, nil
}

func saveReactionBlockConfig(ctx *core.Context, configManager *files.ConfigManager, cfg files.ReactionBlockConfig) error {
	if configManager == nil {
		return core.NewCommandError("Configuration is not available right now.", true)
	}
	if err := configManager.SetReactionBlockConfig(ctx.GuildID, cfg); err != nil {
		ctx.Logger.Error().Errorf("Failed to save reaction block config: %v", err)
		return core.NewCommandError("That reaction block change couldn't be saved. This reply stays private so it can be adjusted and retried without extra channel noise.", true)
	}
	if ctx.GuildConfig != nil {
		updated, err := configManager.ReactionBlockConfig(ctx.GuildID)
		if err == nil {
			ctx.GuildConfig.ReactionBlocks = updated
		}
	}
	return nil
}

func setReactionBlockPair(
	cfg files.ReactionBlockConfig,
	reactorUserID, targetUserID string,
	emojis []files.ReactionBlockEmojiConfig,
) files.ReactionBlockConfig {
	next := files.CloneReactionBlockConfig(cfg)
	replaced := false
	for idx := range next.Rules {
		if reactionBlockPairMatches(next.Rules[idx], reactorUserID, targetUserID) {
			next.Rules[idx].Emojis = cloneReactionBlockEmojiConfigs(emojis)
			replaced = true
			break
		}
	}
	if !replaced {
		next.Rules = append(next.Rules, files.ReactionBlockRuleConfig{
			ReactorUserID: strings.TrimSpace(reactorUserID),
			TargetUserID:  strings.TrimSpace(targetUserID),
			Emojis:        cloneReactionBlockEmojiConfigs(emojis),
		})
	}
	return next
}

func addReactionBlockPairEmojis(
	cfg files.ReactionBlockConfig,
	reactorUserID, targetUserID string,
	emojis []files.ReactionBlockEmojiConfig,
) files.ReactionBlockConfig {
	next := files.CloneReactionBlockConfig(cfg)
	for idx := range next.Rules {
		if reactionBlockPairMatches(next.Rules[idx], reactorUserID, targetUserID) {
			next.Rules[idx].Emojis = append(next.Rules[idx].Emojis, cloneReactionBlockEmojiConfigs(emojis)...)
			return next
		}
	}
	next.Rules = append(next.Rules, files.ReactionBlockRuleConfig{
		ReactorUserID: strings.TrimSpace(reactorUserID),
		TargetUserID:  strings.TrimSpace(targetUserID),
		Emojis:        cloneReactionBlockEmojiConfigs(emojis),
	})
	return next
}

func removeReactionBlockPairEmojis(
	cfg files.ReactionBlockConfig,
	reactorUserID, targetUserID string,
	emojis []files.ReactionBlockEmojiConfig,
) files.ReactionBlockConfig {
	next := files.CloneReactionBlockConfig(cfg)
	removeKeys := make(map[string]struct{}, len(emojis))
	for _, emojiConfig := range emojis {
		if key := reactionBlockEmojiUpdateKey(emojiConfig); key != "" {
			removeKeys[key] = struct{}{}
		}
	}
	filteredRules := next.Rules[:0]
	for _, rule := range next.Rules {
		if !reactionBlockPairMatches(rule, reactorUserID, targetUserID) {
			filteredRules = append(filteredRules, rule)
			continue
		}
		filteredEmojis := rule.Emojis[:0]
		for _, candidate := range rule.Emojis {
			if _, remove := removeKeys[reactionBlockEmojiUpdateKey(candidate)]; remove {
				continue
			}
			filteredEmojis = append(filteredEmojis, candidate)
		}
		if len(filteredEmojis) == 0 {
			continue
		}
		rule.Emojis = filteredEmojis
		filteredRules = append(filteredRules, rule)
	}
	next.Rules = filteredRules
	return next
}

func clearReactionBlockPair(cfg files.ReactionBlockConfig, reactorUserID, targetUserID string) files.ReactionBlockConfig {
	next := files.CloneReactionBlockConfig(cfg)
	filtered := next.Rules[:0]
	for _, rule := range next.Rules {
		if reactionBlockPairMatches(rule, reactorUserID, targetUserID) {
			continue
		}
		filtered = append(filtered, rule)
	}
	next.Rules = filtered
	return next
}

func reactionBlockPairMatches(rule files.ReactionBlockRuleConfig, reactorUserID, targetUserID string) bool {
	return strings.TrimSpace(rule.ReactorUserID) == strings.TrimSpace(reactorUserID) && strings.TrimSpace(rule.TargetUserID) == strings.TrimSpace(targetUserID)
}

func reactionBlockEmojiUpdateKey(emoji files.ReactionBlockEmojiConfig) string {
	kind := strings.TrimSpace(strings.ToLower(emoji.Kind))
	value := strings.TrimSpace(emoji.Value)
	if kind == "" || value == "" {
		return ""
	}
	return kind + ":" + value
}

func cloneReactionBlockEmojiConfigs(in []files.ReactionBlockEmojiConfig) []files.ReactionBlockEmojiConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]files.ReactionBlockEmojiConfig, 0, len(in))
	out = append(out, in...)
	return out
}

func sameReactionBlockEmojiList(left, right []files.ReactionBlockEmojiConfig) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if reactionBlockEmojiUpdateKey(left[idx]) != reactionBlockEmojiUpdateKey(right[idx]) {
			return false
		}
	}
	return true
}

func formatReactionBlockEmojiList(emojis []files.ReactionBlockEmojiConfig) string {
	if len(emojis) == 0 {
		return "none"
	}
	labels := make([]string, 0, len(emojis))
	for _, emojiConfig := range emojis {
		label := emojiConfig.Display()
		if label == "" {
			continue
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return "none"
	}
	sort.Strings(labels)
	return strings.Join(labels, ", ")
}

func userOptionID(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option == nil || option.Name != name {
			continue
		}
		if value, ok := option.Value.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
