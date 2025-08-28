package discordcore

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

// Event defines a generic interface for events that can be monitored.
type Event interface {
	GetGuildID() string
	GetUserID() string
	GetEventType() string
	GetData() map[string]interface{}
	GetTimestamp() time.Time
}

// EventProcessor defines an interface for processing events.
type EventProcessor interface {
	ProcessEvent(event Event)
	Start()
	Stop()
}

// EventHandler is a function type for handling specific event types.
type EventHandler func(event Event)

// SlashCommand defines a generic interface for slash commands.
type SlashCommand interface {
	GetName() string
	GetDescription() string
	Execute(session *discordgo.Session, interaction *discordgo.InteractionCreate) error
}

// SlashCommandHandler is a function type for handling slash command executions.
type SlashCommandHandler func(session *discordgo.Session, interaction *discordgo.InteractionCreate) error

// SlashCommandManager manages registration and handling of slash commands.
type SlashCommandManager struct {
	commands map[string]SlashCommand
	session  *discordgo.Session
	mu       sync.RWMutex
}

// NewSlashCommandManager creates a new slash command manager.
func NewSlashCommandManager(session *discordgo.Session) *SlashCommandManager {
	return &SlashCommandManager{
		commands: make(map[string]SlashCommand),
		session:  session,
	}
}

// RegisterCommand registers a slash command with Discord.
func (scm *SlashCommandManager) RegisterCommand(command SlashCommand) error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	scm.commands[command.GetName()] = command

	// Register with Discord
	appCmd := &discordgo.ApplicationCommand{
		Name:        command.GetName(),
		Description: command.GetDescription(),
		Type:        discordgo.ChatApplicationCommand,
	}

	_, err := scm.session.ApplicationCommandCreate(scm.session.State.User.ID, "", appCmd)
	if err != nil {
		logutil.Errorf("Failed to register slash command %s: %v", command.GetName(), err)
		return err
	}

	logutil.Infof("Registered slash command: %s", command.GetName())
	return nil
}

// RegisterCommandWithOptions registers a slash command with options.
func (scm *SlashCommandManager) RegisterCommandWithOptions(command SlashCommand, options []*discordgo.ApplicationCommandOption) error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	scm.commands[command.GetName()] = command

	// Register with Discord
	appCmd := &discordgo.ApplicationCommand{
		Name:        command.GetName(),
		Description: command.GetDescription(),
		Options:     options,
		Type:        discordgo.ChatApplicationCommand,
	}

	_, err := scm.session.ApplicationCommandCreate(scm.session.State.User.ID, "", appCmd)
	if err != nil {
		logutil.Errorf("Failed to register slash command with options %s: %v", command.GetName(), err)
		return err
	}

	logutil.Infof("Registered slash command with options: %s", command.GetName())
	return nil
}

// HandleInteraction processes slash command interactions.
func (scm *SlashCommandManager) HandleInteraction(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}

	scm.mu.RLock()
	command, exists := scm.commands[interaction.ApplicationCommandData().Name]
	scm.mu.RUnlock()

	if !exists {
		logutil.Warnf("Unknown slash command received: %s", interaction.ApplicationCommandData().Name)
		return
	}

	err := command.Execute(session, interaction)
	if err != nil {
		logutil.Errorf("Failed to execute slash command %s: %v", command.GetName(), err)

		// Try to respond with error message
		responseErr := session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Ocorreu um erro ao executar este comando.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if responseErr != nil {
			logutil.Errorf("Failed to send error response: %v", responseErr)
		}
	}
}

// Start registers the interaction handler.
func (scm *SlashCommandManager) Start() {
	scm.session.AddHandler(scm.HandleInteraction)
	logutil.Info("Slash command manager started")
}

// Stop is a no-op for slash commands.
func (scm *SlashCommandManager) Stop() {
	// Slash commands don't need explicit stopping
	logutil.Info("Slash command manager stopped")
}

// GetRegisteredCommands returns a list of registered command names.
func (scm *SlashCommandManager) GetRegisteredCommands() []string {
	scm.mu.RLock()
	defer scm.mu.RUnlock()

	var commands []string
	for name := range scm.commands {
		commands = append(commands, name)
	}
	return commands
}

