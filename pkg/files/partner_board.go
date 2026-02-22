package files

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
)

var (
	// ErrPartnerNotFound indicates no partner matched the requested key.
	ErrPartnerNotFound = errors.New("partner not found")
	// ErrPartnerAlreadyExists indicates a duplicate partner key (name/link).
	ErrPartnerAlreadyExists = errors.New("partner already exists")
	// ErrGuildConfigNotFound indicates requested guild config was not found.
	ErrGuildConfigNotFound = errors.New("guild config not found")
	// ErrInvalidPartnerBoardInput indicates invalid partner board input payload.
	ErrInvalidPartnerBoardInput = errors.New("invalid partner board input")
)

// GetPartnerBoardTarget returns the configured board target for a guild.
func (mgr *ConfigManager) GetPartnerBoardTarget(guildID string) (EmbedUpdateTargetConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return EmbedUpdateTargetConfig{}, fmt.Errorf("get partner board target: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return EmbedUpdateTargetConfig{}, err
	}

	target, err := normalizeEmbedUpdateTargetConfig(guildConfig.PartnerBoard.Target)
	if err != nil {
		return EmbedUpdateTargetConfig{}, fmt.Errorf("get partner board target: %w", err)
	}
	return target, nil
}

// SetPartnerBoardTarget sets or clears the board update target for a guild.
func (mgr *ConfigManager) SetPartnerBoardTarget(guildID string, target EmbedUpdateTargetConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("set partner board target: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	normalized, err := normalizeEmbedUpdateTargetConfig(target)
	if err != nil {
		return fmt.Errorf("set partner board target: %w", err)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig, err := mgr.guildConfigByIDLockedMutable(scope)
	if err != nil {
		return err
	}

	guildConfig.PartnerBoard.Target = normalized

	if err := mgr.saveConfigLocked(); err != nil {
		return fmt.Errorf("set partner board target: save config: %w", err)
	}
	return nil
}

// GetPartnerBoardTemplate returns the configured board template for a guild.
func (mgr *ConfigManager) GetPartnerBoardTemplate(guildID string) (PartnerBoardTemplateConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return PartnerBoardTemplateConfig{}, fmt.Errorf("get partner board template: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return PartnerBoardTemplateConfig{}, err
	}
	return normalizePartnerBoardTemplate(guildConfig.PartnerBoard.Template), nil
}

// SetPartnerBoardTemplate sets the board template for a guild.
func (mgr *ConfigManager) SetPartnerBoardTemplate(guildID string, template PartnerBoardTemplateConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("set partner board template: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	normalized := normalizePartnerBoardTemplate(template)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig, err := mgr.guildConfigByIDLockedMutable(scope)
	if err != nil {
		return err
	}
	guildConfig.PartnerBoard.Template = normalized

	if err := mgr.saveConfigLocked(); err != nil {
		return fmt.Errorf("set partner board template: save config: %w", err)
	}
	return nil
}

// GetPartnerBoard returns target/template/partners using canonical partner ordering.
func (mgr *ConfigManager) GetPartnerBoard(guildID string) (PartnerBoardConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return PartnerBoardConfig{}, fmt.Errorf("get partner board: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return PartnerBoardConfig{}, err
	}

	target, err := normalizeEmbedUpdateTargetConfig(guildConfig.PartnerBoard.Target)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("get partner board: %w", err)
	}
	partners, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("get partner board: %w", err)
	}

	return PartnerBoardConfig{
		Target:   target,
		Template: normalizePartnerBoardTemplate(guildConfig.PartnerBoard.Template),
		Partners: clonePartnerEntries(partners),
	}, nil
}

// ListPartners lists partner records for a guild in canonical deterministic order.
func (mgr *ConfigManager) ListPartners(guildID string) ([]PartnerEntryConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return nil, fmt.Errorf("list partners: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return nil, err
	}

	partners, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}
	return clonePartnerEntries(partners), nil
}

// GetPartner retrieves one partner by name (case-insensitive).
func (mgr *ConfigManager) GetPartner(guildID, name string) (PartnerEntryConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return PartnerEntryConfig{}, fmt.Errorf("get partner: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	targetName := normalizeNameKey(name)
	if targetName == "" {
		return PartnerEntryConfig{}, fmt.Errorf("get partner: %w", invalidPartnerBoardInput("name is required"))
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return PartnerEntryConfig{}, err
	}

	partners, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return PartnerEntryConfig{}, fmt.Errorf("get partner: %w", err)
	}

	idx := findPartnerIndexByNameKey(partners, targetName)
	if idx < 0 {
		return PartnerEntryConfig{}, fmt.Errorf("%w: name=%s", ErrPartnerNotFound, strings.TrimSpace(name))
	}
	return clonePartnerEntry(partners[idx]), nil
}

