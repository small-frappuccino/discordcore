package files

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

// log.GenerateRequestID cria um identificador único transiente para correlação de erros.
func GenerateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(bytes)
}

// log.EmitBlockingError encapsula a emissão de falhas estruturais com metadados obrigatórios.
func EmitBlockingError(msg string, err error, requestID string) {
	slog.Error(msg,
		slog.String("request_id", requestID),
		slog.String("synthetic_code", "500"),
		slog.String("stack_trace", string(debug.Stack())),
		slog.Any("error", err),
	)
}

// --- Inicialização e Persistência ---

// NewConfigManagerWithStore instancia um novo gerenciador de configuração apoiado pela
// camada de persistência fornecida.
func NewConfigManagerWithStore(store ConfigStore) *ConfigManager {
	description := ""
	if store != nil {
		description = store.Describe()
	}
	return &ConfigManager{
		configFilePath: description,
		store:          store,
	}
}

// LoadConfigFromStore executa uma leitura atômica e validação da configuração
// na camada de persistência sem sofrer mutação no estado ativo do gerenciador.
func (mgr *ConfigManager) LoadConfigFromStore() (*BotConfig, bool, error) {
	if mgr.store == nil {
		err := fmt.Errorf("config store não está configurado")
		log.EmitBlockingError("Falha na inicialização da leitura de configuração", err, log.GenerateRequestID())
		return nil, false, err
	}
	cfg, err := mgr.store.Load()
	if err != nil {
		errWrap := fmt.Errorf("carregar configuração de %s: %w", mgr.ConfigPath(), err)
		log.EmitBlockingError("Falha estrutural no carregamento do arquivo", errWrap, log.GenerateRequestID())
		return nil, false, errWrap
	}

	orderMigrated := normalizeAutoAssignmentRoleOrder(cfg)

	if validationErr := validateBotConfig(cfg); validationErr != nil {
		errWrap := wrapValidationError(validationErr)
		log.EmitBlockingError("Falha de validação da configuração carregada", errWrap, log.GenerateRequestID())
		return nil, false, errWrap
	}
	return cfg, orderMigrated, nil
}

// ApplyConfig rotaciona atomicamente o ponteiro de configuração global e reconstrói índices.
func (mgr *ConfigManager) ApplyConfig(cfg *BotConfig) int {
	if cfg == nil {
		return 0
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	slog.Debug("Iniciando transição atômica de estado da configuração",
		slog.Int("guilds_payload_size", len(cfg.Guilds)),
	)

	oldCfg := mgr.config
	mgr.config = cfg

	if len(mgr.config.Guilds) == 0 {
		slog.Warn("Configuração aplicada não contém guildas ativas. Execução operando em limite basal.",
			slog.String("path", mgr.ConfigPath()),
		)
	}

	dupCount, err := mgr.rebuildGuildIndexLocked("apply")
	if err != nil {
		slog.Warn("Degradação mitigada na reconstrução do índice",
			slog.String("error", err.Error()),
			slog.String("path", mgr.ConfigPath()),
		)
	}

	mgr.publishSnapshotLocked()
	mgr.notifySubscribersLocked(oldCfg, cfg)

	slog.Info("Transição de estado da configuração concluída",
		slog.Int("duplicates_removed", dupCount),
	)
	return dupCount
}

// LoadConfig carrega a configuração diretamente do sistema de arquivos.
func (mgr *ConfigManager) LoadConfig() error {
	cfg, orderMigrated, err := mgr.LoadConfigFromStore()
	if err != nil {
		return err
	}

	dupCount := mgr.ApplyConfig(cfg)

	if dupCount > 0 || orderMigrated {
		slog.Debug("Anomalia estrutural sanada em memória, forçando persistência compensatória",
			slog.Bool("order_migrated", orderMigrated),
			slog.Int("duplicates", dupCount),
		)
		if saveErr := mgr.SaveConfig(); saveErr != nil {
			errWrap := fmt.Errorf("salvar configuração após normalização: %w", saveErr)
			log.EmitBlockingError("Falha ao gravar correções estruturais na configuração", errWrap, log.GenerateRequestID())
			return errWrap
		}
		slog.Info("Configuração re-persistida após normalização em tempo de execução",
			slog.String("path", mgr.ConfigPath()),
			slog.Int("duplicates", dupCount),
			slog.Bool("autoRoleOrderMigrated", orderMigrated),
		)
	} else if exists, err := mgr.store.Exists(); err == nil && !exists {
		slog.Info("Inicialização em estado limpo: arquivo primário não detectado",
			slog.String("path", mgr.ConfigPath()),
		)
	}
	return nil
}

// SaveConfig persiste a configuração ativa no sistema de arquivos.
func (mgr *ConfigManager) SaveConfig() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if err := mgr.saveConfigLocked(); err != nil {
		errWrap := fmt.Errorf("ConfigManager.SaveConfig: %w", err)
		log.EmitBlockingError("Falha de persistência global bloqueante", errWrap, log.GenerateRequestID())
		return errWrap
	}
	mgr.publishSnapshotLocked()
	return nil
}

