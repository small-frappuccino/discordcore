package files

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	// ErrRolePanelNotFound indicates no role panel matched the requested key.
	ErrRolePanelNotFound = errors.New("role panel not found")
	// ErrRolePanelButtonNotFound indicates no panel button matched the requested role ID.
	ErrRolePanelButtonNotFound = errors.New("role panel button not found")
	// ErrRolePanelPostingNotFound indicates no posting matched the requested message ID.
	ErrRolePanelPostingNotFound = errors.New("role panel posting not found")
	// ErrInvalidRolePanelInput indicates invalid role panel input payload.
	ErrInvalidRolePanelInput = errors.New("invalid role panel input")
)

const (
	// RolePanelMaxButtons mirrors Discord's hard cap of 25 components per
	// message (5 ActionRows × 5 buttons each).
	RolePanelMaxButtons = 25
	// RolePanelKeyMaxLen bounds the per-guild panel key so command custom
	// IDs and config keys stay readable.
	RolePanelKeyMaxLen = 32
	// RolePanelLabelMaxLen mirrors Discord's button label limit.
	RolePanelLabelMaxLen = 80
	// RolePanelTitleMaxLen mirrors Discord's embed title limit.
	RolePanelTitleMaxLen = 256
	// RolePanelDescriptionMaxLen is the embed description limit. Discord
	// caps at 4096; the slightly smaller bound here leaves a margin for
	// the renderer to add suffixes if needed without re-validating.
	RolePanelDescriptionMaxLen = 4000
	// RolePanelColorMax is the maximum allowed 24-bit RGB color value.
	RolePanelColorMax = 0xFFFFFF
	// RolePanelAuthorMaxLen mirrors Discord's embed author name limit.
	RolePanelAuthorMaxLen = 256
	// RolePanelFooterMaxLen mirrors Discord's embed footer text limit.
	RolePanelFooterMaxLen = 2048
	// RolePanelFieldNameMaxLen mirrors Discord's embed field name limit.
	RolePanelFieldNameMaxLen = 256
	// RolePanelFieldValueMaxLen mirrors Discord's embed field value limit.
	RolePanelFieldValueMaxLen = 1024
	// RolePanelMaxFields mirrors Discord's embed fields limit.
	RolePanelMaxFields = 25
	// RolePanelMaxTotalLen mirrors Discord's embed character limit.
	RolePanelMaxTotalLen = 6000
)

// RolePanelEmbedFieldConfig captures one field in a role panel embed.
type RolePanelEmbedFieldConfig struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// RolePanelButtonConfig captures one toggleable role button on a panel.
//
// EmojiID is set for custom Discord emojis; EmojiName carries either the
// custom emoji name (when EmojiID is set) or the unicode glyph (when
// EmojiID is empty). EmojiAnimated is only meaningful for custom emojis.
type RolePanelButtonConfig struct {
	RoleID        string `json:"role_id"`
	Label         string `json:"label"`
	EmojiName     string `json:"emoji_name,omitempty"`
	EmojiID       string `json:"emoji_id,omitempty"`
	EmojiAnimated bool   `json:"emoji_animated,omitempty"`
}

// RolePanelPostingConfig identifies one Discord message authored by
// the bot that materializes a role panel. Postings are recorded after
// /roles post succeeds so /roles delete and /roles refresh can edit
// the original messages (strip components on delete, re-render
// embed+buttons on refresh) instead of leaving them frozen and
// half-functional.
type RolePanelPostingConfig struct {
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
	WebhookID    string `json:"webhook_id,omitempty"`
	WebhookToken string `json:"webhook_token,omitempty"`
}

// IsZero reports whether the posting carries no meaningful data.
func (p RolePanelPostingConfig) IsZero() bool {
	return strings.TrimSpace(p.ChannelID) == "" && strings.TrimSpace(p.MessageID) == "" && strings.TrimSpace(p.WebhookID) == "" && strings.TrimSpace(p.WebhookToken) == ""
}