// UnregisterCommand removes a slash command from Discord.
func (scm *SlashCommandManager) UnregisterCommand(commandName string) error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	delete(scm.commands, commandName)

	// Get all registered commands from Discord
	commands, err := scm.session.ApplicationCommands(scm.session.State.User.ID, "")
	if err != nil {
		return err
	}

	// Find and delete the specific command
	for _, cmd := range commands {
		if cmd.Name == commandName {
			err = scm.session.ApplicationCommandDelete(scm.session.State.User.ID, "", cmd.ID)
			if err != nil {
				logutil.Errorf("Failed to unregister slash command %s: %v", commandName, err)
				return err
			}
			logutil.Infof("Unregistered slash command: %s", commandName)
			return nil
		}
	}

	return fmt.Errorf("command %s not found", commandName)
}

// MonitoringService provides a generic monitoring service that can handle various types of events.
type MonitoringService struct {
	processors    []EventProcessor
	eventHandlers map[string][]EventHandler
	isRunning     bool
	stopChan      chan struct{}
	runMu         sync.RWMutex
	handlerMu     sync.RWMutex
}

// NewMonitoringService creates a new generic monitoring service.
func NewMonitoringService() *MonitoringService {
	return &MonitoringService{
		processors:    []EventProcessor{},
		eventHandlers: make(map[string][]EventHandler),
		stopChan:      make(chan struct{}),
	}
}

// AddProcessor adds an event processor to the service.
func (ms *MonitoringService) AddProcessor(processor EventProcessor) {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	ms.processors = append(ms.processors, processor)
}

// RegisterEventHandler registers a handler for a specific event type.
func (ms *MonitoringService) RegisterEventHandler(eventType string, handler EventHandler) {
	ms.handlerMu.Lock()
	defer ms.handlerMu.Unlock()
	ms.eventHandlers[eventType] = append(ms.eventHandlers[eventType], handler)
}

// HandleEvent processes an event by calling all registered handlers and processors.
func (ms *MonitoringService) HandleEvent(event Event) {
	// Call type-specific handlers
	ms.handlerMu.RLock()
	handlers, exists := ms.eventHandlers[event.GetEventType()]
	ms.handlerMu.RUnlock()

	if exists {
		for _, handler := range handlers {
			handler(event)
		}
	}

	// Forward to processors
	for _, processor := range ms.processors {
		processor.ProcessEvent(event)
	}
}

// Start starts the monitoring service and all its processors.
func (ms *MonitoringService) Start() error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	if ms.isRunning {
		return fmt.Errorf("monitoring service is already running")
	}
	ms.isRunning = true
	ms.stopChan = make(chan struct{})

	for _, processor := range ms.processors {
		processor.Start()
	}
	return nil
}

// Stop stops the monitoring service and all its processors.
func (ms *MonitoringService) Stop() error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	if !ms.isRunning {
		return fmt.Errorf("monitoring service is not running")
	}
	ms.isRunning = false
	close(ms.stopChan)

	for _, processor := range ms.processors {
		processor.Stop()
	}
	return nil
}

// PresenceEvent represents a presence update event with extracted data.
type PresenceEvent struct {
	GuildID    string
	UserID     string
	Username   string
	Avatar     string
	Status     string
	Activities []*discordgo.Activity
	Timestamp  time.Time
}

func (p *PresenceEvent) GetGuildID() string {
	return p.GuildID
}

func (p *PresenceEvent) GetUserID() string {
	return p.UserID
}

func (p *PresenceEvent) GetEventType() string {
	return "presence_update"
}

func (p *PresenceEvent) GetData() map[string]interface{} {
	return map[string]interface{}{
		"username":   p.Username,
		"avatar":     p.Avatar,
		"status":     p.Status,
		"activities": p.Activities,
	}
}

func (p *PresenceEvent) GetTimestamp() time.Time {
	return p.Timestamp
}

// GuildMemberAddEvent represents a user joining a guild event.
type GuildMemberAddEvent struct {
	GuildID        string
	UserID         string
	Username       string
	Discriminator  string
	Avatar         string
	JoinedAt       time.Time
	AccountCreated time.Time
	Timestamp      time.Time
}

func (g *GuildMemberAddEvent) GetGuildID() string {
	return g.GuildID
}