// SaveGuildConfig atualiza uma configuração específica de guilda e persiste a alteração imediatamente.
func (mgr *ConfigManager) SaveGuildConfig(cfg GuildConfig) error {
	slog.Debug("Atualizando estado granular de guilda",
		slog.String("guildID", cfg.GuildID),
	)
	if err := mgr.AddGuildConfig(cfg); err != nil {
		return fmt.Errorf("falha ao atualizar configuração em memória: %w", err)
	}
	if err := mgr.SaveConfig(); err != nil {
		return fmt.Errorf("falha ao persistir configuração de guilda: %w", err)
	}
	return nil
}

func (mgr *ConfigManager) saveConfigLocked() error {
	if mgr.config == nil {
		return errors.New(ErrCannotSaveNilConfig)
	}
	if mgr.store == nil {
		return fmt.Errorf("config store não está configurado")
	}
	if validationErr := validateBotConfig(mgr.config); validationErr != nil {
		return wrapValidationError(validationErr)
	}

	if err := mgr.store.Save(mgr.config); err != nil {
		return fmt.Errorf("salvar configuração para %s: %w", mgr.ConfigPath(), err)
	}

	slog.Info("Transição de estado I/O: Configuração persistida com sucesso",
		slog.String("path", mgr.ConfigPath()),
	)

	return nil
}

// UpdateRuntimeConfig muta runtime_config e persiste a mudança em disco.
func (mgr *ConfigManager) UpdateRuntimeConfig(fn func(*RuntimeConfig) error) (RuntimeConfig, error) {
	snapshot, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		if fn == nil {
			return nil
		}
		return fn(&cfg.RuntimeConfig)
	})
	if err != nil {
		errWrap := fmt.Errorf("ConfigManager.UpdateRuntimeConfig: %w", err)
		log.EmitBlockingError("Falha mutacional na configuração de runtime", errWrap, log.GenerateRequestID())
		return RuntimeConfig{}, errWrap
	}
	return snapshot.RuntimeConfig, nil
}

// --- Getters ---

// ConfigPath retorna uma descrição em formato texto do backend de configuração ativo.
func (mgr *ConfigManager) ConfigPath() string {
	if mgr == nil {
		return ""
	}
	if strings.TrimSpace(mgr.configFilePath) != "" {
		return mgr.configFilePath
	}
	if mgr.store != nil {
		return mgr.store.Describe()
	}
	return ""
}

// Config retorna a publicação atual de leitura imutável do snapshot.
func (mgr *ConfigManager) Config() *BotConfig {
	snap := mgr.currentPublishedSnapshot()
	if snap == nil {
		return nil
	}
	return snap.config
}

// HasAnyGuilds avalia a existência de guildas configuradas.
func (mgr *ConfigManager) HasAnyGuilds() bool {
	snap := mgr.currentPublishedSnapshot()
	return snap != nil && snap.config != nil && len(snap.config.Guilds) > 0
}

// --- Guild Config Management ---

