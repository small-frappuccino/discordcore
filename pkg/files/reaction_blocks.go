package files

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ReactionBlockEmojiKindUnicode defines reaction block emoji kind unicode.
// ReactionBlockEmojiKindCustom defines reaction block emoji kind custom.
const (
	ReactionBlockEmojiKindCustom  = "custom"
	ReactionBlockEmojiKindUnicode = "unicode"
)

// ErrInvalidReactionBlockInput defines err invalid reaction block input.
var ErrInvalidReactionBlockInput = errors.New("invalid reaction block input")

// CloneReactionBlockConfig clones reaction block config.
func CloneReactionBlockConfig(in ReactionBlockConfig) ReactionBlockConfig {
	return cloneReactionBlockConfig(in)
}

// IsZero is zero.
func (cfg ReactionBlockConfig) IsZero() bool {
	return len(cfg.Rules) == 0
}

// IsZero is zero.
func (rule ReactionBlockRuleConfig) IsZero() bool {
	return strings.TrimSpace(rule.ReactorUserID) == "" && strings.TrimSpace(rule.TargetUserID) == "" && len(rule.Emojis) == 0
}

// IsZero is zero.
func (emoji ReactionBlockEmojiConfig) IsZero() bool {
	return reactionBlockEmojiKey(emoji) == ""
}

// Display displays.
func (emoji ReactionBlockEmojiConfig) Display() string {
	switch reactionBlockEmojiKind(emoji.Kind) {
	case ReactionBlockEmojiKindCustom:
		name := strings.TrimSpace(emoji.Name)
		if name == "" {
			name = "emoji"
		}
		prefix := ":"
		if emoji.Animated {
			prefix = "a:"
		}
		if value := strings.TrimSpace(emoji.Value); value != "" {
			return "<" + prefix + name + ":" + value + ">"
		}
	case ReactionBlockEmojiKindUnicode:
		if alias := normalizeReactionBlockAlias(emoji.Alias); alias != "" {
			return alias
		}
		if value := strings.TrimSpace(emoji.Value); value != "" {
			return value
		}
	}
	return ""
}

