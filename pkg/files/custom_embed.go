package files

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	// ErrCustomEmbedNotFound indicates no custom embed matched the requested key.
	ErrCustomEmbedNotFound = errors.New("custom embed not found")
	// ErrCustomEmbedPostingNotFound indicates no posting matched the requested message ID.
	ErrCustomEmbedPostingNotFound = errors.New("custom embed posting not found")
	// ErrInvalidCustomEmbedInput indicates invalid custom embed input payload.
	ErrInvalidCustomEmbedInput = errors.New("invalid custom embed input")
)

// CustomEmbedTitleMaxLen defines custom embed title max len.
// CustomEmbedDescriptionMaxLen defines custom embed description max len.
// CustomEmbedColorMax defines custom embed color max.
// CustomEmbedAuthorMaxLen defines custom embed author max len.
// CustomEmbedFooterMaxLen defines custom embed footer max len.
// CustomEmbedFieldNameMaxLen defines custom embed field name max len.
// CustomEmbedFieldValueMaxLen defines custom embed field value max len.
// CustomEmbedMaxFields defines custom embed max fields.
// CustomEmbedMaxTotalLen defines custom embed max total len.
// CustomEmbedKeyMaxLen defines custom embed key max len.
const (
	CustomEmbedKeyMaxLen         = 32
	CustomEmbedTitleMaxLen       = 256
	CustomEmbedDescriptionMaxLen = 4000
	CustomEmbedColorMax          = 0xFFFFFF
	CustomEmbedAuthorMaxLen      = 256
	CustomEmbedFooterMaxLen      = 2048
	CustomEmbedFieldNameMaxLen   = 256
	CustomEmbedFieldValueMaxLen  = 1024
	CustomEmbedMaxFields         = 25
	CustomEmbedMaxTotalLen       = 6000
)

// CustomEmbedFieldConfig captures one field in a custom embed.
type CustomEmbedFieldConfig struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// CustomEmbedPostingConfig identifies one Discord message authored by the bot.
type CustomEmbedPostingConfig struct {
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
	WebhookID    string `json:"webhook_id,omitempty"`
	WebhookToken string `json:"webhook_token,omitempty"`
}

// IsZero is zero.
func (p CustomEmbedPostingConfig) IsZero() bool {
	return strings.TrimSpace(p.ChannelID) == "" &&
		strings.TrimSpace(p.MessageID) == "" &&
		strings.TrimSpace(p.WebhookID) == "" &&
		strings.TrimSpace(p.WebhookToken) == ""
}

// CustomEmbedConfig captures one keyed custom embed for a guild.
type CustomEmbedConfig struct {
	Key           string                     `json:"key"`
	Title         string                     `json:"title,omitempty"`
	Description   string                     `json:"description,omitempty"`
	Color         int                        `json:"color,omitempty"`
	AuthorName    string                     `json:"author_name,omitempty"`
	AuthorIconURL string                     `json:"author_icon_url,omitempty"`
	FooterText    string                     `json:"footer_text,omitempty"`
	FooterIconURL string                     `json:"footer_icon_url,omitempty"`
	ImageURL      string                     `json:"image_url,omitempty"`
	ThumbnailURL  string                     `json:"thumbnail_url,omitempty"`
	Fields        []CustomEmbedFieldConfig   `json:"fields,omitempty"`
	Postings      []CustomEmbedPostingConfig `json:"postings,omitempty"`
}

// IsZero reports whether the embed carries no meaningful data.
func (cfg CustomEmbedConfig) IsZero() bool {
	return strings.TrimSpace(cfg.Key) == "" &&
		strings.TrimSpace(cfg.Title) == "" &&
		strings.TrimSpace(cfg.Description) == "" &&
		cfg.Color == 0 &&
		strings.TrimSpace(cfg.AuthorName) == "" &&
		strings.TrimSpace(cfg.AuthorIconURL) == "" &&
		strings.TrimSpace(cfg.FooterText) == "" &&
		strings.TrimSpace(cfg.FooterIconURL) == "" &&
		strings.TrimSpace(cfg.ImageURL) == "" &&
		strings.TrimSpace(cfg.ThumbnailURL) == "" &&
		len(cfg.Fields) == 0 &&
		len(cfg.Postings) == 0
}