// GuildConfig retorna a publicação atual de leitura imutável do snapshot para uma guilda.
func (mgr *ConfigManager) GuildConfig(guildID string) *GuildConfig {
	if mgr == nil || guildID == "" {
		return nil
	}
	snap := mgr.currentPublishedSnapshot()
	if snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
		return nil
	}
	mgr.indexMisses.Add(1)
	return mgr.guildConfigWithPublish(guildID)
}

func (mgr *ConfigManager) guildConfigWithPublish(guildID string) *GuildConfig {
	if mgr == nil {
		return nil
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil || guildID == "" {
		return nil
	}
	if snap := mgr.publishSnapshotLocked(); snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
	}
	if _, err := mgr.rebuildGuildIndexLocked("lookup_miss"); err != nil {
		slog.Warn("Reconstrução de índice acionada via falha de cache mitigada",
			slog.String("guildID", guildID),
			slog.String("error", err.Error()),
		)
	}
	if snap := mgr.publishSnapshotLocked(); snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
	}
	slog.Debug("Mapeamento de guilda inexistente no índice consolidado",
		slog.String("guildID", guildID),
	)
	return nil
}

func (mgr *ConfigManager) rebuildGuildIndexLocked(reason string) (int, error) {
	mgr.indexRebuilds.Add(1)
	if mgr.config == nil {
		mgr.guildIndex = nil
		slog.Info("Índice de guildas anulado devido à configuração nula",
			slog.String("reason", reason),
		)
		return 0, nil
	}

	slog.Debug("Iterando estrutura de guildas para reconstrução de índice hash",
		slog.String("reason", reason),
	)

	index := make(map[string]int, len(mgr.config.Guilds))
	deduped := make([]GuildConfig, 0, len(mgr.config.Guilds))
	dupCount := 0

	for _, g := range mgr.config.Guilds {
		gid := g.GuildID
		if gid == "" {
			deduped = append(deduped, g)
			continue
		}
		if _, exists := index[gid]; exists {
			slog.Debug("Colisão de chaves evitada durante a construção do índice",
				slog.String("guildID", gid),
			)
			dupCount++
			continue
		}
		index[gid] = len(deduped)
		deduped = append(deduped, g)
	}

	if dupCount > 0 {
		mgr.indexDuplicates.Add(uint64(dupCount))
		slog.Warn("Integridade estrutural corrigida localmente: guildas duplicadas expurgadas do vetor",
			slog.String("reason", reason),
			slog.Int("duplicates", dupCount),
			slog.Int("remaining", len(deduped)),
		)
		mgr.config.Guilds = deduped
	}

	mgr.guildIndex = index
	slog.Info("Transição de estado estrutural concluída: Índice de guildas reconstruído",
		slog.String("reason", reason),
		slog.Int("guilds_count", len(mgr.config.Guilds)),
	)

	if dupCount > 0 {
		return dupCount, fmt.Errorf("removidas %d configurações de guildas duplicadas", dupCount)
	}
	return dupCount, nil
}

// GuildIndexStats retorna contadores operacionais para métricas de índice.
func (mgr *ConfigManager) GuildIndexStats() GuildIndexStats {
	if mgr == nil {
		return GuildIndexStats{}
	}
	return GuildIndexStats{
		Rebuilds:   mgr.indexRebuilds.Load(),
		Misses:     mgr.indexMisses.Load(),
		Duplicates: mgr.indexDuplicates.Load(),
	}
}

// AddGuildConfig injeta ou substitui a configuração mapeada de uma guilda.
func (mgr *ConfigManager) AddGuildConfig(guildCfg GuildConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	next := cloneBotConfigPtr(mgr.config)
	if next == nil {
		next = &BotConfig{Guilds: []GuildConfig{}}
	}

	slog.Debug("Injeção granular de guilda na árvore de configuração",
		slog.String("guildID", guildCfg.GuildID),
	)

	next.Guilds = append(slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildCfg.GuildID
	}), guildCfg)

	mgr.config = next
	if _, err := mgr.rebuildGuildIndexLocked("add"); err != nil {
		errWrap := fmt.Errorf("adicionar configuração de guilda: %w", err)
		log.EmitBlockingError("Falha crítica ao anexar configuração na árvore de estados", errWrap, log.GenerateRequestID())
		return errWrap
	}
	mgr.publishSnapshotLocked()
	return nil
}

