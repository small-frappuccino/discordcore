package main

import (
	
	"io/ioutil"
	
	"strings"
)

func main() {
	content, err := ioutil.ReadFile("pkg/discord/logging/stats_channels.go")
	if err != nil {
		panic(err)
	}

	s := string(content)

	// Update imports
	s = strings.Replace(s, "\"github.com/small-frappuccino/discordcore/pkg/log\"", "\"log/slog\"\n\t\"github.com/small-frappuccino/discordcore/pkg/log\"", 1)

	// Replace struct
	s = strings.Replace(s, `type statsCoordinator struct {
	actorCh chan func()
	guilds  map[string]*statsGuildState
	lastRun map[string]time.Time
}`, `type StatsService struct {
	session              *discordgo.Session
	configManager        *files.ConfigManager
	store                *storage.Store
	logger               *slog.Logger
	botInstanceID        string
	defaultBotInstanceID string

	actorCh chan func()
	guilds  map[string]*statsGuildState
	lastRun map[string]time.Time

	currentRunCtx func() context.Context
	getHeartbeat func(context.Context) (time.Time, bool, error)
	fetchMembers func(context.Context, string, func([]*discordgo.Member) error) (int, error)
}`, 1)

	// Replace constructor
	s = strings.Replace(s, `func newStatsCoordinator() *statsCoordinator {
	return &statsCoordinator{
		actorCh: make(chan func(), 1024),
		guilds:  make(map[string]*statsGuildState),
		lastRun: make(map[string]time.Time),
	}
}`, `func NewStatsService(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	store *storage.Store,
	logger *slog.Logger,
	botInstanceID string,
	defaultBotInstanceID string,
	currentRunCtx func() context.Context,
	getHeartbeat func(context.Context) (time.Time, bool, error),
	fetchMembers func(context.Context, string, func([]*discordgo.Member) error) (int, error),
) *StatsService {
	return &StatsService{
		session:              session,
		configManager:        configManager,
		store:                store,
		logger:               logger,
		botInstanceID:        botInstanceID,
		defaultBotInstanceID: defaultBotInstanceID,
		actorCh:              make(chan func(), 1024),
		guilds:               make(map[string]*statsGuildState),
		lastRun:              make(map[string]time.Time),
		currentRunCtx:        currentRunCtx,
		getHeartbeat:         getHeartbeat,
		fetchMembers:         fetchMembers,
	}
}

func (s *StatsService) Start(ctx context.Context) error {
	go s.loop(ctx)
	return nil
}

func (s *StatsService) Stop(ctx context.Context) error {
	return nil
}

func (s *StatsService) handlesGuild(guildID string) bool {
	cfg := s.scopedConfig()
	if cfg == nil {
		return false
	}
	return cfg.GuildConfig(guildID) != nil
}

func (s *StatsService) scopedConfig() *files.BotConfig {
	if s.configManager != nil {
		return s.configManager.BotConfig(s.botInstanceID)
	}
	return nil
}`, 1)

	// Rename receivers
	s = strings.ReplaceAll(s, "(sc *statsCoordinator)", "(s *StatsService)")
	s = strings.ReplaceAll(s, "(ms *MonitoringService)", "(s *StatsService)")

	// Rename internal ms accesses
	s = strings.ReplaceAll(s, "ms.stats.", "s.")
	s = strings.ReplaceAll(s, "ms.", "s.")

	// Rename s.forEachGuildMemberPageContext to s.fetchMembers
	s = strings.ReplaceAll(s, "s.forEachGuildMemberPageContext", "s.fetchMembers")

	// Fix shadowing and undefined variable errors
	s = strings.ReplaceAll(s, "handleStatsMemberAdd(s *discordgo.Session", "HandleStatsMemberAdd(_ *discordgo.Session")
	s = strings.ReplaceAll(s, "handleStatsMemberRemove(s *discordgo.Session", "HandleStatsMemberRemove(_ *discordgo.Session")
	s = strings.ReplaceAll(s, "updateStatsChannels(", "UpdateStatsChannels(")
	s = strings.ReplaceAll(s, "applyStatsMemberUpdate(", "ApplyStatsMemberUpdate(")

	// Fix ms == nil null checks (since ms is now s)
	s = strings.ReplaceAll(s, "ms == nil", "s == nil")

	err = ioutil.WriteFile("pkg/discord/logging/stats_channels.go", []byte(s), 0644)
	if err != nil {
		panic(err)
	}
}