func invalidCustomEmbedInput(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%w: %s", ErrInvalidCustomEmbedInput, msg)
}

// NormalizeCustomEmbedKey normalizes custom embed key.
func NormalizeCustomEmbedKey(raw string) string {
	out := strings.TrimSpace(raw)
	out = strings.ToLower(out)
	return out
}

func validateCustomEmbedKey(raw string) (string, error) {
	out := NormalizeCustomEmbedKey(raw)
	if out == "" {
		return "", invalidCustomEmbedInput("key is required")
	}
	if utf8.RuneCountInString(out) > CustomEmbedKeyMaxLen {
		return "", invalidCustomEmbedInput("key must be at most %d characters", CustomEmbedKeyMaxLen)
	}
	for _, r := range out {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return "", invalidCustomEmbedInput("key may only contain lowercase letters, digits, '-' and '_'")
		}
	}
	return out, nil
}

func validateCustomEmbedFields(in CustomEmbedConfig) (CustomEmbedConfig, error) {
	out := in
	out.Title = strings.TrimSpace(in.Title)
	out.Description = strings.TrimSpace(in.Description)
	out.AuthorName = strings.TrimSpace(in.AuthorName)
	out.AuthorIconURL = strings.TrimSpace(in.AuthorIconURL)
	out.FooterText = strings.TrimSpace(in.FooterText)
	out.FooterIconURL = strings.TrimSpace(in.FooterIconURL)
	out.ImageURL = strings.TrimSpace(in.ImageURL)
	out.ThumbnailURL = strings.TrimSpace(in.ThumbnailURL)

	if utf8.RuneCountInString(out.Title) > CustomEmbedTitleMaxLen {
		return CustomEmbedConfig{}, invalidCustomEmbedInput("title must be at most %d characters", CustomEmbedTitleMaxLen)
	}
	if utf8.RuneCountInString(out.Description) > CustomEmbedDescriptionMaxLen {
		return CustomEmbedConfig{}, invalidCustomEmbedInput("description must be at most %d characters", CustomEmbedDescriptionMaxLen)
	}
	if out.Color < 0 || out.Color > CustomEmbedColorMax {
		return CustomEmbedConfig{}, invalidCustomEmbedInput("color must be in range [0, %d]", CustomEmbedColorMax)
	}
	if utf8.RuneCountInString(out.AuthorName) > CustomEmbedAuthorMaxLen {
		return CustomEmbedConfig{}, invalidCustomEmbedInput("author_name must be at most %d characters", CustomEmbedAuthorMaxLen)
	}
	if utf8.RuneCountInString(out.FooterText) > CustomEmbedFooterMaxLen {
		return CustomEmbedConfig{}, invalidCustomEmbedInput("footer_text must be at most %d characters", CustomEmbedFooterMaxLen)
	}
	return out, nil
}

func customEmbedTotalLen(embed CustomEmbedConfig) int {
	count := utf8.RuneCountInString(embed.Title) +
		utf8.RuneCountInString(embed.Description) +
		utf8.RuneCountInString(embed.AuthorName) +
		utf8.RuneCountInString(embed.FooterText)
	for _, f := range embed.Fields {
		count += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
	}
	return count
}

func normalizeCustomEmbedField(in CustomEmbedFieldConfig) (CustomEmbedFieldConfig, error) {
	out := CustomEmbedFieldConfig{
		Name:   strings.TrimSpace(in.Name),
		Value:  strings.TrimSpace(in.Value),
		Inline: in.Inline,
	}
	if out.Name == "" {
		return CustomEmbedFieldConfig{}, invalidCustomEmbedInput("field name is required")
	}
	if out.Value == "" {
		return CustomEmbedFieldConfig{}, invalidCustomEmbedInput("field value is required")
	}
	if utf8.RuneCountInString(out.Name) > CustomEmbedFieldNameMaxLen {
		return CustomEmbedFieldConfig{}, invalidCustomEmbedInput("field name must be at most %d characters", CustomEmbedFieldNameMaxLen)
	}
	if utf8.RuneCountInString(out.Value) > CustomEmbedFieldValueMaxLen {
		return CustomEmbedFieldConfig{}, invalidCustomEmbedInput("field value must be at most %d characters", CustomEmbedFieldValueMaxLen)
	}
	return out, nil
}

