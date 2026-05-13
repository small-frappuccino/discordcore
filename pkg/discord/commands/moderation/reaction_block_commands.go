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
	reactionBlockSetSubCommandName    = "reaction_block_set"
	reactionBlockAddSubCommandName    = "reaction_block_add"
	reactionBlockRemoveSubCommandName = "reaction_block_remove"
	reactionBlockListSubCommandName   = "reaction_block_list"
	reactionBlockClearSubCommandName  = "reaction_block_clear"

	reactionBlockReactorOptionName = "reactor"
	reactionBlockTargetOptionName  = "target"
	reactionBlockEmojisOptionName  = "emojis"
)

var reactionBlockCustomEmojiRe = regexp.MustCompile(`^<(a?):([^:\s>]+):(\d+)>$`)

type reactionBlockSetCommand struct {
	configManager *files.ConfigManager
}

type reactionBlockAddCommand struct {
	configManager *files.ConfigManager
}

type reactionBlockRemoveCommand struct {
	configManager *files.ConfigManager
}

type reactionBlockListCommand struct {
	configManager *files.ConfigManager
}

type reactionBlockClearCommand struct {
	configManager *files.ConfigManager
}

type reactionBlockRequest struct {
	reactorUserID string
	targetUserID  string
	emojis        []files.ReactionBlockEmojiConfig
}

func newReactionBlockSetCommand(configManager *files.ConfigManager) *reactionBlockSetCommand {
	return &reactionBlockSetCommand{configManager: configManager}
}

func newReactionBlockAddCommand(configManager *files.ConfigManager) *reactionBlockAddCommand {
	return &reactionBlockAddCommand{configManager: configManager}
}

func newReactionBlockRemoveCommand(configManager *files.ConfigManager) *reactionBlockRemoveCommand {
	return &reactionBlockRemoveCommand{configManager: configManager}
}

func newReactionBlockListCommand(configManager *files.ConfigManager) *reactionBlockListCommand {
	return &reactionBlockListCommand{configManager: configManager}
}

func newReactionBlockClearCommand(configManager *files.ConfigManager) *reactionBlockClearCommand {
	return &reactionBlockClearCommand{configManager: configManager}
}

func (c *reactionBlockSetCommand) Name() string { return reactionBlockSetSubCommandName }
func (c *reactionBlockAddCommand) Name() string { return reactionBlockAddSubCommandName }
func (c *reactionBlockRemoveCommand) Name() string { return reactionBlockRemoveSubCommandName }
func (c *reactionBlockListCommand) Name() string { return reactionBlockListSubCommandName }
func (c *reactionBlockClearCommand) Name() string { return reactionBlockClearSubCommandName }

func (c *reactionBlockSetCommand) Description() string {
	return "Replace the blocked reaction list for one reactor and one message author"
}

func (c *reactionBlockAddCommand) Description() string {
	return "Add blocked reaction emojis for one reactor and one message author"
}

func (c *reactionBlockRemoveCommand) Description() string {
	return "Remove blocked reaction emojis for one reactor and one message author"
}

func (c *reactionBlockListCommand) Description() string {
	return "Show blocked reaction emojis for one reactor and one message author"
}

func (c *reactionBlockClearCommand) Description() string {
	return "Clear all blocked reaction emojis for one reactor and one message author"
}

func (c *reactionBlockSetCommand) Options() []*discordgo.ApplicationCommandOption {
	return reactionBlockMutationOptions()
}

func (c *reactionBlockAddCommand) Options() []*discordgo.ApplicationCommandOption {
	return reactionBlockMutationOptions()
}

func (c *reactionBlockRemoveCommand) Options() []*discordgo.ApplicationCommandOption {
	return reactionBlockMutationOptions()
}

func (c *reactionBlockListCommand) Options() []*discordgo.ApplicationCommandOption {
	return reactionBlockPairOptions()
}

func (c *reactionBlockClearCommand) Options() []*discordgo.ApplicationCommandOption {
	return reactionBlockPairOptions()
}

func (c *reactionBlockSetCommand) RequiresGuild() bool { return true }
func (c *reactionBlockAddCommand) RequiresGuild() bool { return true }
func (c *reactionBlockRemoveCommand) RequiresGuild() bool { return true }
func (c *reactionBlockListCommand) RequiresGuild() bool { return true }
func (c *reactionBlockClearCommand) RequiresGuild() bool { return true }

func (c *reactionBlockSetCommand) RequiresPermissions() bool { return true }
func (c *reactionBlockAddCommand) RequiresPermissions() bool { return true }
func (c *reactionBlockRemoveCommand) RequiresPermissions() bool { return true }
func (c *reactionBlockListCommand) RequiresPermissions() bool { return true }
func (c *reactionBlockClearCommand) RequiresPermissions() bool { return true }