// RolePanelConfig captures one keyed role panel for a guild.
type RolePanelConfig struct {
	Key           string                      `json:"key"`
	Title         string                      `json:"title,omitempty"`
	Description   string                      `json:"description,omitempty"`
	Color         int                         `json:"color,omitempty"`
	AuthorName    string                      `json:"author_name,omitempty"`
	AuthorIconURL string                      `json:"author_icon_url,omitempty"`
	FooterText    string                      `json:"footer_text,omitempty"`
	FooterIconURL string                      `json:"footer_icon_url,omitempty"`
	ImageURL      string                      `json:"image_url,omitempty"`
	ThumbnailURL  string                      `json:"thumbnail_url,omitempty"`
	Fields        []RolePanelEmbedFieldConfig `json:"fields,omitempty"`
	Buttons       []RolePanelButtonConfig     `json:"buttons,omitempty"`
	Postings      []RolePanelPostingConfig    `json:"postings,omitempty"`
}

// IsZero reports whether the button carries no meaningful data.
func (b RolePanelButtonConfig) IsZero() bool {
	return strings.TrimSpace(b.RoleID) == "" &&
		strings.TrimSpace(b.Label) == "" &&
		strings.TrimSpace(b.EmojiName) == "" &&
		strings.TrimSpace(b.EmojiID) == ""
}

// HasEmoji reports whether the button carries either a custom or unicode emoji.
func (b RolePanelButtonConfig) HasEmoji() bool {
	return strings.TrimSpace(b.EmojiName) != "" || strings.TrimSpace(b.EmojiID) != ""
}

// IsZero reports whether the panel carries no meaningful data.
func (cfg RolePanelConfig) IsZero() bool {
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
		len(cfg.Buttons) == 0 &&
		len(cfg.Postings) == 0
}

// CloneRolePanelConfig returns a deep copy safe for callers to mutate.
func CloneRolePanelConfig(in RolePanelConfig) RolePanelConfig {
	return cloneRolePanel(in)
}

// CloneRolePanelConfigs returns a deep copy of the panel slice.
func CloneRolePanelConfigs(in []RolePanelConfig) []RolePanelConfig {
	return cloneRolePanels(in)
}

// --- Normalization ---

// NormalizeRolePanelKey lower-cases and trims a key in the canonical form
// used for lookup and storage. Returns an empty string when the input is
// blank after normalization.
func NormalizeRolePanelKey(raw string) string {
	out := strings.TrimSpace(raw)
	out = strings.ToLower(out)
	return out
}

func validateRolePanelKey(raw string) (string, error) {
	out := NormalizeRolePanelKey(raw)
	if out == "" {
		return "", invalidRolePanelInput("key is required")
	}
	if utf8.RuneCountInString(out) > RolePanelKeyMaxLen {
		return "", invalidRolePanelInput("key must be at most %d characters", RolePanelKeyMaxLen)
	}
	for _, r := range out {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return "", invalidRolePanelInput("key may only contain lowercase letters, digits, '-' and '_'")
		}
	}
	return out, nil
}

func normalizeRolePanelButton(in RolePanelButtonConfig) (RolePanelButtonConfig, error) {
	out := RolePanelButtonConfig{
		RoleID:        strings.TrimSpace(in.RoleID),
		Label:         sanitizeSingleLine(in.Label),
		EmojiName:     strings.TrimSpace(in.EmojiName),
		EmojiID:       strings.TrimSpace(in.EmojiID),
		EmojiAnimated: in.EmojiAnimated,
	}
	if out.RoleID == "" {
		return RolePanelButtonConfig{}, invalidRolePanelInput("role_id is required")
	}
	if !isAllDigits(out.RoleID) {
		return RolePanelButtonConfig{}, invalidRolePanelInput("role_id must be numeric")
	}
	if out.Label == "" {
		return RolePanelButtonConfig{}, invalidRolePanelInput("label is required")
	}
	if utf8.RuneCountInString(out.Label) > RolePanelLabelMaxLen {
		return RolePanelButtonConfig{}, invalidRolePanelInput("label must be at most %d characters", RolePanelLabelMaxLen)
	}
	if out.EmojiID != "" {
		if !isAllDigits(out.EmojiID) {
			return RolePanelButtonConfig{}, invalidRolePanelInput("emoji_id must be numeric")
		}
		if out.EmojiName == "" {
			return RolePanelButtonConfig{}, invalidRolePanelInput("emoji_name is required when emoji_id is set")
		}
	} else {
		out.EmojiAnimated = false
	}
	return out, nil
}