// RemoveGuildConfig expurga uma configuração de guilda.
func (mgr *ConfigManager) RemoveGuildConfig(guildID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil {
		return
	}

	slog.Debug("Remoção atômica de nó da guilda na configuração",
		slog.String("guildID", guildID),
	)

	next := cloneBotConfigPtr(mgr.config)
	next.Guilds = slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildID
	})
	mgr.config = next

	if _, err := mgr.rebuildGuildIndexLocked("remove"); err != nil {
		slog.Warn("Colisão mitigada durante a reconstrução pós-remoção",
			slog.String("guildID", guildID),
			slog.String("error", err.Error()),
		)
	}
	mgr.publishSnapshotLocked()
}

// --- Guild Detection & Addition ---

// DetectGuilds detecta automaticamente guildas nas quais o bot está ativo.
func (mgr *ConfigManager) DetectGuilds(session *discordgo.Session) error {
	return mgr.DetectGuildsForBot(session, "")
}

// DetectGuildsForBot automatiza a descoberta de guildas e as acopla ao
// identificador de bot correspondente.
func (mgr *ConfigManager) DetectGuildsForBot(session *discordgo.Session, botInstanceID string) error {
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	detected := make([]GuildConfig, 0, len(session.State.Guilds))

	for _, g := range session.State.Guilds {
		fullGuild, err := session.Guild(g.ID)
		if err != nil {
			slog.Warn("Degradação na busca de dados arquiteturais da guilda; a operação principal continuará ociosamente",
				slog.String("guildID", g.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		channelID := FindSuitableChannel(session, g.ID)
		if channelID == "" {
			slog.Warn("Falha mitigada: canal operacional primário ausente na guilda alvo",
				slog.String("guildName", fullGuild.Name),
				slog.String("guildID", g.ID),
			)
			continue
		}

		roles := FindAdminRoles(session, g.ID)

		entryLeaveID := FindEntryLeaveChannel(session, g.ID)
		if entryLeaveID == "" {
			slog.Debug("Roteamento dinâmico: utilizando canal principal como fallback para entry_leave",
				slog.String("guildID", g.ID),
			)
			entryLeaveID = channelID
		}

		guildCfg := GuildConfig{
			GuildID: g.ID,
			Channels: ChannelsConfig{
				Commands:      channelID,
				AvatarLogging: channelID,
				RoleUpdate:    channelID,
				MemberJoin:    entryLeaveID,
				MemberLeave:   entryLeaveID,
				MessageEdit:   channelID,
				MessageDelete: channelID,
			},
			Roles: RolesConfig{
				Allowed: roles,
			},
		}
		detected = append(detected, guildCfg)
		slog.Info("Transição de rede: Guilda vinculada à matriz de descoberta",
			slog.String("guildName", fullGuild.Name),
			slog.String("guildID", g.ID),
			slog.String("channelID", channelID),
		)
	}

	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		cfg.Guilds = detected
		return nil
	})
	if err != nil {
		log.EmitBlockingError("Falha de atualização em bloco durante fase de detecção heurística", err, log.GenerateRequestID())
	}
	return err
}

// RegisterGuild injeta explicitamente uma nova guilda.
func (mgr *ConfigManager) RegisterGuild(session *discordgo.Session, guildID string) error {
	return mgr.RegisterGuildForBot(session, guildID, "")
}

// RegisterGuildForBot injeta e acopla a guilda à instância de bot parametrizada.
func (mgr *ConfigManager) RegisterGuildForBot(session *discordgo.Session, guildID, botInstanceID string) error {
	if session == nil {
		err := fmt.Errorf("%w: sessão do discord não está disponível", ErrGuildBootstrapDiscordFetch)
		log.EmitBlockingError("Estado corrompido em rotina de registro: Sessão nula", err, log.GenerateRequestID())
		return err
	}
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	if mgr.GuildConfig(guildID) != nil {
		slog.Info("Condição preexistente sanada silenciada: guilda já injetada",
			slog.String("guildID", guildID),
		)
		return nil
	}
	guild, err := session.Guild(guildID)
	if err != nil {
		return fmt.Errorf("%w: "+ErrGuildInfoFetchMsg, ErrGuildBootstrapDiscordFetch, guildID, err)
	}
	channelID := FindSuitableChannel(session, guildID)
	if channelID == "" {
		return fmt.Errorf("%w: "+ErrNoSuitableChannelMsg, ErrGuildBootstrapPrerequisite, guild.Name)
	}
	roles := FindAdminRoles(session, guildID)
	entryLeaveID := FindEntryLeaveChannel(session, guildID)
	if entryLeaveID == "" {
		entryLeaveID = channelID
	}

	guildCfg := GuildConfig{
		GuildID: guildID,
		Channels: ChannelsConfig{
			Commands:      channelID,
			AvatarLogging: channelID,
			RoleUpdate:    channelID,
			MemberJoin:    entryLeaveID,
			MemberLeave:   entryLeaveID,
			MessageEdit:   channelID,
			MessageDelete: channelID,
		},
		Roles: RolesConfig{
			Allowed: roles,
		},
	}

	if _, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		cfg.Guilds = append(cfg.Guilds, guildCfg)
		return nil
	}); err != nil {
		errWrap := fmt.Errorf("registrar guilda: salvar configuração: %w", err)
		log.EmitBlockingError("Falha bloqueante em rotina primária de injeção", errWrap, log.GenerateRequestID())
		return errWrap
	}

	channelName := channelID
	if ch, err := session.Channel(channelID); err == nil {
		channelName = ch.Name
	}
	slog.Info("Transição de estado arquitetural: Registro de guilda consumado e acoplado a porta serial",
		slog.String("guildName", guild.Name),
		slog.String("guildID", guildID),
		slog.String("channel", channelName),
	)
	return nil
}