func (c *reactionBlockSetCommand) Handle(ctx *core.Context) error {
	request, err := parseReactionBlockRequest(ctx, true)
	if err != nil {
		return err
	}
	updated, err := updateReactionBlockConfig(ctx, c.configManager, func(current files.ReactionBlockConfig) (files.ReactionBlockConfig, error) {
		return setReactionBlockPair(current, request.reactorUserID, request.targetUserID, request.emojis), nil
	})
	if err != nil {
		return err
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

func (c *reactionBlockAddCommand) Handle(ctx *core.Context) error {
	request, err := parseReactionBlockRequest(ctx, true)
	if err != nil {
		return err
	}
	updated, err := updateReactionBlockConfig(ctx, c.configManager, func(current files.ReactionBlockConfig) (files.ReactionBlockConfig, error) {
		return addReactionBlockPairEmojis(current, request.reactorUserID, request.targetUserID, request.emojis), nil
	})
	if err != nil {
		return err
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

func (c *reactionBlockRemoveCommand) Handle(ctx *core.Context) error {
	request, err := parseReactionBlockRequest(ctx, true)
	if err != nil {
		return err
	}

	current, err := loadReactionBlockConfig(c.configManager, ctx.GuildID)
	if err != nil {
		return err
	}
	updated := removeReactionBlockPairEmojis(current, request.reactorUserID, request.targetUserID, request.emojis)
	if sameReactionBlockEmojiList(current.EmojisForPair(request.reactorUserID, request.targetUserID), updated.EmojisForPair(request.reactorUserID, request.targetUserID)) {
		return core.NewResponseBuilder(ctx.Session).Success(
			ctx.Interaction,
			fmt.Sprintf("No matching blocked emojis were configured from <@%s> to <@%s>.", request.reactorUserID, request.targetUserID),
		)
	}
	if err := saveReactionBlockConfig(ctx, c.configManager, updated); err != nil {
		return err
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

func (c *reactionBlockListCommand) Handle(ctx *core.Context) error {
	request, err := parseReactionBlockRequest(ctx, false)
	if err != nil {
		return err
	}
	current, err := loadReactionBlockConfig(c.configManager, ctx.GuildID)
	if err != nil {
		return err
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

func (c *reactionBlockClearCommand) Handle(ctx *core.Context) error {
	request, err := parseReactionBlockRequest(ctx, false)
	if err != nil {
		return err
	}
	current, err := loadReactionBlockConfig(c.configManager, ctx.GuildID)
	if err != nil {
		return err
	}
	updated := clearReactionBlockPair(current, request.reactorUserID, request.targetUserID)
	if sameReactionBlockEmojiList(current.EmojisForPair(request.reactorUserID, request.targetUserID), updated.EmojisForPair(request.reactorUserID, request.targetUserID)) {
		return core.NewResponseBuilder(ctx.Session).Success(
			ctx.Interaction,
			fmt.Sprintf("No blocked emojis were configured from <@%s> to <@%s>.", request.reactorUserID, request.targetUserID),
		)
	}
	if err := saveReactionBlockConfig(ctx, c.configManager, updated); err != nil {
		return err
	}
	return core.NewResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Cleared all blocked reactions from <@%s> to <@%s>.", request.reactorUserID, request.targetUserID),
	)
}

func reactionBlockPairOptions() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
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
	}
}

func reactionBlockMutationOptions() []*discordgo.ApplicationCommandOption {
	options := reactionBlockPairOptions()
	options = append(options, &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        reactionBlockEmojisOptionName,
		Description: "Blocked emojis separated by spaces or commas, e.g. <:skrunklyalice:123> :x:",
		Required:    true,
	})
	return options
}

func parseReactionBlockRequest(ctx *core.Context, requireEmojis bool) (reactionBlockRequest, error) {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return reactionBlockRequest{}, err
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

	emojis, err := parseReactionBlockEmojiList(core.NewOptionExtractor(options).String(reactionBlockEmojisOptionName))
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
		return files.ReactionBlockConfig{}, err
	}
	next, err := mutate(current)
	if err != nil {
		return files.ReactionBlockConfig{}, err
	}
	if err := saveReactionBlockConfig(ctx, configManager, next); err != nil {
		return files.ReactionBlockConfig{}, err
	}
	updated, err := loadReactionBlockConfig(configManager, ctx.GuildID)
	if err != nil {
		return files.ReactionBlockConfig{}, err
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