func validateRolePanelEmbedFields(in RolePanelConfig) (RolePanelConfig, error) {
	out := in
	out.Title = strings.TrimSpace(in.Title)
	out.Description = strings.TrimSpace(in.Description)
	out.AuthorName = strings.TrimSpace(in.AuthorName)
	out.AuthorIconURL = strings.TrimSpace(in.AuthorIconURL)
	out.FooterText = strings.TrimSpace(in.FooterText)
	out.FooterIconURL = strings.TrimSpace(in.FooterIconURL)
	out.ImageURL = strings.TrimSpace(in.ImageURL)
	out.ThumbnailURL = strings.TrimSpace(in.ThumbnailURL)

	if utf8.RuneCountInString(out.Title) > RolePanelTitleMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("title must be at most %d characters", RolePanelTitleMaxLen)
	}
	if utf8.RuneCountInString(out.Description) > RolePanelDescriptionMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("description must be at most %d characters", RolePanelDescriptionMaxLen)
	}
	if out.Color < 0 || out.Color > RolePanelColorMax {
		return RolePanelConfig{}, invalidRolePanelInput("color must be in range [0, %d]", RolePanelColorMax)
	}
	if utf8.RuneCountInString(out.AuthorName) > RolePanelAuthorMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("author_name must be at most %d characters", RolePanelAuthorMaxLen)
	}
	if utf8.RuneCountInString(out.FooterText) > RolePanelFooterMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("footer_text must be at most %d characters", RolePanelFooterMaxLen)
	}
	return out, nil
}

func rolePanelTotalLen(embed RolePanelConfig) int {
	count := utf8.RuneCountInString(embed.Title) +
		utf8.RuneCountInString(embed.Description) +
		utf8.RuneCountInString(embed.AuthorName) +
		utf8.RuneCountInString(embed.FooterText)
	for _, f := range embed.Fields {
		count += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
	}
	return count
}

func normalizeRolePanelEmbedField(in RolePanelEmbedFieldConfig) (RolePanelEmbedFieldConfig, error) {
	out := RolePanelEmbedFieldConfig{
		Name:   strings.TrimSpace(in.Name),
		Value:  strings.TrimSpace(in.Value),
		Inline: in.Inline,
	}
	if out.Name == "" {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field name is required")
	}
	if out.Value == "" {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field value is required")
	}
	if utf8.RuneCountInString(out.Name) > RolePanelFieldNameMaxLen {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field name must be at most %d characters", RolePanelFieldNameMaxLen)
	}
	if utf8.RuneCountInString(out.Value) > RolePanelFieldValueMaxLen {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field value must be at most %d characters", RolePanelFieldValueMaxLen)
	}
	return out, nil
}

func normalizeRolePanelPosting(in RolePanelPostingConfig) (RolePanelPostingConfig, error) {
	out := RolePanelPostingConfig{
		ChannelID:    strings.TrimSpace(in.ChannelID),
		MessageID:    strings.TrimSpace(in.MessageID),
		WebhookID:    strings.TrimSpace(in.WebhookID),
		WebhookToken: strings.TrimSpace(in.WebhookToken),
	}
	if out.ChannelID == "" {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.channel_id is required")
	}
	if !isAllDigits(out.ChannelID) {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.channel_id must be numeric")
	}
	if out.MessageID == "" {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.message_id is required")
	}
	if !isAllDigits(out.MessageID) {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.message_id must be numeric")
	}
	if out.WebhookID != "" && !isAllDigits(out.WebhookID) {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.webhook_id must be numeric")
	}
	return out, nil
}