func normalizeCustomEmbed(in CustomEmbedConfig) (CustomEmbedConfig, error) {
	key, err := validateCustomEmbedKey(in.Key)
	if err != nil {
		return CustomEmbedConfig{}, fmt.Errorf("normalizeCustomEmbed: %w", err)
	}
	out, err := validateCustomEmbedFields(in)
	if err != nil {
		return CustomEmbedConfig{}, fmt.Errorf("normalizeCustomEmbed: %w", err)
	}
	out.Key = key

	if len(in.Fields) > 0 {
		out.Fields = make([]CustomEmbedFieldConfig, 0, len(in.Fields))
		for i, f := range in.Fields {
			nf, err := normalizeCustomEmbedField(f)
			if err != nil {
				return CustomEmbedConfig{}, fmt.Errorf("fields[%d]: %w", i, err)
			}
			out.Fields = append(out.Fields, nf)
		}
		if len(out.Fields) > CustomEmbedMaxFields {
			return CustomEmbedConfig{}, invalidCustomEmbedInput("embed must have at most %d fields", CustomEmbedMaxFields)
		}
	} else {
		out.Fields = nil
	}

	if len(in.Postings) > 0 {
		out.Postings = make([]CustomEmbedPostingConfig, 0, len(in.Postings))
		for _, p := range in.Postings {
			if p.IsZero() {
				continue
			}
			out.Postings = append(out.Postings, CustomEmbedPostingConfig{
				ChannelID:    strings.TrimSpace(p.ChannelID),
				MessageID:    strings.TrimSpace(p.MessageID),
				WebhookID:    strings.TrimSpace(p.WebhookID),
				WebhookToken: strings.TrimSpace(p.WebhookToken),
			})
		}
	} else {
		out.Postings = nil
	}

	return out, nil
}

func cloneCustomEmbed(in CustomEmbedConfig) CustomEmbedConfig {
	out := CustomEmbedConfig{
		Key:           in.Key,
		Title:         in.Title,
		Description:   in.Description,
		Color:         in.Color,
		AuthorName:    in.AuthorName,
		AuthorIconURL: in.AuthorIconURL,
		FooterText:    in.FooterText,
		FooterIconURL: in.FooterIconURL,
		ImageURL:      in.ImageURL,
		ThumbnailURL:  in.ThumbnailURL,
	}

	if len(in.Fields) > 0 {
		out.Fields = make([]CustomEmbedFieldConfig, len(in.Fields))
		copy(out.Fields, in.Fields)
	}

	if len(in.Postings) > 0 {
		out.Postings = make([]CustomEmbedPostingConfig, len(in.Postings))
		copy(out.Postings, in.Postings)
	}

	return out
}

func findCustomEmbedIndex(embeds []CustomEmbedConfig, key string) int {
	for i, e := range embeds {
		if e.Key == key {
			return i
		}
	}
	return -1
}

// CustomEmbeds customs embeds.
func (mgr *ConfigManager) CustomEmbeds(guildID string) ([]CustomEmbedConfig, error) {
	if guildID == "" {
		return nil, invalidCustomEmbedInput("guild_id is required")
	}

	gcfg := mgr.GuildConfig(guildID)
	if gcfg == nil {
		return nil, nil
	}

	if len(gcfg.CustomEmbeds) == 0 {
		return nil, nil
	}

	out := make([]CustomEmbedConfig, 0, len(gcfg.CustomEmbeds))
	for _, e := range gcfg.CustomEmbeds {
		out = append(out, cloneCustomEmbed(e))
	}
	return out, nil
}