// EmojisForPair emojis for pair.
func (cfg ReactionBlockConfig) EmojisForPair(reactorUserID, targetUserID string) []ReactionBlockEmojiConfig {
	reactorUserID = strings.TrimSpace(reactorUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	if reactorUserID == "" || targetUserID == "" {
		return nil
	}
	for _, rule := range cfg.Rules {
		if strings.TrimSpace(rule.ReactorUserID) != reactorUserID || strings.TrimSpace(rule.TargetUserID) != targetUserID {
			continue
		}
		if len(rule.Emojis) == 0 {
			return nil
		}
		out := make([]ReactionBlockEmojiConfig, 0, len(rule.Emojis))
		for _, emoji := range rule.Emojis {
			if emoji.IsZero() {
				continue
			}
			out = append(out, emoji)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

// BlocksEmojiForPair blocks emoji for pair.
func (cfg ReactionBlockConfig) BlocksEmojiForPair(reactorUserID, targetUserID string, emoji ReactionBlockEmojiConfig) bool {
	key := reactionBlockEmojiKey(emoji)
	if key == "" {
		return false
	}
	for _, candidate := range cfg.EmojisForPair(reactorUserID, targetUserID) {
		if reactionBlockEmojiKey(candidate) == key {
			return true
		}
	}
	return false
}

// NormalizeReactionBlockConfig normalizes reaction block config.
func NormalizeReactionBlockConfig(in ReactionBlockConfig) (ReactionBlockConfig, error) {
	if len(in.Rules) == 0 {
		return ReactionBlockConfig{}, nil
	}

	out := make([]ReactionBlockRuleConfig, 0, len(in.Rules))
	indexByPair := make(map[string]int, len(in.Rules))
	for idx, rule := range in.Rules {
		normalized, err := normalizeReactionBlockRuleConfig(rule)
		if err != nil {
			return ReactionBlockConfig{}, invalidReactionBlockInput("rules[%d]: %v", idx, err)
		}
		pairKey := reactionBlockPairKey(normalized.ReactorUserID, normalized.TargetUserID)
		if existingIdx, ok := indexByPair[pairKey]; ok {
			merged := append(cloneReactionBlockRuleConfig(out[existingIdx]).Emojis, normalized.Emojis...)
			normalizedEmojis, err := normalizeReactionBlockEmojiConfigs(merged)
			if err != nil {
				return ReactionBlockConfig{}, invalidReactionBlockInput("rules[%d]: %v", idx, err)
			}
			out[existingIdx].Emojis = normalizedEmojis
			continue
		}
		indexByPair[pairKey] = len(out)
		out = append(out, normalized)
	}

	if len(out) == 0 {
		return ReactionBlockConfig{}, nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReactorUserID != out[j].ReactorUserID {
			return out[i].ReactorUserID < out[j].ReactorUserID
		}
		return out[i].TargetUserID < out[j].TargetUserID
	})
	return ReactionBlockConfig{Rules: out}, nil
}

// ReactionBlockConfig reactions block config.
func (mgr *ConfigManager) ReactionBlockConfig(guildID string) (_ ReactionBlockConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get reaction block config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return ReactionBlockConfig{}, invalidReactionBlockInput("guild_id is required")
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return ReactionBlockConfig{}, fmt.Errorf("ConfigManager.ReactionBlockConfig: %w", err)
	}

	normalized, err := NormalizeReactionBlockConfig(guildConfig.ReactionBlocks)
	if err != nil {
		return ReactionBlockConfig{}, fmt.Errorf("ConfigManager.ReactionBlockConfig: %w", err)
	}
	return normalized, nil
}

// GetReactionBlockConfig gets reaction block config.
func (mgr *ConfigManager) GetReactionBlockConfig(guildID string) (ReactionBlockConfig, error) {
	return mgr.ReactionBlockConfig(guildID)
}

// SetReactionBlockConfig sets reaction block config.
func (mgr *ConfigManager) SetReactionBlockConfig(guildID string, cfg ReactionBlockConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("set reaction block config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidReactionBlockInput("guild_id is required")
	}

	normalized, err := NormalizeReactionBlockConfig(cfg)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetReactionBlockConfig: %w", err)
	}
	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		guildConfig.ReactionBlocks = normalized
		return nil
	})
}

func invalidReactionBlockInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidReactionBlockInput, fmt.Sprintf(format, args...))
}

func normalizeReactionBlockRuleConfig(in ReactionBlockRuleConfig) (ReactionBlockRuleConfig, error) {
	reactorUserID := strings.TrimSpace(in.ReactorUserID)
	if reactorUserID == "" {
		return ReactionBlockRuleConfig{}, fmt.Errorf("reactor_user_id is required")
	}
	if !isAllDigits(reactorUserID) {
		return ReactionBlockRuleConfig{}, fmt.Errorf("reactor_user_id must be numeric")
	}

	targetUserID := strings.TrimSpace(in.TargetUserID)
	if targetUserID == "" {
		return ReactionBlockRuleConfig{}, fmt.Errorf("target_user_id is required")
	}
	if !isAllDigits(targetUserID) {
		return ReactionBlockRuleConfig{}, fmt.Errorf("target_user_id must be numeric")
	}

	emojis, err := normalizeReactionBlockEmojiConfigs(in.Emojis)
	if err != nil {
		return ReactionBlockRuleConfig{}, fmt.Errorf("normalizeReactionBlockRuleConfig: %w", err)
	}
	if len(emojis) == 0 {
		return ReactionBlockRuleConfig{}, fmt.Errorf("emojis must contain at least one blocked emoji")
	}

	return ReactionBlockRuleConfig{
		ReactorUserID: reactorUserID,
		TargetUserID:  targetUserID,
		Emojis:        emojis,
	}, nil
}