// CreatePartner creates a new partner record (dedupe by name/link).
func (mgr *ConfigManager) CreatePartner(guildID string, partner PartnerEntryConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("create partner: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	normalized, err := normalizePartnerEntry(partner)
	if err != nil {
		return fmt.Errorf("create partner: %w", err)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig, err := mgr.guildConfigByIDLockedMutable(scope)
	if err != nil {
		return err
	}

	current, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return fmt.Errorf("create partner: %w", err)
	}

	nameKey := normalizeNameKey(normalized.Name)
	if findPartnerIndexByNameKey(current, nameKey) >= 0 {
		return fmt.Errorf("%w: name=%s", ErrPartnerAlreadyExists, normalized.Name)
	}
	linkKey := normalizeLinkKey(normalized.Link)
	if findPartnerIndexByLinkKey(current, linkKey) >= 0 {
		return fmt.Errorf("%w: link=%s", ErrPartnerAlreadyExists, normalized.Link)
	}

	current = append(current, normalized)
	sortPartnersDeterministically(current)
	guildConfig.PartnerBoard.Partners = current

	if err := mgr.saveConfigLocked(); err != nil {
		return fmt.Errorf("create partner: save config: %w", err)
	}
	return nil
}

// UpdatePartner updates one existing partner selected by current name (case-insensitive).
func (mgr *ConfigManager) UpdatePartner(guildID, currentName string, partner PartnerEntryConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("update partner: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	targetNameKey := normalizeNameKey(currentName)
	if targetNameKey == "" {
		return fmt.Errorf("update partner: %w", invalidPartnerBoardInput("current_name is required"))
	}

	normalized, err := normalizePartnerEntry(partner)
	if err != nil {
		return fmt.Errorf("update partner: %w", err)
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig, err := mgr.guildConfigByIDLockedMutable(scope)
	if err != nil {
		return err
	}

	current, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return fmt.Errorf("update partner: %w", err)
	}

	idx := findPartnerIndexByNameKey(current, targetNameKey)
	if idx < 0 {
		return fmt.Errorf("%w: name=%s", ErrPartnerNotFound, strings.TrimSpace(currentName))
	}

	newNameKey := normalizeNameKey(normalized.Name)
	if dup := findPartnerIndexByNameKey(current, newNameKey); dup >= 0 && dup != idx {
		return fmt.Errorf("%w: name=%s", ErrPartnerAlreadyExists, normalized.Name)
	}
	newLinkKey := normalizeLinkKey(normalized.Link)
	if dup := findPartnerIndexByLinkKey(current, newLinkKey); dup >= 0 && dup != idx {
		return fmt.Errorf("%w: link=%s", ErrPartnerAlreadyExists, normalized.Link)
	}

	current[idx] = normalized
	sortPartnersDeterministically(current)
	guildConfig.PartnerBoard.Partners = current

	if err := mgr.saveConfigLocked(); err != nil {
		return fmt.Errorf("update partner: save config: %w", err)
	}
	return nil
}

// DeletePartner deletes one partner by name (case-insensitive).
func (mgr *ConfigManager) DeletePartner(guildID, name string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("delete partner: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	targetNameKey := normalizeNameKey(name)
	if targetNameKey == "" {
		return fmt.Errorf("delete partner: %w", invalidPartnerBoardInput("name is required"))
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig, err := mgr.guildConfigByIDLockedMutable(scope)
	if err != nil {
		return err
	}

	current, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return fmt.Errorf("delete partner: %w", err)
	}

	idx := findPartnerIndexByNameKey(current, targetNameKey)
	if idx < 0 {
		return fmt.Errorf("%w: name=%s", ErrPartnerNotFound, strings.TrimSpace(name))
	}

	current = slices.Delete(current, idx, idx+1)
	sortPartnersDeterministically(current)
	guildConfig.PartnerBoard.Partners = current

	if err := mgr.saveConfigLocked(); err != nil {
		return fmt.Errorf("delete partner: save config: %w", err)
	}
	return nil
}

func (mgr *ConfigManager) guildConfigByIDLocked(guildID string) (*GuildConfig, error) {
	if mgr == nil || mgr.config == nil {
		return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, strings.TrimSpace(guildID))
	}
	target := strings.TrimSpace(guildID)
	if target == "" {
		return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, strings.TrimSpace(guildID))
	}

	if mgr.guildIndex != nil {
		if idx, ok := mgr.guildIndex[target]; ok {
			if idx >= 0 && idx < len(mgr.config.Guilds) && mgr.config.Guilds[idx].GuildID == target {
				return &mgr.config.Guilds[idx], nil
			}
		}
	}

	for i := range mgr.config.Guilds {
		if mgr.config.Guilds[i].GuildID == target {
			return &mgr.config.Guilds[i], nil
		}
	}
	return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, target)
}