// CustomEmbed customs embed.
func (mgr *ConfigManager) CustomEmbed(guildID, key string) (CustomEmbedConfig, error) {
	if guildID == "" {
		return CustomEmbedConfig{}, invalidCustomEmbedInput("guild_id is required")
	}
	target, err := validateCustomEmbedKey(key)
	if err != nil {
		return CustomEmbedConfig{}, fmt.Errorf("ConfigManager.CustomEmbed: %w", err)
	}

	gcfg := mgr.GuildConfig(guildID)
	if gcfg == nil {
		return CustomEmbedConfig{}, fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, target)
	}

	idx := findCustomEmbedIndex(gcfg.CustomEmbeds, target)
	if idx < 0 {
		return CustomEmbedConfig{}, fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, target)
	}

	return cloneCustomEmbed(gcfg.CustomEmbeds[idx]), nil
}

// SetCustomEmbedProperties sets custom embed properties.
func (mgr *ConfigManager) SetCustomEmbedProperties(guildID, key string, embed CustomEmbedConfig) error {
	if guildID == "" {
		return invalidCustomEmbedInput("guild_id is required")
	}
	targetKey, err := validateCustomEmbedKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetCustomEmbedProperties: %w", err)
	}
	validated, err := validateCustomEmbedFields(embed)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetCustomEmbedProperties: %w", err)
	}

	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.SetCustomEmbedProperties: %w", err)
		}

		idx := findCustomEmbedIndex(gc.CustomEmbeds, targetKey)
		if idx >= 0 {
			copyEmbed := gc.CustomEmbeds[idx]
			copyEmbed.Title = validated.Title
			copyEmbed.Description = validated.Description
			copyEmbed.Color = validated.Color
			copyEmbed.AuthorName = validated.AuthorName
			copyEmbed.AuthorIconURL = validated.AuthorIconURL
			copyEmbed.FooterText = validated.FooterText
			copyEmbed.FooterIconURL = validated.FooterIconURL
			copyEmbed.ImageURL = validated.ImageURL
			copyEmbed.ThumbnailURL = validated.ThumbnailURL

			if customEmbedTotalLen(copyEmbed) > CustomEmbedMaxTotalLen {
				return invalidCustomEmbedInput("embed total character count must be at most %d", CustomEmbedMaxTotalLen)
			}

			gc.CustomEmbeds[idx] = copyEmbed
		} else {
			if len(gc.CustomEmbeds) >= 25 {
				return invalidCustomEmbedInput("guild cannot have more than 25 custom embeds")
			}
			newEmbed := CustomEmbedConfig{
				Key:           targetKey,
				Title:         validated.Title,
				Description:   validated.Description,
				Color:         validated.Color,
				AuthorName:    validated.AuthorName,
				AuthorIconURL: validated.AuthorIconURL,
				FooterText:    validated.FooterText,
				FooterIconURL: validated.FooterIconURL,
				ImageURL:      validated.ImageURL,
				ThumbnailURL:  validated.ThumbnailURL,
			}
			gc.CustomEmbeds = append(gc.CustomEmbeds, newEmbed)
		}
		return nil
	})
	return err
}

// DeleteCustomEmbed deletes custom embed.
func (mgr *ConfigManager) DeleteCustomEmbed(guildID, key string) (CustomEmbedConfig, error) {
	if guildID == "" {
		return CustomEmbedConfig{}, invalidCustomEmbedInput("guild_id is required")
	}
	target, err := validateCustomEmbedKey(key)
	if err != nil {
		return CustomEmbedConfig{}, fmt.Errorf("ConfigManager.DeleteCustomEmbed: %w", err)
	}

	var deleted CustomEmbedConfig
	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.DeleteCustomEmbed: %w", err)
		}

		idx := findCustomEmbedIndex(gc.CustomEmbeds, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, target)
		}

		deleted = cloneCustomEmbed(gc.CustomEmbeds[idx])
		gc.CustomEmbeds = append(gc.CustomEmbeds[:idx], gc.CustomEmbeds[idx+1:]...)
		return nil
	})
	return deleted, err
}