func (g *GuildMemberAddEvent) GetUserID() string {
	return g.UserID
}

func (g *GuildMemberAddEvent) GetEventType() string {
	return "guild_member_add"
}

func (g *GuildMemberAddEvent) GetData() map[string]interface{} {
	return map[string]interface{}{
		"username":         g.Username,
		"discriminator":    g.Discriminator,
		"avatar":           g.Avatar,
		"joined_at":        g.JoinedAt,
		"account_created":  g.AccountCreated,
		"account_age_days": time.Since(g.AccountCreated).Hours() / 24,
	}
}

func (g *GuildMemberAddEvent) GetTimestamp() time.Time {
	return g.Timestamp
}

// GuildMemberRemoveEvent represents a user leaving a guild event.
type GuildMemberRemoveEvent struct {
	GuildID       string
	UserID        string
	Username      string
	Discriminator string
	JoinedAt      time.Time
	LeftAt        time.Time
	TimeInGuild   time.Duration
	Timestamp     time.Time
}

func (g *GuildMemberRemoveEvent) GetGuildID() string {
	return g.GuildID
}

func (g *GuildMemberRemoveEvent) GetUserID() string {
	return g.UserID
}

func (g *GuildMemberRemoveEvent) GetEventType() string {
	return "guild_member_remove"
}

func (g *GuildMemberRemoveEvent) GetData() map[string]interface{} {
	return map[string]interface{}{
		"username":              g.Username,
		"discriminator":         g.Discriminator,
		"joined_at":             g.JoinedAt,
		"left_at":               g.LeftAt,
		"time_in_guild":         g.TimeInGuild.String(),
		"time_in_guild_days":    g.TimeInGuild.Hours() / 24,
		"time_in_guild_hours":   g.TimeInGuild.Hours(),
		"time_in_guild_minutes": g.TimeInGuild.Minutes(),
	}
}

func (g *GuildMemberRemoveEvent) GetTimestamp() time.Time {
	return g.Timestamp
}

// GuildMemberUpdateEvent represents a guild member update event.
type GuildMemberUpdateEvent struct {
	GuildID     string
	UserID      string
	Username    string
	Nickname    string
	Roles       []string
	JoinedAt    time.Time
	TimeInGuild time.Duration
	Timestamp   time.Time
}

func (g *GuildMemberUpdateEvent) GetGuildID() string {
	return g.GuildID
}

func (g *GuildMemberUpdateEvent) GetUserID() string {
	return g.UserID
}

func (g *GuildMemberUpdateEvent) GetEventType() string {
	return "guild_member_update"
}

func (g *GuildMemberUpdateEvent) GetData() map[string]interface{} {
	return map[string]interface{}{
		"username":      g.Username,
		"nickname":      g.Nickname,
		"roles":         g.Roles,
		"joined_at":     g.JoinedAt,
		"time_in_guild": g.TimeInGuild.String(),
	}
}

func (g *GuildMemberUpdateEvent) GetTimestamp() time.Time {
	return g.Timestamp
}

// AvatarChangeEvent represents an avatar change event.
type AvatarChangeEvent struct {
	GuildID     string
	UserID      string
	Username    string
	OldAvatar   string
	NewAvatar   string
	AvatarURL   string
	ChangedAt   time.Time
	TimeInGuild time.Duration
	Timestamp   time.Time
}

func (a *AvatarChangeEvent) GetGuildID() string {
	return a.GuildID
}

func (a *AvatarChangeEvent) GetUserID() string {
	return a.UserID
}

func (a *AvatarChangeEvent) GetEventType() string {
	return "avatar_change"
}

func (a *AvatarChangeEvent) GetData() map[string]interface{} {
	return map[string]interface{}{
		"username":      a.Username,
		"old_avatar":    a.OldAvatar,
		"new_avatar":    a.NewAvatar,
		"avatar_url":    a.AvatarURL,
		"changed_at":    a.ChangedAt,
		"time_in_guild": a.TimeInGuild.String(),
	}
}

func (a *AvatarChangeEvent) GetTimestamp() time.Time {
	return a.Timestamp
}

// DiscordEventAdapter adapts Discord events to the generic Event interface.
type DiscordEventAdapter struct {
	monitoringService  *MonitoringService
	session            *discordgo.Session
	configManager      *ConfigManager
	avatarCacheManager *AvatarCacheManager
}