func (mgr *ConfigManager) guildConfigByIDLockedMutable(guildID string) (*GuildConfig, error) {
	if mgr == nil {
		return nil, fmt.Errorf("%w: config manager is nil", ErrInvalidPartnerBoardInput)
	}
	if mgr.config == nil {
		mgr.config = &BotConfig{Guilds: []GuildConfig{}}
	}
	return mgr.guildConfigByIDLocked(guildID)
}

func normalizeEmbedUpdateTargetConfig(in EmbedUpdateTargetConfig) (EmbedUpdateTargetConfig, error) {
	out := EmbedUpdateTargetConfig{
		Type:       strings.ToLower(strings.TrimSpace(in.Type)),
		MessageID:  strings.TrimSpace(in.MessageID),
		ChannelID:  strings.TrimSpace(in.ChannelID),
		WebhookURL: strings.TrimSpace(in.WebhookURL),
	}

	if out.Type == "" && out.MessageID == "" && out.ChannelID == "" && out.WebhookURL == "" {
		return EmbedUpdateTargetConfig{}, nil
	}
	if out.Type == "" {
		if out.WebhookURL != "" {
			out.Type = EmbedUpdateTargetTypeWebhookMessage
		} else if out.ChannelID != "" {
			out.Type = EmbedUpdateTargetTypeChannelMessage
		}
	}

	if out.MessageID == "" {
		return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("message_id is required")
	}
	if !isAllDigits(out.MessageID) {
		return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("message_id must be numeric")
	}

	switch out.Type {
	case EmbedUpdateTargetTypeWebhookMessage:
		if out.WebhookURL == "" {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("webhook_url is required for type=%s", out.Type)
		}
		if err := validateDiscordWebhookURL(out.WebhookURL); err != nil {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("webhook_url is invalid: %v", err)
		}
		out.ChannelID = ""
	case EmbedUpdateTargetTypeChannelMessage:
		if out.ChannelID == "" {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("channel_id is required for type=%s", out.Type)
		}
		if !isAllDigits(out.ChannelID) {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("channel_id must be numeric")
		}
		out.WebhookURL = ""
	default:
		return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput(
			"type is invalid (use %s or %s)",
			EmbedUpdateTargetTypeWebhookMessage,
			EmbedUpdateTargetTypeChannelMessage,
		)
	}

	return out, nil
}

func canonicalizePartnerEntries(entries []PartnerEntryConfig) ([]PartnerEntryConfig, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	normalized := make([]PartnerEntryConfig, 0, len(entries))
	for i, entry := range entries {
		n, err := normalizePartnerEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("partner[%d]: %w", i, err)
		}
		normalized = append(normalized, n)
	}

	sortPartnersDeterministically(normalized)

	seenNames := make(map[string]struct{}, len(normalized))
	seenLinks := make(map[string]struct{}, len(normalized))
	deduped := make([]PartnerEntryConfig, 0, len(normalized))
	for _, item := range normalized {
		nameKey := normalizeNameKey(item.Name)
		if _, exists := seenNames[nameKey]; exists {
			continue
		}
		linkKey := normalizeLinkKey(item.Link)
		if _, exists := seenLinks[linkKey]; exists {
			continue
		}
		seenNames[nameKey] = struct{}{}
		seenLinks[linkKey] = struct{}{}
		deduped = append(deduped, item)
	}

	return deduped, nil
}

func normalizePartnerEntry(in PartnerEntryConfig) (PartnerEntryConfig, error) {
	out := PartnerEntryConfig{
		Fandom: sanitizeSingleLine(in.Fandom),
		Name:   sanitizeSingleLine(in.Name),
	}
	if out.Name == "" {
		return PartnerEntryConfig{}, invalidPartnerBoardInput("name is required")
	}

	link, err := normalizeDiscordInviteURL(in.Link)
	if err != nil {
		return PartnerEntryConfig{}, fmt.Errorf("link: %w", err)
	}
	out.Link = link
	return out, nil
}