func normalizeRolePanel(in RolePanelConfig) (RolePanelConfig, error) {
	key, err := validateRolePanelKey(in.Key)
	if err != nil {
		return RolePanelConfig{}, err
	}
	embedFields, err := validateRolePanelEmbedFields(in)
	if err != nil {
		return RolePanelConfig{}, err
	}

	fields := make([]RolePanelEmbedFieldConfig, 0, len(in.Fields))
	for i, f := range in.Fields {
		nf, err := normalizeRolePanelEmbedField(f)
		if err != nil {
			return RolePanelConfig{}, fmt.Errorf("fields[%d]: %w", i, err)
		}
		fields = append(fields, nf)
	}
	if len(fields) > RolePanelMaxFields {
		return RolePanelConfig{}, invalidRolePanelInput("panel must have at most %d fields", RolePanelMaxFields)
	}

	seen := make(map[string]struct{}, len(in.Buttons))
	buttons := make([]RolePanelButtonConfig, 0, len(in.Buttons))
	for i, b := range in.Buttons {
		nb, err := normalizeRolePanelButton(b)
		if err != nil {
			return RolePanelConfig{}, fmt.Errorf("buttons[%d]: %w", i, err)
		}
		if _, dup := seen[nb.RoleID]; dup {
			continue
		}
		seen[nb.RoleID] = struct{}{}
		buttons = append(buttons, nb)
	}
	if len(buttons) > RolePanelMaxButtons {
		return RolePanelConfig{}, invalidRolePanelInput("panel must have at most %d buttons", RolePanelMaxButtons)
	}

	postings, err := normalizeRolePanelPostingList(in.Postings)
	if err != nil {
		return RolePanelConfig{}, err
	}

	return RolePanelConfig{
		Key:           key,
		Title:         embedFields.Title,
		Description:   embedFields.Description,
		Color:         embedFields.Color,
		AuthorName:    embedFields.AuthorName,
		AuthorIconURL: embedFields.AuthorIconURL,
		FooterText:    embedFields.FooterText,
		FooterIconURL: embedFields.FooterIconURL,
		ImageURL:      embedFields.ImageURL,
		ThumbnailURL:  embedFields.ThumbnailURL,
		Fields:        fields,
		Buttons:       buttons,
		Postings:      postings,
	}, nil
}

func normalizeRolePanelPostingList(in []RolePanelPostingConfig) ([]RolePanelPostingConfig, error) {
	if len(in) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]RolePanelPostingConfig, 0, len(in))
	for i, p := range in {
		np, err := normalizeRolePanelPosting(p)
		if err != nil {
			return nil, fmt.Errorf("postings[%d]: %w", i, err)
		}
		key := np.MessageID
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, np)
	}
	return out, nil
}

func cloneRolePanelButton(in RolePanelButtonConfig) RolePanelButtonConfig {
	return RolePanelButtonConfig{
		RoleID:        in.RoleID,
		Label:         in.Label,
		EmojiName:     in.EmojiName,
		EmojiID:       in.EmojiID,
		EmojiAnimated: in.EmojiAnimated,
	}
}

func cloneRolePanel(in RolePanelConfig) RolePanelConfig {
	out := RolePanelConfig{
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
		out.Fields = make([]RolePanelEmbedFieldConfig, 0, len(in.Fields))
		for _, f := range in.Fields {
			out.Fields = append(out.Fields, RolePanelEmbedFieldConfig{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}
	if len(in.Buttons) > 0 {
		out.Buttons = make([]RolePanelButtonConfig, 0, len(in.Buttons))
		for _, b := range in.Buttons {
			out.Buttons = append(out.Buttons, cloneRolePanelButton(b))
		}
	}
	if len(in.Postings) > 0 {
		out.Postings = make([]RolePanelPostingConfig, 0, len(in.Postings))
		for _, p := range in.Postings {
			out.Postings = append(out.Postings, p)
		}
	}
	return out
}

func cloneRolePanels(in []RolePanelConfig) []RolePanelConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]RolePanelConfig, 0, len(in))
	for _, p := range in {
		out = append(out, cloneRolePanel(p))
	}
	return out
}