func normalizeReactionBlockEmojiConfigs(in []ReactionBlockEmojiConfig) ([]ReactionBlockEmojiConfig, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]ReactionBlockEmojiConfig, 0, len(in))
	indexByKey := make(map[string]int, len(in))
	for idx, emoji := range in {
		normalized, err := normalizeReactionBlockEmojiConfig(emoji)
		if err != nil {
			return nil, fmt.Errorf("emojis[%d]: %w", idx, err)
		}
		key := reactionBlockEmojiKey(normalized)
		if existingIdx, ok := indexByKey[key]; ok {
			out[existingIdx] = mergeReactionBlockEmojiConfig(out[existingIdx], normalized)
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, nil
	}
	sort.Slice(out, func(i, j int) bool {
		return reactionBlockEmojiKey(out[i]) < reactionBlockEmojiKey(out[j])
	})
	return out, nil
}

func normalizeReactionBlockEmojiConfig(in ReactionBlockEmojiConfig) (ReactionBlockEmojiConfig, error) {
	kind := reactionBlockEmojiKind(in.Kind)
	value := strings.TrimSpace(in.Value)
	name := strings.TrimSpace(in.Name)
	alias := normalizeReactionBlockAlias(in.Alias)

	switch kind {
	case ReactionBlockEmojiKindCustom:
		if value == "" {
			return ReactionBlockEmojiConfig{}, fmt.Errorf("custom emoji value is required")
		}
		if !isAllDigits(value) {
			return ReactionBlockEmojiConfig{}, fmt.Errorf("custom emoji value must be numeric")
		}
		return ReactionBlockEmojiConfig{
			Kind:     kind,
			Value:    value,
			Name:     name,
			Animated: in.Animated,
		}, nil
	case ReactionBlockEmojiKindUnicode:
		if value == "" {
			return ReactionBlockEmojiConfig{}, fmt.Errorf("unicode emoji value is required")
		}
		return ReactionBlockEmojiConfig{
			Kind:  kind,
			Value: value,
			Alias: alias,
		}, nil
	default:
		return ReactionBlockEmojiConfig{}, fmt.Errorf("emoji kind must be %q or %q", ReactionBlockEmojiKindCustom, ReactionBlockEmojiKindUnicode)
	}
}

func normalizeReactionBlockAlias(alias string) string {
	alias = strings.ToLower(strings.TrimSpace(alias))
	if alias == "" {
		return ""
	}
	if strings.Count(alias, ":") < 2 || !strings.HasPrefix(alias, ":") || !strings.HasSuffix(alias, ":") {
		return ""
	}
	return alias
}

func reactionBlockEmojiKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case ReactionBlockEmojiKindCustom:
		return ReactionBlockEmojiKindCustom
	case ReactionBlockEmojiKindUnicode:
		return ReactionBlockEmojiKindUnicode
	default:
		return ""
	}
}

func reactionBlockPairKey(reactorUserID, targetUserID string) string {
	reactorUserID = strings.TrimSpace(reactorUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	if reactorUserID == "" || targetUserID == "" {
		return ""
	}
	return reactorUserID + ":" + targetUserID
}

func reactionBlockEmojiKey(emoji ReactionBlockEmojiConfig) string {
	kind := reactionBlockEmojiKind(emoji.Kind)
	value := strings.TrimSpace(emoji.Value)
	if kind == "" || value == "" {
		return ""
	}
	return kind + ":" + value
}

func mergeReactionBlockEmojiConfig(current, incoming ReactionBlockEmojiConfig) ReactionBlockEmojiConfig {
	if current.Name == "" {
		current.Name = incoming.Name
	}
	if current.Alias == "" {
		current.Alias = incoming.Alias
	}
	current.Animated = current.Animated || incoming.Animated
	return current
}

func cloneReactionBlockRuleConfig(in ReactionBlockRuleConfig) ReactionBlockRuleConfig {
	out := ReactionBlockRuleConfig{
		ReactorUserID: in.ReactorUserID,
		TargetUserID:  in.TargetUserID,
	}
	if len(in.Emojis) > 0 {
		out.Emojis = make([]ReactionBlockEmojiConfig, 0, len(in.Emojis))
		out.Emojis = append(out.Emojis, in.Emojis...)
	}
	return out
}
