package partner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

const (
	optionFandom      = "fandom"
	optionName        = "name"
	optionLink        = "link"
	optionCurrentName = "current_name"
)

// PartnerCommands wires the /partner command group into the command router.
type PartnerCommands struct {
	boardService partners.BoardService
	syncExecutor partners.GuildSyncExecutor
}

func NewPartnerCommands(configManager *files.ConfigManager) *PartnerCommands {
	return NewPartnerCommandsWithServices(
		partners.NewBoardApplicationService(configManager, nil),
		nil,
	)
}

func NewPartnerCommandsWithService(boardService partners.BoardService) *PartnerCommands {
	return NewPartnerCommandsWithServices(boardService, nil)
}

func NewPartnerCommandsWithServices(
	boardService partners.BoardService,
	syncExecutor partners.GuildSyncExecutor,
) *PartnerCommands {
	return &PartnerCommands{
		boardService: boardService,
		syncExecutor: syncExecutor,
	}
}

func (pc *PartnerCommands) RegisterCommands(router *core.CommandRouter) {
	if router == nil || pc == nil || pc.boardService == nil {
		return
	}
	if pc.syncExecutor == nil {
		configManager := router.GetConfigManager()
		session := router.GetSession()
		if configManager != nil && session != nil {
			syncService := partners.NewBoardSyncService(configManager)
			pc.syncExecutor = partners.NewSessionBoundBoardSyncExecutor(syncService, session)
		}
	}

	group := core.NewGroupCommand(
		"partner",
		"Manage partner board records",
		core.NewPermissionChecker(router.GetSession(), router.GetConfigManager()),
	)
	group.AddSubCommand(NewPartnerAddSubCommand(pc.boardService))
	group.AddSubCommand(NewPartnerReadSubCommand(pc.boardService))
	group.AddSubCommand(NewPartnerUpdateSubCommand(pc.boardService))
	group.AddSubCommand(NewPartnerDeleteSubCommand(pc.boardService))
	group.AddSubCommand(NewPartnerListSubCommand(pc.boardService))
	group.AddSubCommand(NewPartnerSyncSubCommand(pc.syncExecutor))

	router.RegisterCommand(group)
}

type PartnerAddSubCommand struct {
	boardService partners.BoardService
}

func NewPartnerAddSubCommand(boardService partners.BoardService) *PartnerAddSubCommand {
	return &PartnerAddSubCommand{boardService: boardService}
}

func (c *PartnerAddSubCommand) Name() string { return "add" }
func (c *PartnerAddSubCommand) Description() string {
	return "Add one partner server record"
}
func (c *PartnerAddSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionName,
			Description: "Partner server name",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionLink,
			Description: "Discord invite URL",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionFandom,
			Description: "Fandom/group label (example: Genshin Impact)",
			Required:    false,
		},
	}
}
func (c *PartnerAddSubCommand) RequiresGuild() bool       { return true }
func (c *PartnerAddSubCommand) RequiresPermissions() bool { return true }
func (c *PartnerAddSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}
	link, err := extractor.StringRequired(optionLink)
	if err != nil {
		return err
	}

	entry := files.PartnerEntryConfig{
		Fandom: extractor.String(optionFandom),
		Name:   name,
		Link:   link,
	}
	if err := c.boardService.CreatePartner(ctx.GuildID, entry); err != nil {
		if errors.Is(err, files.ErrPartnerAlreadyExists) {
			return core.NewCommandError("Partner with same name or invite already exists", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to create partner: %v", err), true)
	}

	saved, err := c.boardService.Partner(ctx.GuildID, name)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Partner created but lookup failed: %v", err), true)
	}

	content := formatPartnerEntry("Partner added", saved)
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, content)
}

type PartnerReadSubCommand struct {
	boardService partners.BoardService
}

func NewPartnerReadSubCommand(boardService partners.BoardService) *PartnerReadSubCommand {
	return &PartnerReadSubCommand{boardService: boardService}
}

func (c *PartnerReadSubCommand) Name() string { return "read" }
func (c *PartnerReadSubCommand) Description() string {
	return "Read one partner server record"
}
func (c *PartnerReadSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionName,
			Description: "Partner server name",
			Required:    true,
		},
	}
}
func (c *PartnerReadSubCommand) RequiresGuild() bool       { return true }
func (c *PartnerReadSubCommand) RequiresPermissions() bool { return true }
func (c *PartnerReadSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}

	entry, err := c.boardService.Partner(ctx.GuildID, name)
	if err != nil {
		if errors.Is(err, files.ErrPartnerNotFound) {
			return core.NewCommandError("Partner not found", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to read partner: %v", err), true)
	}

	content := formatPartnerEntry("Partner details", entry)
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, content)
}

type PartnerUpdateSubCommand struct {
	boardService partners.BoardService
}

func NewPartnerUpdateSubCommand(boardService partners.BoardService) *PartnerUpdateSubCommand {
	return &PartnerUpdateSubCommand{boardService: boardService}
}