// --- Utility & Logging ---

// ShowConfiguredGuilds emite logs sumários das instâncias indexadas.
func ShowConfiguredGuilds(s *discordgo.Session, configManager *ConfigManager) {
	configuration := configManager.Config()
	if configuration == nil || len(configuration.Guilds) == 0 {
		return
	}
	for _, guildConfig := range configuration.Guilds {
		if guild, err := s.Guild(guildConfig.GuildID); err == nil {
			slog.Info("Procedimento em conformidade: Monitoramento ativo sobre canal de telemetria da guilda",
				slog.String("guildName", guild.Name),
				slog.String("guildID", guild.ID),
			)
		} else {
			slog.Warn("Obstrução na malha de comunicação: Guilda registrada inacessível à inspeção de telemetria",
				slog.String("guildID", guildConfig.GuildID),
			)
		}
	}
}

// FindSuitableChannel busca o canal primário condizente para alocação de pipes.
func FindSuitableChannel(session *discordgo.Session, guildID string) string {
	if session == nil || session.State == nil || session.State.User == nil {
		return ""
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil || channels == nil {
		return ""
	}
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			permissions, err := session.UserChannelPermissions(session.State.User.ID, channel.ID)
			if err == nil && (permissions&discordgo.PermissionSendMessages) != 0 {
				if channel.Name == "general" || channel.Name == "geral" || channel.Name == "bot-logs" || channel.Name == "logs" {
					return channel.ID
				}
				if channel.ID != "" {
					return channel.ID
				}
			}
		}
	}
	return ""
}

// FindEntryLeaveChannel busca canal primário para registro de eventos de I/O de usuários.
func FindEntryLeaveChannel(session *discordgo.Session, guildID string) string {
	if session == nil || session.State == nil || session.State.User == nil {
		return ""
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return ""
	}
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			name := strings.ToLower(channel.Name)
			if name == "user-entry-leave" {
				if HasSendPermission(session, channel.ID) {
					return channel.ID
				}
			}
		}
	}
	return ""
}

// HasSendPermission valida os vetores de autorização contra a bitmask alvo.
func HasSendPermission(session *discordgo.Session, channelID string) bool {
	if session == nil || session.State == nil || session.State.User == nil || channelID == "" {
		return false
	}
	if perms, err := session.UserChannelPermissions(session.State.User.ID, channelID); err == nil {
		return (perms & discordgo.PermissionSendMessages) != 0
	}
	return false
}