func sortRolePanels(panels []RolePanelConfig) {
	sort.SliceStable(panels, func(i, j int) bool {
		return panels[i].Key < panels[j].Key
	})
}

func findRolePanelIndex(panels []RolePanelConfig, key string) int {
	target := NormalizeRolePanelKey(key)
	if target == "" {
		return -1
	}
	for i, p := range panels {
		if NormalizeRolePanelKey(p.Key) == target {
			return i
		}
	}
	return -1
}

func findRolePanelButtonIndex(buttons []RolePanelButtonConfig, roleID string) int {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return -1
	}
	for i, b := range buttons {
		if strings.TrimSpace(b.RoleID) == roleID {
			return i
		}
	}
	return -1
}

func findRolePanelPostingIndex(postings []RolePanelPostingConfig, messageID string) int {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return -1
	}
	for i, p := range postings {
		if strings.TrimSpace(p.MessageID) == messageID {
			return i
		}
	}
	return -1
}

func invalidRolePanelInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRolePanelInput, fmt.Sprintf(format, args...))
}

// --- ConfigManager API ---

// RolePanels returns the role panels configured for a guild in
// deterministic key order. Callers receive a deep copy and may mutate
// freely.
func (mgr *ConfigManager) RolePanels(guildID string) ([]RolePanelConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return nil, invalidRolePanelInput("guild_id is required")
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return nil, err
	}
	out := cloneRolePanels(guildConfig.RolePanels)
	sortRolePanels(out)
	return out, nil
}

// RolePanel returns one panel by key. Returns ErrRolePanelNotFound when
// the panel does not exist.
func (mgr *ConfigManager) RolePanel(guildID, key string) (RolePanelConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return RolePanelConfig{}, invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return RolePanelConfig{}, err
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return RolePanelConfig{}, err
	}
	idx := findRolePanelIndex(guildConfig.RolePanels, target)
	if idx < 0 {
		return RolePanelConfig{}, fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
	}
	return cloneRolePanel(guildConfig.RolePanels[idx]), nil
}

// SetRolePanelEmbed sets the embed-level fields for one panel,
// creating the panel when missing. Buttons, fields, and postings on an
// existing panel are preserved.
func (mgr *ConfigManager) SetRolePanelEmbed(guildID, key string, embed RolePanelConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}

	validated, err := validateRolePanelEmbedFields(embed)
	if err != nil {
		return err
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			validated.Key = target
			if rolePanelTotalLen(validated) > RolePanelMaxTotalLen {
				return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
			}
			gc.RolePanels = append(gc.RolePanels, validated)
			sortRolePanels(gc.RolePanels)
			return nil
		}

		copyEmbed := gc.RolePanels[idx]
		copyEmbed.Title = validated.Title
		copyEmbed.Description = validated.Description
		copyEmbed.Color = validated.Color
		copyEmbed.AuthorName = validated.AuthorName
		copyEmbed.AuthorIconURL = validated.AuthorIconURL
		copyEmbed.FooterText = validated.FooterText
		copyEmbed.FooterIconURL = validated.FooterIconURL
		copyEmbed.ImageURL = validated.ImageURL
		copyEmbed.ThumbnailURL = validated.ThumbnailURL

		if rolePanelTotalLen(copyEmbed) > RolePanelMaxTotalLen {
			return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
		}

		gc.RolePanels[idx] = copyEmbed
		return nil
	})
}

// AddRolePanelField appends a field to the panel's embed.
func (mgr *ConfigManager) AddRolePanelField(guildID, key string, field RolePanelEmbedFieldConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}
	validated, err := normalizeRolePanelEmbedField(field)
	if err != nil {
		return err
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		if len(gc.RolePanels[idx].Fields) >= RolePanelMaxFields {
			return invalidRolePanelInput("panel must have at most %d fields", RolePanelMaxFields)
		}

		copyEmbed := gc.RolePanels[idx]
		copyEmbed.Fields = append(copyEmbed.Fields, validated)

		if rolePanelTotalLen(copyEmbed) > RolePanelMaxTotalLen {
			return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
		}

		gc.RolePanels[idx] = copyEmbed
		return nil
	})
}

