package logging

import (
	"fmt"
	"sync"
	"time"

	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

// CachedMessage armazena dados de mensagens para comparação
type CachedMessage struct {
	ID        string
	Content   string
	Author    *discordgo.User
	ChannelID string
	GuildID   string
	Timestamp time.Time
}

// MessageEventService gerencia eventos de mensagens (deletar/editar)
type MessageEventService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	notifier      *NotificationSender
	messageCache  map[string]*CachedMessage // Cache de mensagens para detectar edições
	cacheMutex    sync.RWMutex              // Mutex para proteger o cache
	isRunning     bool
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewMessageEventService cria uma nova instância do serviço de eventos de mensagens
func NewMessageEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender) *MessageEventService {
	return &MessageEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		messageCache:  make(map[string]*CachedMessage),
		isRunning:     false,
		stopCleanup:   make(chan struct{}),
	}
}

// Start registra os handlers de eventos de mensagens
func (mes *MessageEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("message event service is already running")
	}
	mes.isRunning = true

	mes.session.AddHandler(mes.handleMessageCreate)
	mes.session.AddHandler(mes.handleMessageUpdate)
	mes.session.AddHandler(mes.handleMessageDelete)

	// Iniciar limpeza automática do cache a cada hora
	mes.cleanupTicker = time.NewTicker(1 * time.Hour)
	go mes.cleanupRoutine()

	logutil.Info("Message event service started")
	return nil
}

// Stop para o serviço
func (mes *MessageEventService) Stop() error {
	if !mes.isRunning {
		return fmt.Errorf("message event service is not running")
	}
	mes.isRunning = false

	if mes.cleanupTicker != nil {
		mes.cleanupTicker.Stop()
	}
	close(mes.stopCleanup)

	logutil.Info("Message event service stopped")
	return nil
}

// IsRunning retorna se o serviço está rodando
func (mes *MessageEventService) IsRunning() bool {
	return mes.isRunning
}

// handleMessageCreate armazena mensagens no cache para futuras comparações
func (mes *MessageEventService) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Author == nil || m.Author.Bot || m.Content == "" {
		return
	}

	// Verificar se é uma mensagem de guild
	channel, err := s.Channel(m.ChannelID)
	if err != nil || channel.GuildID == "" {
		return // Ignorar DMs
	}

	// Verificar se o guild está configurado
	guildConfig := mes.configManager.GuildConfig(channel.GuildID)
	if guildConfig == nil {
		return
	}

	// Armazenar no cache
	mes.cacheMutex.Lock()
	mes.messageCache[m.ID] = &CachedMessage{
		ID:        m.ID,
		Content:   m.Content,
		Author:    m.Author,
		ChannelID: m.ChannelID,
		GuildID:   channel.GuildID,
		Timestamp: m.Timestamp,
	}
	mes.cacheMutex.Unlock()

	logutil.WithFields(map[string]interface{}{
		"guildID":   channel.GuildID,
		"channelID": m.ChannelID,
		"messageID": m.ID,
		"userID":    m.Author.ID,
	}).Debug("Message cached for monitoring")
}

// handleMessageUpdate processa edições de mensagens
func (mes *MessageEventService) handleMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m == nil || m.Author == nil || m.Author.Bot {
		return
	}

	// Verificar se temos a mensagem original no cache
	mes.cacheMutex.RLock()
	cached, exists := mes.messageCache[m.ID]
	mes.cacheMutex.RUnlock()

	if !exists {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"userID":    m.Author.ID,
		}).Debug("Message edit detected but original not in cache")
		return
	}

	// Verificar se realmente mudou o conteúdo
	if cached.Content == m.Content {
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		return
	}

	logChannelID := guildConfig.MessageLogChannelID
	if logChannelID == "" {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
		}).Debug("MessageLogChannelID not configured for guild, message edit notification not sent")
		return
	}

	logutil.WithFields(map[string]interface{}{
		"guildID":   cached.GuildID,
		"channelID": cached.ChannelID,
		"messageID": m.ID,
		"userID":    cached.Author.ID,
		"username":  cached.Author.Username,
	}).Info("Message edit detected")

	// Enviar notificação de edição
	if err := mes.notifier.SendMessageEditNotification(logChannelID, cached, m); err != nil {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
			"channelID": logChannelID,
			"error":     err,
		}).Error("Failed to send message edit notification")
	} else {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
			"channelID": logChannelID,
		}).Info("Message edit notification sent successfully")
	}

	// Atualizar cache com novo conteúdo
	mes.cacheMutex.Lock()
	if cachedMsg, exists := mes.messageCache[m.ID]; exists {
		cachedMsg.Content = m.Content
	}
	mes.cacheMutex.Unlock()
}