// FindAdminRoles extrai do vetor as funções contendo o bitmask de administrador.
func FindAdminRoles(session *discordgo.Session, guildID string) []string {
	var allowedRoles []string
	roles, err := session.GuildRoles(guildID)
	if err == nil {
		for _, role := range roles {
			if role.Name != "@everyone" && (role.Permissions&discordgo.PermissionAdministrator) != 0 {
				allowedRoles = append(allowedRoles, role.ID)
			}
		}
	}
	return allowedRoles
}

// TextChannels converte e extrai do multiplexador os canais aptos para transmissão em formato texto.
func TextChannels(session *discordgo.Session, guildID string) ([]*discordgo.Channel, error) {
	if session == nil || session.State == nil || session.State.User == nil {
		return nil, fmt.Errorf("sessão não inicializada")
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("TextChannels: %w", err)
	}
	var textChannels []*discordgo.Channel
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			permissions, err := session.UserChannelPermissions(session.State.User.ID, channel.ID)
			if err == nil && (permissions&discordgo.PermissionSendMessages) != 0 {
				textChannels = append(textChannels, channel)
			}
		}
	}
	return textChannels, nil
}

// ValidateChannel valida propriedades de nó, estrutura hierárquica e integridade de restrições.
func ValidateChannel(session *discordgo.Session, guildID, channelID string) error {
	if session == nil || session.State == nil || session.State.User == nil {
		return errors.New("sessão não inicializada")
	}
	channel, err := session.Channel(channelID)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrChannelNotFound, err)
	}
	if channel.GuildID != guildID {
		return errors.New(ErrChannelWrongGuild)
	}
	if channel.Type != discordgo.ChannelTypeGuildText {
		return errors.New(ErrChannelWrongType)
	}
	permissions, err := session.UserChannelPermissions(session.State.User.ID, channelID)
	if err != nil {
		return fmt.Errorf(ErrFailedCheckPerms, err)
	}
	if (permissions & discordgo.PermissionSendMessages) == 0 {
		return errors.New(ErrChannelNoPermissions)
	}
	return nil
}

// LogConfiguredGuilds sumaria em log a árvore de nós mapeada.
func LogConfiguredGuilds(configManager *ConfigManager, session *discordgo.Session) error {
	return LogConfiguredGuildsForBot(configManager, session, "")
}

// LogConfiguredGuildsForBot sumariza o subconjunto mapeado designado para roteamento de instância bot explícita.
func LogConfiguredGuildsForBot(configManager *ConfigManager, session *discordgo.Session, botInstanceID string) error {
	return logConfiguredGuildSubset(configManager, session, func(cfg *BotConfig) []GuildConfig {
		guilds := cfg.Guilds
		if normalizedBotInstanceID := NormalizeBotInstanceID(botInstanceID); normalizedBotInstanceID != "" {
			guilds = cfg.GuildsForBotInstance(normalizedBotInstanceID)
		}
		return guilds
	})
}

func logConfiguredGuildSubset(configManager *ConfigManager, session *discordgo.Session, resolve func(*BotConfig) []GuildConfig) error {
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		slog.Warn("Limiar basal atingido: Vetor de alocação de guildas vazio na rotina de boot")
		return nil
	}

	guilds := cfg.Guilds
	if resolve != nil {
		guilds = resolve(cfg)
	}
	if len(guilds) == 0 {
		slog.Warn("Limiar basal atingido: Vetor de alocação de guildas vazio na rotina de boot")
		return nil
	}

	slog.Info("Sumarização de carga inicializada",
		slog.Int("guilds_count", len(guilds)),
	)

	var errCount int
	for _, g := range guilds {
		guild, err := session.Guild(g.GuildID)
		if err == nil {
			slog.Info("Interface ativa confirmada",
				slog.String("guildName", guild.Name),
				slog.String("guildID", guild.ID),
			)
		} else {
			slog.Warn("Falha de handshake com interface da guilda reportada pelo hub central",
				slog.String("guildID", g.GuildID),
			)
			errCount++
		}
	}
	return nil
}