// AddCustomEmbedPosting adds custom embed posting.
func (mgr *ConfigManager) AddCustomEmbedPosting(guildID, key string, posting CustomEmbedPostingConfig) error {
	if guildID == "" {
		return invalidCustomEmbedInput("guild_id is required")
	}
	if posting.IsZero() {
		return invalidCustomEmbedInput("posting cannot be empty")
	}
	targetKey, err := validateCustomEmbedKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.AddCustomEmbedPosting: %w", err)
	}

	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.AddCustomEmbedPosting: %w", err)
		}

		idx := findCustomEmbedIndex(gc.CustomEmbeds, targetKey)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, targetKey)
		}

		embed := &gc.CustomEmbeds[idx]
		for _, p := range embed.Postings {
			if p.MessageID == posting.MessageID {
				return nil
			}
		}

		if len(embed.Postings) >= 50 {
			embed.Postings = embed.Postings[1:]
		}
		embed.Postings = append(embed.Postings, CustomEmbedPostingConfig{
			ChannelID:    strings.TrimSpace(posting.ChannelID),
			MessageID:    strings.TrimSpace(posting.MessageID),
			WebhookID:    strings.TrimSpace(posting.WebhookID),
			WebhookToken: strings.TrimSpace(posting.WebhookToken),
		})
		return nil
	})
	return err
}

// RemoveCustomEmbedPosting removes custom embed posting.
func (mgr *ConfigManager) RemoveCustomEmbedPosting(guildID, key, messageID string) error {
	if guildID == "" {
		return invalidCustomEmbedInput("guild_id is required")
	}
	msgID := strings.TrimSpace(messageID)
	if msgID == "" {
		return invalidCustomEmbedInput("message_id is required")
	}
	targetKey, err := validateCustomEmbedKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.RemoveCustomEmbedPosting: %w", err)
	}

	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.RemoveCustomEmbedPosting: %w", err)
		}

		idx := findCustomEmbedIndex(gc.CustomEmbeds, targetKey)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, targetKey)
		}

		embed := &gc.CustomEmbeds[idx]
		for i, p := range embed.Postings {
			if p.MessageID == msgID {
				embed.Postings = append(embed.Postings[:i], embed.Postings[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("%w: message_id=%s", ErrCustomEmbedPostingNotFound, msgID)
	})
	return err
}

// RemoveCustomEmbedPostings removes custom embed postings.
func (mgr *ConfigManager) RemoveCustomEmbedPostings(guildID, key string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if guildID == "" {
		return invalidCustomEmbedInput("guild_id is required")
	}
	targetKey, err := validateCustomEmbedKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.RemoveCustomEmbedPostings: %w", err)
	}

	idsToRemove := make(map[string]bool, len(messageIDs))
	for _, id := range messageIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			idsToRemove[trimmed] = true
		}
	}
	if len(idsToRemove) == 0 {
		return nil
	}

	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.RemoveCustomEmbedPostings: %w", err)
		}

		idx := findCustomEmbedIndex(gc.CustomEmbeds, targetKey)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, targetKey)
		}

		embed := &gc.CustomEmbeds[idx]
		var kept []CustomEmbedPostingConfig
		for _, p := range embed.Postings {
			if !idsToRemove[p.MessageID] {
				kept = append(kept, p)
			}
		}
		embed.Postings = kept
		return nil
	})
	return err
}

// SetCustomEmbedFields sets custom embed fields.
func (mgr *ConfigManager) SetCustomEmbedFields(guildID, key string, fields []CustomEmbedFieldConfig) error {
	if guildID == "" {
		return invalidCustomEmbedInput("guild_id is required")
	}
	targetKey, err := validateCustomEmbedKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetCustomEmbedFields: %w", err)
	}

	if len(fields) > CustomEmbedMaxFields {
		return invalidCustomEmbedInput("embed must have at most %d fields", CustomEmbedMaxFields)
	}

	normalized := make([]CustomEmbedFieldConfig, 0, len(fields))
	for i, f := range fields {
		nf, err := normalizeCustomEmbedField(f)
		if err != nil {
			return fmt.Errorf("fields[%d]: %w", i, err)
		}
		normalized = append(normalized, nf)
	}

	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.SetCustomEmbedFields: %w", err)
		}

		idx := findCustomEmbedIndex(gc.CustomEmbeds, targetKey)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, targetKey)
		}

		copyEmbed := gc.CustomEmbeds[idx]
		copyEmbed.Fields = normalized

		if customEmbedTotalLen(copyEmbed) > CustomEmbedMaxTotalLen {
			return invalidCustomEmbedInput("embed total character count must be at most %d", CustomEmbedMaxTotalLen)
		}

		gc.CustomEmbeds[idx] = copyEmbed
		return nil
	})
	return err
}