// NewDiscordEventAdapter creates a new Discord event adapter.
func NewDiscordEventAdapter(session *discordgo.Session, configManager *ConfigManager, monitoringService *MonitoringService, avatarCacheManager *AvatarCacheManager) *DiscordEventAdapter {
	return &DiscordEventAdapter{
		monitoringService:  monitoringService,
		session:            session,
		configManager:      configManager,
		avatarCacheManager: avatarCacheManager,
	}
}

// ProcessEvent implements EventProcessor interface.
func (dea *DiscordEventAdapter) ProcessEvent(event Event) {
	// This adapter is for receiving Discord events, not processing generic events
	// So this method is a no-op for the EventProcessor interface
}

// Start registers Discord event handlers and automatically detects guilds.
func (dea *DiscordEventAdapter) Start() {
	// Automatically detect and configure guilds during initialization
	if err := dea.configManager.detectGuilds(dea.session); err != nil {
		logutil.Errorf("Failed to auto-detect guilds during initialization: %v", err)
	} else {
		logutil.Info("✅ Guilds auto-detected and configured successfully")
	}

	dea.session.AddHandler(dea.handlePresenceUpdate)
	dea.session.AddHandler(dea.handleGuildMemberAdd)
	dea.session.AddHandler(dea.handleGuildMemberRemove)
	dea.session.AddHandler(dea.handleGuildMemberUpdate)

	// Only add avatar change handler if avatar cache manager is available
	if dea.avatarCacheManager != nil {
		dea.session.AddHandler(dea.handleAvatarChange)
	}
}

// Stop removes Discord event handlers.
// Note: discordgo doesn't provide a direct way to remove handlers, so this is a no-op.
func (dea *DiscordEventAdapter) Stop() {
	// Discord event handlers cannot be easily removed once added
}

// handlePresenceUpdate processes presence updates and forwards them to the monitoring service.
func (dea *DiscordEventAdapter) handlePresenceUpdate(s *discordgo.Session, m *discordgo.PresenceUpdate) {
	if m.User == nil || m.GuildID == "" {
		return
	}
	if dea.configManager.GuildConfig(m.GuildID) == nil {
		return
	}
	if m.User.Username == "" {
		logutil.WithFields(map[string]interface{}{"userID": m.User.ID, "guildID": m.GuildID}).Debug("PresenceUpdate ignored (empty username)")
		return
	}

	event := &PresenceEvent{
		GuildID:    m.GuildID,
		UserID:     m.User.ID,
		Username:   m.User.Username,
		Avatar:     m.User.Avatar,
		Status:     string(m.Status),
		Activities: m.Activities,
		Timestamp:  time.Now(),
	}

	dea.monitoringService.HandleEvent(event)
}

// handleGuildMemberAdd processes guild member add events.
func (dea *DiscordEventAdapter) handleGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.User == nil || m.GuildID == "" {
		return
	}
	if dea.configManager.GuildConfig(m.GuildID) == nil {
		return
	}

	// Calculate account creation time from Discord snowflake ID
	var accountCreated time.Time
	if userID, err := strconv.ParseInt(m.User.ID, 10, 64); err == nil {
		accountCreated = time.UnixMilli((userID >> 22) + 1420070400000)
	}

	event := &GuildMemberAddEvent{
		GuildID:        m.GuildID,
		UserID:         m.User.ID,
		Username:       m.User.Username,
		Discriminator:  m.User.Discriminator,
		Avatar:         m.User.Avatar,
		JoinedAt:       m.JoinedAt,
		AccountCreated: accountCreated,
		Timestamp:      time.Now(),
	}

	dea.monitoringService.HandleEvent(event)
}