func (c *PartnerUpdateSubCommand) Name() string { return "update" }
func (c *PartnerUpdateSubCommand) Description() string {
	return "Update one partner server record"
}
func (c *PartnerUpdateSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionCurrentName,
			Description: "Current partner server name",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionName,
			Description: "New partner server name",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionLink,
			Description: "New Discord invite URL",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionFandom,
			Description: "Fandom/group label (example: Genshin Impact)",
			Required:    false,
		},
	}
}
func (c *PartnerUpdateSubCommand) RequiresGuild() bool       { return true }
func (c *PartnerUpdateSubCommand) RequiresPermissions() bool { return true }
func (c *PartnerUpdateSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	currentName, err := extractor.StringRequired(optionCurrentName)
	if err != nil {
		return err
	}
	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}
	link, err := extractor.StringRequired(optionLink)
	if err != nil {
		return err
	}

	existing, err := c.boardService.Partner(ctx.GuildID, currentName)
	if err != nil {
		if errors.Is(err, files.ErrPartnerNotFound) {
			return core.NewCommandError("Partner not found", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to load current partner: %v", err), true)
	}

	fandom := extractor.String(optionFandom)
	if !extractor.HasOption(optionFandom) {
		fandom = existing.Fandom
	}

	entry := files.PartnerEntryConfig{
		Fandom: fandom,
		Name:   name,
		Link:   link,
	}
	if err := c.boardService.UpdatePartner(ctx.GuildID, currentName, entry); err != nil {
		if errors.Is(err, files.ErrPartnerAlreadyExists) {
			return core.NewCommandError("Another partner already uses this name or invite", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to update partner: %v", err), true)
	}

	saved, err := c.boardService.Partner(ctx.GuildID, name)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Partner updated but lookup failed: %v", err), true)
	}

	content := formatPartnerEntry("Partner updated", saved)
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, content)
}

type PartnerDeleteSubCommand struct {
	boardService partners.BoardService
}

func NewPartnerDeleteSubCommand(boardService partners.BoardService) *PartnerDeleteSubCommand {
	return &PartnerDeleteSubCommand{boardService: boardService}
}

func (c *PartnerDeleteSubCommand) Name() string { return "delete" }
func (c *PartnerDeleteSubCommand) Description() string {
	return "Delete one partner server record"
}
func (c *PartnerDeleteSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionName,
			Description: "Partner server name",
			Required:    true,
		},
	}
}
func (c *PartnerDeleteSubCommand) RequiresGuild() bool       { return true }
func (c *PartnerDeleteSubCommand) RequiresPermissions() bool { return true }
func (c *PartnerDeleteSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}

	if err := c.boardService.DeletePartner(ctx.GuildID, name); err != nil {
		if errors.Is(err, files.ErrPartnerNotFound) {
			return core.NewCommandError("Partner not found", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to delete partner: %v", err), true)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(
		ctx.Interaction,
		fmt.Sprintf("Partner `%s` deleted", strings.TrimSpace(name)),
	)
}

type PartnerListSubCommand struct {
	boardService partners.BoardService
}

func NewPartnerListSubCommand(boardService partners.BoardService) *PartnerListSubCommand {
	return &PartnerListSubCommand{boardService: boardService}
}

func (c *PartnerListSubCommand) Name() string { return "list" }
func (c *PartnerListSubCommand) Description() string {
	return "List partner server records"
}
func (c *PartnerListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}
func (c *PartnerListSubCommand) RequiresGuild() bool       { return true }
func (c *PartnerListSubCommand) RequiresPermissions() bool { return true }
func (c *PartnerListSubCommand) Handle(ctx *core.Context) error {
	partners, err := c.boardService.ListPartners(ctx.GuildID)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to list partners: %v", err), true)
	}
	if len(partners) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, "No partners configured.")
	}

	var b strings.Builder
	b.WriteString("Configured partners:\n")
	for i, p := range partners {
		fandom := strings.TrimSpace(p.Fandom)
		if fandom == "" {
			fandom = "Other"
		}
		b.WriteString(fmt.Sprintf(
			"%d. `%s` | `%s` | %s\n",
			i+1,
			p.Name,
			fandom,
			p.Link,
		))
	}

	builder := core.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Partner List").
		WithColor(theme.Info()).
		Ephemeral()
	return builder.Info(ctx.Interaction, strings.TrimSpace(b.String()))
}

func formatPartnerEntry(prefix string, entry files.PartnerEntryConfig) string {
	fandom := strings.TrimSpace(entry.Fandom)
	if fandom == "" {
		fandom = "Other"
	}
	return strings.Join([]string{
		prefix,
		fmt.Sprintf("Name: `%s`", strings.TrimSpace(entry.Name)),
		fmt.Sprintf("Fandom: `%s`", fandom),
		fmt.Sprintf("Invite: %s", strings.TrimSpace(entry.Link)),
	}, "\n")
}

type PartnerSyncSubCommand struct {
	syncExecutor partners.GuildSyncExecutor
}

func NewPartnerSyncSubCommand(syncExecutor partners.GuildSyncExecutor) *PartnerSyncSubCommand {
	return &PartnerSyncSubCommand{syncExecutor: syncExecutor}
}

func (c *PartnerSyncSubCommand) Name() string { return "sync" }
func (c *PartnerSyncSubCommand) Description() string {
	return "Render and publish partner board to configured target"
}
func (c *PartnerSyncSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}
func (c *PartnerSyncSubCommand) RequiresGuild() bool       { return true }
func (c *PartnerSyncSubCommand) RequiresPermissions() bool { return true }
func (c *PartnerSyncSubCommand) Handle(ctx *core.Context) error {
	if c.syncExecutor == nil {
		return core.NewCommandError("Partner sync is not configured", true)
	}

	syncCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := c.syncExecutor.SyncGuild(syncCtx, ctx.GuildID); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to sync partner board: %v", err), true)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(
		ctx.Interaction,
		"Partner board synced successfully.",
	)
}