// FindCustomEmbedPosting searches all custom embeds in a guild for a posting
// matching the message ID. Returns the custom embed key plus the posting on
// hit, or ErrCustomEmbedPostingNotFound when no custom embed tracks the
// message.
func (mgr *ConfigManager) FindCustomEmbedPosting(guildID, messageID string) (string, CustomEmbedPostingConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return "", CustomEmbedPostingConfig{}, invalidCustomEmbedInput("guild_id is required")
	}
	mid := strings.TrimSpace(messageID)
	if mid == "" {
		return "", CustomEmbedPostingConfig{}, invalidCustomEmbedInput("message_id is required")
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return "", CustomEmbedPostingConfig{}, fmt.Errorf("ConfigManager.FindCustomEmbedPosting: %w", err)
	}
	for _, ce := range guildConfig.CustomEmbeds {
		pIdx := findCustomEmbedPostingIndex(ce.Postings, mid)
		if pIdx >= 0 {
			return ce.Key, ce.Postings[pIdx], nil
		}
	}
	return "", CustomEmbedPostingConfig{}, fmt.Errorf("%w: message_id=%s", ErrCustomEmbedPostingNotFound, mid)
}

func findCustomEmbedPostingIndex(postings []CustomEmbedPostingConfig, messageID string) int {
	for i, p := range postings {
		if p.MessageID == messageID {
			return i
		}
	}
	return -1
}

// AddCustomEmbedField appends a field to the custom embed.
func (mgr *ConfigManager) AddCustomEmbedField(guildID, key string, field CustomEmbedFieldConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidCustomEmbedInput("guild_id is required")
	}
	target, err := validateCustomEmbedKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.AddCustomEmbedField: %w", err)
	}
	nf, err := normalizeCustomEmbedField(field)
	if err != nil {
		return fmt.Errorf("ConfigManager.AddCustomEmbedField: %w", err)
	}

	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, scope)
		if err != nil {
			return fmt.Errorf("ConfigManager.AddCustomEmbedField: %w", err)
		}
		idx := findCustomEmbedIndex(gc.CustomEmbeds, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, target)
		}
		if len(gc.CustomEmbeds[idx].Fields) >= CustomEmbedMaxFields {
			return invalidCustomEmbedInput("embed must have at most %d fields", CustomEmbedMaxFields)
		}

		copyEmbed := gc.CustomEmbeds[idx]
		copyEmbed.Fields = append(copyEmbed.Fields, nf)

		if customEmbedTotalLen(copyEmbed) > CustomEmbedMaxTotalLen {
			return invalidCustomEmbedInput("embed total character count must be at most %d", CustomEmbedMaxTotalLen)
		}

		gc.CustomEmbeds[idx] = copyEmbed
		return nil
	})
	return err
}

// RemoveCustomEmbedField removes a field from the custom embed by its index (0-based).
func (mgr *ConfigManager) RemoveCustomEmbedField(guildID, key string, fieldIndex int) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidCustomEmbedInput("guild_id is required")
	}
	target, err := validateCustomEmbedKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.RemoveCustomEmbedField: %w", err)
	}

	_, err = mgr.UpdateConfig(func(cfg *BotConfig) error {
		gc, err := guildConfigByID(cfg, scope)
		if err != nil {
			return fmt.Errorf("ConfigManager.RemoveCustomEmbedField: %w", err)
		}
		idx := findCustomEmbedIndex(gc.CustomEmbeds, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrCustomEmbedNotFound, target)
		}
		fields := gc.CustomEmbeds[idx].Fields
		if fieldIndex < 0 || fieldIndex >= len(fields) {
			return invalidCustomEmbedInput("invalid field index")
		}
		gc.CustomEmbeds[idx].Fields = append(fields[:fieldIndex], fields[fieldIndex+1:]...)
		return nil
	})
	return err
}

func cloneCustomEmbeds(in []CustomEmbedConfig) []CustomEmbedConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]CustomEmbedConfig, 0, len(in))
	for _, ce := range in {
		out = append(out, cloneCustomEmbed(ce))
	}
	return out
}