// handleMessageDelete processa deleções de mensagens
func (mes *MessageEventService) handleMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	if m == nil {
		return
	}

	mes.cacheMutex.RLock()
	cached, exists := mes.messageCache[m.ID]
	mes.cacheMutex.RUnlock()

	if !exists {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"channelID": m.ChannelID,
		}).Debug("Message delete detected but original not in cache")
		return
	}

	// Pular se for bot
	if cached.Author.Bot {
		mes.cacheMutex.Lock()
		delete(mes.messageCache, m.ID)
		mes.cacheMutex.Unlock()
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		mes.cacheMutex.Lock()
		delete(mes.messageCache, m.ID)
		mes.cacheMutex.Unlock()
		return
	}

	logChannelID := guildConfig.MessageLogChannelID
	if logChannelID == "" {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
		}).Debug("MessageLogChannelID not configured for guild, message delete notification not sent")
		mes.cacheMutex.Lock()
		delete(mes.messageCache, m.ID)
		mes.cacheMutex.Unlock()
		return
	}

	logutil.WithFields(map[string]interface{}{
		"guildID":   cached.GuildID,
		"channelID": cached.ChannelID,
		"messageID": m.ID,
		"userID":    cached.Author.ID,
		"username":  cached.Author.Username,
	}).Info("Message delete detected")

	// Tentar determinar quem deletou (limitado pela API do Discord)
	deletedBy := "Usuário" // Padrão - assumimos que foi o próprio usuário
	// TODO: Implementar auditlog check para detectar se foi um moderador

	// Enviar notificação de deleção
	if err := mes.notifier.SendMessageDeleteNotification(logChannelID, cached, deletedBy); err != nil {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
			"channelID": logChannelID,
			"error":     err,
		}).Error("Failed to send message delete notification")
	} else {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
			"channelID": logChannelID,
		}).Info("Message delete notification sent successfully")
	}

	// Remover do cache
	mes.cacheMutex.Lock()
	delete(mes.messageCache, m.ID)
	mes.cacheMutex.Unlock()
}

// cleanupRoutine executa limpeza automática do cache
func (mes *MessageEventService) cleanupRoutine() {
	for {
		select {
		case <-mes.cleanupTicker.C:
			mes.cleanOldCache()
		case <-mes.stopCleanup:
			return
		}
	}
}

// cleanOldCache remove mensagens antigas do cache (> 24 horas)
func (mes *MessageEventService) cleanOldCache() {
	cutoff := time.Now().Add(-24 * time.Hour)
	removedCount := 0

	mes.cacheMutex.Lock()
	for id, msg := range mes.messageCache {
		if msg.Timestamp.Before(cutoff) {
			delete(mes.messageCache, id)
			removedCount++
		}
	}
	cacheSize := len(mes.messageCache)
	mes.cacheMutex.Unlock()

	if removedCount > 0 {
		logutil.WithFields(map[string]interface{}{
			"removedCount":       removedCount,
			"remainingCacheSize": cacheSize,
		}).Info("Message cache cleanup completed")
	}
}

// GetCacheStats retorna estatísticas do cache para debugging
func (mes *MessageEventService) GetCacheStats() map[string]interface{} {
	mes.cacheMutex.RLock()
	defer mes.cacheMutex.RUnlock()

	stats := map[string]interface{}{
		"totalCached": len(mes.messageCache),
		"isRunning":   mes.isRunning,
	}

	// Contar mensagens por guild
	guildCounts := make(map[string]int)
	for _, msg := range mes.messageCache {
		guildCounts[msg.GuildID]++
	}
	stats["perGuild"] = guildCounts

	return stats
}