// RemoveRolePanelField removes a field from the panel's embed by its index (0-based).
func (mgr *ConfigManager) RemoveRolePanelField(guildID, key string, fieldIndex int) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		fields := gc.RolePanels[idx].Fields
		if fieldIndex < 0 || fieldIndex >= len(fields) {
			return invalidRolePanelInput("invalid field index")
		}

		normalized := slices.Delete(fields, fieldIndex, fieldIndex+1)

		copyEmbed := gc.RolePanels[idx]
		copyEmbed.Fields = normalized

		if rolePanelTotalLen(copyEmbed) > RolePanelMaxTotalLen {
			return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
		}

		gc.RolePanels[idx] = copyEmbed
		return nil
	})
}

// UpsertRolePanelButton inserts a new button or replaces the existing
// one matching the same role ID, creating the panel when missing.
func (mgr *ConfigManager) UpsertRolePanelButton(guildID, key string, button RolePanelButtonConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}
	normalized, err := normalizeRolePanelButton(button)
	if err != nil {
		return err
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			gc.RolePanels = append(gc.RolePanels, RolePanelConfig{
				Key:     target,
				Buttons: []RolePanelButtonConfig{normalized},
			})
			sortRolePanels(gc.RolePanels)
			return nil
		}
		buttons := gc.RolePanels[idx].Buttons
		btnIdx := findRolePanelButtonIndex(buttons, normalized.RoleID)
		if btnIdx >= 0 {
			buttons[btnIdx] = normalized
			gc.RolePanels[idx].Buttons = buttons
			return nil
		}
		if len(buttons) >= RolePanelMaxButtons {
			return invalidRolePanelInput("panel must have at most %d buttons", RolePanelMaxButtons)
		}
		gc.RolePanels[idx].Buttons = append(buttons, normalized)
		return nil
	})
}

// DeleteRolePanelButton removes the button matching the given role ID
// from a panel. Returns ErrRolePanelNotFound or
// ErrRolePanelButtonNotFound when the targets do not exist.
func (mgr *ConfigManager) DeleteRolePanelButton(guildID, key, roleID string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}
	rid := strings.TrimSpace(roleID)
	if rid == "" {
		return invalidRolePanelInput("role_id is required")
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		btnIdx := findRolePanelButtonIndex(gc.RolePanels[idx].Buttons, rid)
		if btnIdx < 0 {
			return fmt.Errorf("%w: role_id=%s", ErrRolePanelButtonNotFound, rid)
		}
		gc.RolePanels[idx].Buttons = slices.Delete(gc.RolePanels[idx].Buttons, btnIdx, btnIdx+1)
		return nil
	})
}

// DeleteRolePanel removes the entire panel for a guild. Returns
// ErrRolePanelNotFound when the panel does not exist.
func (mgr *ConfigManager) DeleteRolePanel(guildID, key string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		gc.RolePanels = slices.Delete(gc.RolePanels, idx, idx+1)
		return nil
	})
}

// ListRolePanelPostings returns the persisted (channel_id, message_id)
// pairs for one panel in insertion order.
func (mgr *ConfigManager) ListRolePanelPostings(guildID, key string) ([]RolePanelPostingConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return nil, invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return nil, err
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return nil, err
	}
	idx := findRolePanelIndex(guildConfig.RolePanels, target)
	if idx < 0 {
		return nil, fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
	}
	postings := guildConfig.RolePanels[idx].Postings
	if len(postings) == 0 {
		return nil, nil
	}
	out := make([]RolePanelPostingConfig, len(postings))
	copy(out, postings)
	return out, nil
}

// AddRolePanelPosting records a (channel_id, message_id) pair on a
// panel. Duplicates by message ID are silently coalesced. The panel
// must already exist.
func (mgr *ConfigManager) AddRolePanelPosting(guildID, key string, posting RolePanelPostingConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}
	normalized, err := normalizeRolePanelPosting(posting)
	if err != nil {
		return err
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		if findRolePanelPostingIndex(gc.RolePanels[idx].Postings, normalized.MessageID) >= 0 {
			return nil
		}
		gc.RolePanels[idx].Postings = append(gc.RolePanels[idx].Postings, normalized)
		return nil
	})
}

