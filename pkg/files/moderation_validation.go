package files

import (
	"fmt"
	"strings"
)

// GuildModerationConfig is the canonical moderation subtree persisted inside a
// guild config.
type GuildModerationConfig struct {
	Rulesets   []Ruleset
	LooseRules []Rule
	Blocklist  []string
}

// NormalizeGuildModerationConfig canonicalizes guild moderation collections and
// enforces structural validation suitable for dashboard-backed CRUD flows.
func NormalizeGuildModerationConfig(
	rulesets []Ruleset,
	looseRules []Rule,
	blocklist []string,
) (GuildModerationConfig, error) {
	normalizedRulesets := make([]Ruleset, 0, len(rulesets))
	seenRulesets := make(map[string]struct{}, len(rulesets))
	for idx, ruleset := range rulesets {
		normalized, key, err := normalizeRuleset(ruleset, fmt.Sprintf("rulesets[%d]", idx))
		if err != nil {
			return GuildModerationConfig{}, err
		}
		if _, exists := seenRulesets[key]; exists {
			return GuildModerationConfig{}, NewValidationError(
				fmt.Sprintf("rulesets[%d].id", idx),
				ruleset.ID,
				"ruleset id must be unique within the guild",
			)
		}
		seenRulesets[key] = struct{}{}
		normalizedRulesets = append(normalizedRulesets, normalized)
	}

	normalizedLooseRules, err := normalizeRuleCollection(looseRules, "loose_rules")
	if err != nil {
		return GuildModerationConfig{}, err
	}

	normalizedBlocklist := normalizeUniqueTextValues(blocklist)

	return GuildModerationConfig{
		Rulesets:   normalizedRulesets,
		LooseRules: normalizedLooseRules,
		Blocklist:  normalizedBlocklist,
	}, nil
}

func normalizeRuleset(in Ruleset, fieldBase string) (Ruleset, string, error) {
	out := Ruleset{
		ID:      sanitizeSingleLine(in.ID),
		Name:    sanitizeSingleLine(in.Name),
		Enabled: in.Enabled,
	}

	if out.ID == "" {
		return Ruleset{}, "", NewValidationError(fieldBase+".id", in.ID, "ruleset id is required")
	}
	if out.Name == "" {
		return Ruleset{}, "", NewValidationError(fieldBase+".name", in.Name, "ruleset name is required")
	}
	if len(in.Rules) == 0 {
		return Ruleset{}, "", NewValidationError(fieldBase+".rules", in.Rules, "ruleset must contain at least one rule")
	}

	rules, err := normalizeRuleCollection(in.Rules, fieldBase+".rules")
	if err != nil {
		return Ruleset{}, "", err
	}
	out.Rules = rules

	return out, strings.ToLower(out.ID), nil
}

func normalizeRuleCollection(in []Rule, fieldBase string) ([]Rule, error) {
	normalized := make([]Rule, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for idx, rule := range in {
		base := fmt.Sprintf("%s[%d]", fieldBase, idx)
		out := Rule{
			ID:      sanitizeSingleLine(rule.ID),
			Name:    sanitizeSingleLine(rule.Name),
			Enabled: rule.Enabled,
		}
		if out.ID == "" {
			return nil, NewValidationError(base+".id", rule.ID, "rule id is required")
		}
		if out.Name == "" {
			return nil, NewValidationError(base+".name", rule.Name, "rule name is required")
		}
		if len(rule.Lists) == 0 {
			return nil, NewValidationError(base+".lists", rule.Lists, "rule must contain at least one list")
		}

		key := strings.ToLower(out.ID)
		if _, exists := seen[key]; exists {
			return nil, NewValidationError(base+".id", rule.ID, "rule id must be unique within its collection")
		}
		seen[key] = struct{}{}

		lists, err := normalizeListCollection(rule.Lists, base+".lists")
		if err != nil {
			return nil, err
		}
		out.Lists = lists
		normalized = append(normalized, out)
	}
	return normalized, nil
}

func normalizeListCollection(in []List, fieldBase string) ([]List, error) {
	normalized := make([]List, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for idx, item := range in {
		base := fmt.Sprintf("%s[%d]", fieldBase, idx)
		out := List{
			ID:          sanitizeSingleLine(item.ID),
			Type:        strings.ToLower(sanitizeSingleLine(item.Type)),
			Name:        sanitizeSingleLine(item.Name),
			Description: sanitizeSingleLine(item.Description),
			NativeID:    sanitizeSingleLine(item.NativeID),
		}
		if out.ID == "" {
			return nil, NewValidationError(base+".id", item.ID, "list id is required")
		}
		if out.Name == "" {
			return nil, NewValidationError(base+".name", item.Name, "list name is required")
		}

		key := strings.ToLower(out.ID)
		if _, exists := seen[key]; exists {
			return nil, NewValidationError(base+".id", item.ID, "list id must be unique within its rule")
		}
		seen[key] = struct{}{}

		switch out.Type {
		case RuleTypeNative:
			if out.NativeID == "" {
				return nil, NewValidationError(base+".native_id", item.NativeID, "native list requires native_id")
			}
			if len(normalizeUniqueTextValues(item.BlockedKeywords)) > 0 {
				return nil, NewValidationError(base+".blocked_keywords", item.BlockedKeywords, "native list cannot include blocked_keywords")
			}
		case RuleTypeCustom:
			if out.NativeID != "" {
				return nil, NewValidationError(base+".native_id", item.NativeID, "custom list cannot include native_id")
			}
			out.BlockedKeywords = normalizeUniqueTextValues(item.BlockedKeywords)
			if len(out.BlockedKeywords) == 0 {
				return nil, NewValidationError(base+".blocked_keywords", item.BlockedKeywords, "custom list requires blocked_keywords")
			}
		default:
			return nil, NewValidationError(base+".type", item.Type, "list type must be native or custom")
		}

		normalized = append(normalized, out)
	}
	return normalized, nil
}

func normalizeUniqueTextValues(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, item := range in {
		normalized := sanitizeSingleLine(item)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