func normalizeDiscordInviteURL(raw string) (string, error) {
	raw = sanitizeSingleLine(raw)
	if raw == "" {
		return "", invalidPartnerBoardInput("invite URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", invalidPartnerBoardInput("invalid URL")
	}
	if u == nil {
		return "", invalidPartnerBoardInput("invalid URL")
	}
	if scheme := strings.ToLower(strings.TrimSpace(u.Scheme)); scheme != "http" && scheme != "https" {
		return "", invalidPartnerBoardInput("URL scheme must be http or https")
	}

	code, err := extractDiscordInviteCode(u)
	if err != nil {
		return "", err
	}

	// Canonical persisted format for deterministic comparison/output.
	return "https://discord.gg/" + strings.ToLower(code), nil
}

func extractDiscordInviteCode(u *url.URL) (string, error) {
	if u == nil {
		return "", invalidPartnerBoardInput("invalid URL")
	}

	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return "", invalidPartnerBoardInput("URL host is required")
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) == 0 || strings.TrimSpace(pathParts[0]) == "" {
		return "", invalidPartnerBoardInput("invite code is required")
	}

	var code string
	switch host {
	case "discord.gg", "www.discord.gg":
		code = pathParts[0]
	case "discord.com", "www.discord.com", "ptb.discord.com", "canary.discord.com":
		if len(pathParts) < 2 || pathParts[0] != "invite" {
			return "", invalidPartnerBoardInput("Discord invite URL must match /invite/{code}")
		}
		code = pathParts[1]
	default:
		return "", invalidPartnerBoardInput("URL host must be a Discord invite domain")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return "", invalidPartnerBoardInput("invite code is required")
	}
	for _, r := range code {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' {
			continue
		}
		return "", invalidPartnerBoardInput("invite code has invalid characters")
	}

	return code, nil
}

func sortPartnersDeterministically(entries []PartnerEntryConfig) {
	sort.SliceStable(entries, func(i, j int) bool {
		leftFandom := strings.ToLower(entries[i].Fandom)
		rightFandom := strings.ToLower(entries[j].Fandom)
		if leftFandom != rightFandom {
			return leftFandom < rightFandom
		}

		leftName := strings.ToLower(entries[i].Name)
		rightName := strings.ToLower(entries[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}

		leftLink := strings.ToLower(entries[i].Link)
		rightLink := strings.ToLower(entries[j].Link)
		if leftLink != rightLink {
			return leftLink < rightLink
		}

		if entries[i].Fandom != entries[j].Fandom {
			return entries[i].Fandom < entries[j].Fandom
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Link < entries[j].Link
	})
}

func findPartnerIndexByNameKey(entries []PartnerEntryConfig, nameKey string) int {
	if nameKey == "" {
		return -1
	}
	for i, entry := range entries {
		if normalizeNameKey(entry.Name) == nameKey {
			return i
		}
	}
	return -1
}

func findPartnerIndexByLinkKey(entries []PartnerEntryConfig, linkKey string) int {
	if linkKey == "" {
		return -1
	}
	for i, entry := range entries {
		if normalizeLinkKey(entry.Link) == linkKey {
			return i
		}
	}
	return -1
}

func normalizeNameKey(name string) string {
	return strings.ToLower(sanitizeSingleLine(name))
}

func normalizeLinkKey(link string) string {
	return strings.ToLower(strings.TrimSpace(link))
}

func sanitizeSingleLine(in string) string {
	out := strings.TrimSpace(in)
	out = strings.ReplaceAll(out, "\r\n", " ")
	out = strings.ReplaceAll(out, "\n", " ")
	out = strings.ReplaceAll(out, "\r", " ")
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func clonePartnerEntry(in PartnerEntryConfig) PartnerEntryConfig {
	return PartnerEntryConfig{
		Fandom: strings.TrimSpace(in.Fandom),
		Name:   strings.TrimSpace(in.Name),
		Link:   strings.TrimSpace(in.Link),
	}
}

func clonePartnerEntries(in []PartnerEntryConfig) []PartnerEntryConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]PartnerEntryConfig, 0, len(in))
	for _, item := range in {
		out = append(out, clonePartnerEntry(item))
	}
	return out
}

func normalizePartnerBoardTemplate(in PartnerBoardTemplateConfig) PartnerBoardTemplateConfig {
	return PartnerBoardTemplateConfig{
		Title:                      sanitizeSingleLine(in.Title),
		ContinuationTitle:          sanitizeSingleLine(in.ContinuationTitle),
		Intro:                      strings.TrimSpace(in.Intro),
		SectionHeaderTemplate:      strings.TrimSpace(in.SectionHeaderTemplate),
		SectionContinuationSuffix:  strings.TrimSpace(in.SectionContinuationSuffix),
		SectionContinuationPattern: strings.TrimSpace(in.SectionContinuationPattern),
		LineTemplate:               strings.TrimSpace(in.LineTemplate),
		EmptyStateText:             strings.TrimSpace(in.EmptyStateText),
		FooterTemplate:             strings.TrimSpace(in.FooterTemplate),
		OtherFandomLabel:           sanitizeSingleLine(in.OtherFandomLabel),
		Color:                      in.Color,
		DisableFandomSorting:       in.DisableFandomSorting,
		DisablePartnerSorting:      in.DisablePartnerSorting,
	}
}

func invalidPartnerBoardInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPartnerBoardInput, fmt.Sprintf(format, args...))
}