// RemoveRolePanelPosting drops a (channel_id, message_id) pair from a
// panel. Returns ErrRolePanelPostingNotFound when the message is not
// tracked.
func (mgr *ConfigManager) RemoveRolePanelPosting(guildID, key, messageID string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}
	mid := strings.TrimSpace(messageID)
	if mid == "" {
		return invalidRolePanelInput("message_id is required")
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		pIdx := findRolePanelPostingIndex(gc.RolePanels[idx].Postings, mid)
		if pIdx < 0 {
			return fmt.Errorf("%w: message_id=%s", ErrRolePanelPostingNotFound, mid)
		}
		gc.RolePanels[idx].Postings = slices.Delete(gc.RolePanels[idx].Postings, pIdx, pIdx+1)
		return nil
	})
}

// RemoveRolePanelPostings drops multiple (channel_id, message_id) pairs from a
// panel. Message IDs that are not tracked are safely ignored.
func (mgr *ConfigManager) RemoveRolePanelPostings(guildID, key string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
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

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}

		var kept []RolePanelPostingConfig
		for _, p := range gc.RolePanels[idx].Postings {
			if !idsToRemove[p.MessageID] {
				kept = append(kept, p)
			}
		}
		gc.RolePanels[idx].Postings = kept
		return nil
	})
}

// ClearRolePanelPostings drops every recorded posting for a panel.
// Used by /roles delete after the postings have been edited; the
// caller is responsible for the message-edit pass.
func (mgr *ConfigManager) ClearRolePanelPostings(guildID, key string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return err
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		gc.RolePanels[idx].Postings = nil
		return nil
	})
}

// FindRolePanelPosting searches all panels in a guild for a posting
// matching the message ID. Returns the panel key plus the posting on
// hit, or ErrRolePanelPostingNotFound when no panel tracks the
// message.
func (mgr *ConfigManager) FindRolePanelPosting(guildID, messageID string) (string, RolePanelPostingConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return "", RolePanelPostingConfig{}, invalidRolePanelInput("guild_id is required")
	}
	mid := strings.TrimSpace(messageID)
	if mid == "" {
		return "", RolePanelPostingConfig{}, invalidRolePanelInput("message_id is required")
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return "", RolePanelPostingConfig{}, err
	}
	for _, panel := range guildConfig.RolePanels {
		pIdx := findRolePanelPostingIndex(panel.Postings, mid)
		if pIdx >= 0 {
			return panel.Key, panel.Postings[pIdx], nil
		}
	}
	return "", RolePanelPostingConfig{}, fmt.Errorf("%w: message_id=%s", ErrRolePanelPostingNotFound, mid)
}

// RolePanelButtonByRoleID searches all panels in a guild for a button
// matching the role ID. Used by the component handler to validate
// toggle requests against the current persisted configuration. Returns
// ErrRolePanelButtonNotFound when no panel button references the role.
func (mgr *ConfigManager) RolePanelButtonByRoleID(guildID, roleID string) (RolePanelConfig, RolePanelButtonConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return RolePanelConfig{}, RolePanelButtonConfig{}, invalidRolePanelInput("guild_id is required")
	}
	rid := strings.TrimSpace(roleID)
	if rid == "" {
		return RolePanelConfig{}, RolePanelButtonConfig{}, invalidRolePanelInput("role_id is required")
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return RolePanelConfig{}, RolePanelButtonConfig{}, err
	}
	for _, panel := range guildConfig.RolePanels {
		btnIdx := findRolePanelButtonIndex(panel.Buttons, rid)
		if btnIdx >= 0 {
			return cloneRolePanel(panel), cloneRolePanelButton(panel.Buttons[btnIdx]), nil
		}
	}
	return RolePanelConfig{}, RolePanelButtonConfig{}, fmt.Errorf("%w: role_id=%s", ErrRolePanelButtonNotFound, rid)
}