// handleGuildMemberRemove processes guild member remove events.
func (dea *DiscordEventAdapter) handleGuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.User == nil || m.GuildID == "" {
		return
	}
	if dea.configManager.GuildConfig(m.GuildID) == nil {
		return
	}

	// Note: discordgo.GuildMemberRemove doesn't include JoinedAt
	// In a real implementation, you'd want to store this information
	// when the user joins, so you can calculate time in guild
	var joinedAt time.Time
	var timeInGuild time.Duration

	// This is a simplified version - you'd want to implement proper storage
	// of join times in a real application
	if !joinedAt.IsZero() {
		timeInGuild = time.Since(joinedAt)
	}

	event := &GuildMemberRemoveEvent{
		GuildID:     m.GuildID,
		UserID:      m.User.ID,
		Username:    m.User.Username,
		JoinedAt:    joinedAt,
		LeftAt:      time.Now(),
		TimeInGuild: timeInGuild,
		Timestamp:   time.Now(),
	}

	dea.monitoringService.HandleEvent(event)
}

// handleGuildMemberUpdate processes guild member update events.
func (dea *DiscordEventAdapter) handleGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.User == nil || m.GuildID == "" {
		return
	}
	if dea.configManager.GuildConfig(m.GuildID) == nil {
		return
	}

	var timeInGuild time.Duration
	if !m.JoinedAt.IsZero() {
		timeInGuild = time.Since(m.JoinedAt)
	}

	// Extract role IDs
	var roles []string
	if m.Roles != nil {
		roles = m.Roles
	}

	event := &GuildMemberUpdateEvent{
		GuildID:     m.GuildID,
		UserID:      m.User.ID,
		Username:    m.User.Username,
		Nickname:    m.Nick,
		Roles:       roles,
		JoinedAt:    m.JoinedAt,
		TimeInGuild: timeInGuild,
		Timestamp:   time.Now(),
	}

	dea.monitoringService.HandleEvent(event)
}

// handleAvatarChange processes avatar change events by comparing with cached avatars.
func (dea *DiscordEventAdapter) handleAvatarChange(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.User == nil || m.GuildID == "" || dea.avatarCacheManager == nil {
		return
	}
	if dea.configManager.GuildConfig(m.GuildID) == nil {
		return
	}

	// Check if avatar actually changed
	currentAvatar := m.User.Avatar
	if !dea.avatarCacheManager.AvatarChanged(m.GuildID, m.User.ID, currentAvatar) {
		return // No change detected
	}

	// Get the old avatar from cache
	oldAvatar := dea.avatarCacheManager.AvatarHash(m.GuildID, m.User.ID)

	// Update cache with new avatar
	dea.avatarCacheManager.UpdateAvatar(m.GuildID, m.User.ID, currentAvatar)

	// Calculate time in guild
	var timeInGuild time.Duration
	if !m.JoinedAt.IsZero() {
		timeInGuild = time.Since(m.JoinedAt)
	}

	// Generate avatar URL if avatar exists
	var avatarURL string
	if currentAvatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", m.User.ID, currentAvatar)
	}

	event := &AvatarChangeEvent{
		GuildID:     m.GuildID,
		UserID:      m.User.ID,
		Username:    m.User.Username,
		OldAvatar:   oldAvatar,
		NewAvatar:   currentAvatar,
		AvatarURL:   avatarURL,
		ChangedAt:   time.Now(),
		TimeInGuild: timeInGuild,
		Timestamp:   time.Now(),
	}

	dea.monitoringService.HandleEvent(event)
}

// MonitorableEvent is deprecated: use Event instead.
type MonitorableEvent = Event

// Monitor is deprecated: use EventProcessor instead.
type Monitor = EventProcessor

// CoreMonitoringService is deprecated: use MonitoringService with DiscordEventAdapter instead.
type CoreMonitoringService struct {
	*MonitoringService
	discordAdapter *DiscordEventAdapter
}

// NewCoreMonitoringService creates a new core monitoring service (deprecated).
func NewCoreMonitoringService(session *discordgo.Session, configManager *ConfigManager) *CoreMonitoringService {
	ms := NewMonitoringService()
	adapter := NewDiscordEventAdapter(session, configManager, ms, nil)
	ms.AddProcessor(adapter)
	return &CoreMonitoringService{
		MonitoringService: ms,
		discordAdapter:    adapter,
	}
}

// AddMonitor is deprecated: use AddProcessor instead.
func (cms *CoreMonitoringService) AddMonitor(monitor Monitor) {
	cms.AddProcessor(monitor)
}

// setupEventHandlers is deprecated: handled by DiscordEventAdapter.
func (cms *CoreMonitoringService) setupEventHandlers() {
	// Handled by DiscordEventAdapter
}
