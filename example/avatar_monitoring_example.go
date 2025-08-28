package example

// // =================================================================================
// // EXEMPLO COMPLETO: Sistema de Monitoramento de Avatares em Outro Repositório
// // =================================================================================
// //
// // Este arquivo demonstra como implementar um sistema completo de monitoramento
// // de mudanças de avatar usando o discordcore como base.
// //
// // Cenário: Você quer criar um bot que:
// // 1. Detecta quando usuários mudam seus avatares
// // 2. Mantém um histórico de avatares
// // 3. Notifica administradores sobre mudanças suspeitas
// // 4. Gera relatórios de atividade de avatar
// //
// // Arquitetura:
// // - discordcore: Fornece eventos e infraestrutura
// // - Seu repositório: Contém lógica específica do negócio

// import (
// 	"fmt"
// 	"log"
// 	"time"

// 	"github.com/alice-bnuy/discordcore/v2"
// )

// // ================================================================================
// // 1. PROCESSADOR DE EVENTOS DE AVATAR
// // ================================================================================

// // AvatarMonitor implementa a lógica específica de monitoramento de avatares
// type AvatarMonitor struct {
// 	avatarHistory      map[string][]AvatarRecord // userID -> histórico de avatares
// 	alertChannel       string                    // Canal para notificações
// 	suspiciousPatterns []string                  // Padrões suspeitos de avatar
// }

// // AvatarRecord representa um registro no histórico de avatares
// type AvatarRecord struct {
// 	UserID    string
// 	Username  string
// 	Avatar    string
// 	AvatarURL string
// 	ChangedAt time.Time
// 	GuildID   string
// }

// // NewAvatarMonitor cria um novo monitor de avatares
// func NewAvatarMonitor(alertChannelID string) *AvatarMonitor {
// 	return &AvatarMonitor{
// 		avatarHistory:      make(map[string][]AvatarRecord),
// 		alertChannel:       alertChannelID,
// 		suspiciousPatterns: []string{"suspicious", "spam", "bot"},
// 	}
// }

// // ProcessEvent processa eventos do discordcore
// func (am *AvatarMonitor) ProcessEvent(event discordcore.Event) {
// 	switch event.GetEventType() {
// 	case "avatar_change":
// 		am.handleAvatarChange(event)
// 	case "guild_member_add":
// 		am.handleNewMember(event)
// 	}
// }

// // handleAvatarChange processa mudanças de avatar
// func (am *AvatarMonitor) handleAvatarChange(event discordcore.Event) {
// 	data := event.GetData()
// 	userID := event.GetUserID()
// 	guildID := event.GetGuildID()

// 	record := AvatarRecord{
// 		UserID:    userID,
// 		Username:  data["username"].(string),
// 		Avatar:    data["new_avatar"].(string),
// 		AvatarURL: data["avatar_url"].(string),
// 		ChangedAt: data["changed_at"].(time.Time),
// 		GuildID:   guildID,
// 	}

// 	// Adiciona ao histórico
// 	am.avatarHistory[userID] = append(am.avatarHistory[userID], record)

// 	// Verifica padrões suspeitos
// 	if am.isSuspiciousAvatar(record) {
// 		am.alertSuspiciousChange(record)
// 	}

// 	// Log da mudança
// 	fmt.Printf("📸 Avatar changed: %s (%s) - %s\n",
// 		record.Username, userID, record.AvatarURL)
// }

// // handleNewMember registra o avatar inicial de novos membros
// func (am *AvatarMonitor) handleNewMember(event discordcore.Event) {
// 	data := event.GetData()
// 	userID := event.GetUserID()
// 	guildID := event.GetGuildID()

// 	// Nota: Novos membros podem não ter avatar no evento de entrada
// 	// O avatar será capturado no primeiro GuildMemberUpdate
// 	fmt.Printf("👋 New member: %s joined guild %s\n",
// 		data["username"], guildID)
// }

// // isSuspiciousAvatar verifica se o avatar parece suspeito
// func (am *AvatarMonitor) isSuspiciousAvatar(record AvatarRecord) bool {
// 	// Verifica se o avatar mudou muito rapidamente (possível spam)
// 	history := am.avatarHistory[record.UserID]
// 	if len(history) >= 2 {
// 		lastChange := history[len(history)-2]
// 		timeSinceLastChange := record.ChangedAt.Sub(lastChange.ChangedAt)

// 		// Mudanças muito rápidas podem ser suspeitas
// 		if timeSinceLastChange < 5*time.Minute {
// 			return true
// 		}
// 	}

// 	// Verifica padrões suspeitos no hash do avatar
// 	for _, pattern := range am.suspiciousPatterns {
// 		if record.Avatar != "" && contains(record.Avatar, pattern) {
// 			return true
// 		}
// 	}

// 	return false
// }

// // alertSuspiciousChange envia alerta para administradores
// func (am *AvatarMonitor) alertSuspiciousChange(record AvatarRecord) {
// 	fmt.Printf("🚨 ALERT: Suspicious avatar change detected!\n")
// 	fmt.Printf("   User: %s (%s)\n", record.Username, record.UserID)
// 	fmt.Printf("   New Avatar: %s\n", record.AvatarURL)
// 	fmt.Printf("   Time: %s\n", record.ChangedAt.Format("2006-01-02 15:04:05"))

// 	// Aqui você enviaria uma mensagem para o canal de alertas
// 	// sendDiscordMessage(am.alertChannel, alertMessage)
// }

// // GetAvatarHistory retorna o histórico de avatares de um usuário
// func (am *AvatarMonitor) GetAvatarHistory(userID string) []AvatarRecord {
// 	return am.avatarHistory[userID]
// }

// // GetAvatarChangeStats retorna estatísticas de mudanças de avatar
// func (am *AvatarMonitor) GetAvatarChangeStats() map[string]int {
// 	stats := make(map[string]int)
// 	stats["total_users"] = len(am.avatarHistory)
// 	stats["total_changes"] = 0

// 	for _, history := range am.avatarHistory {
// 		stats["total_changes"] += len(history)
// 	}

// 	return stats
// }

// // Start inicializa o monitor
// func (am *AvatarMonitor) Start() {
// 	fmt.Println("🎨 Avatar Monitor started")
// }

// // Stop finaliza o monitor
// func (am *AvatarMonitor) Stop() {
// 	fmt.Println("🎨 Avatar Monitor stopped")
// }

// // ================================================================================
// // 2. UTILITÁRIOS
// // ================================================================================

// // contains verifica se uma string contém um substring (case-insensitive)
// func contains(s, substr string) bool {
// 	return len(s) >= len(substr) &&
// 		(s == substr ||
// 			contains(s[1:], substr) ||
// 			(len(s) > 0 && s[0] != substr[0] && contains(s[1:], substr)))
// }

// // ================================================================================
// // 3. EXEMPLO DE USO COMPLETO
// // ================================================================================

// func main() {
// 	// ============================================================================
// 	// CONFIGURAÇÃO DO DISCORDCORE
// 	// ============================================================================

// 	// 1. Inicializar o core do Discord
// 	core, err := discordcore.NewDiscordCore("YOUR_BOT_TOKEN")
// 	if err != nil {
// 		log.Fatal("Failed to create Discord core:", err)
// 	}

// 	// 2. Criar sessão
// 	session, err := core.NewDiscordSession()
// 	if err != nil {
// 		log.Fatal("Failed to create Discord session:", err)
// 	}
// 	defer session.Close()

// 	// 3. Inicializar cache de avatares (necessário para detectar mudanças)
// 	avatarCache, err := core.NewAvatarCacheManager()
// 	if err != nil {
// 		log.Fatal("Failed to create avatar cache:", err)
// 	}

// 	// Carregar cache existente
// 	if err := avatarCache.Load(); err != nil {
// 		log.Printf("Warning: Could not load avatar cache: %v", err)
// 	}

// 	// ============================================================================
// 	// CONFIGURAÇÃO DO SISTEMA DE MONITORAMENTO
// 	// ============================================================================

// 	// 1. Criar serviço de monitoramento
// 	monitoring := discordcore.NewMonitoringService()

// 	// 2. Criar monitor de avatares
// 	avatarMonitor := NewAvatarMonitor("ALERT_CHANNEL_ID")

// 	// 3. Registrar processadores
// 	monitoring.AddProcessor(avatarMonitor)

// 	// 4. Registrar handlers específicos (opcional)
// 	monitoring.RegisterEventHandler("avatar_change", func(event discordcore.Event) {
// 		data := event.GetData()
// 		fmt.Printf("🎭 Quick handler: %s changed avatar to %s\n",
// 			data["username"], data["avatar_url"])
// 	})

// 	// ============================================================================
// 	// CONEXÃO COM DISCORD (COM SUPORTE A AVATAR)
// 	// ============================================================================

// 	// 1. Criar adapter com suporte a cache de avatares
// 	adapter := discordcore.NewDiscordEventAdapterWithAvatarCache(
// 		session,
// 		core.ConfigManager,
// 		monitoring,
// 		avatarCache,
// 	)

// 	// 2. Adicionar adapter como processador
// 	monitoring.AddProcessor(adapter)

// 	// ============================================================================
// 	// INICIALIZAÇÃO E LOOP PRINCIPAL
// 	// ============================================================================

// 	// 1. Iniciar monitoramento
// 	if err := monitoring.Start(); err != nil {
// 		log.Fatal("Failed to start monitoring:", err)
// 	}
// 	defer monitoring.Stop()

// 	// 2. Loop principal com estatísticas periódicas
// 	ticker := time.NewTicker(1 * time.Hour)
// 	defer ticker.Stop()

// 	fmt.Println("🤖 Avatar monitoring system started!")
// 	fmt.Println("   Monitoring avatar changes and suspicious activity...")

// 	for {
// 		select {
// 		case <-ticker.C:
// 			// Estatísticas periódicas
// 			stats := avatarMonitor.GetAvatarChangeStats()
// 			fmt.Printf("📊 Stats: %d users, %d total avatar changes\n",
// 				stats["total_users"], stats["total_changes"])

// 			// Aqui você pode adicionar outras lógicas do seu bot
// 		}
// 	}
// }

// // ================================================================================
// // 4. EXEMPLO DE EXTENSÃO: MONITOR DE AVATAR COM BANCO DE DADOS
// // ================================================================================

// // AvatarMonitorDB versão que persiste dados em banco de dados
// type AvatarMonitorDB struct {
// 	*AvatarMonitor
// 	db Database // Interface para seu banco de dados
// }

// // Database interface para abstrair o banco de dados
// type Database interface {
// 	SaveAvatarRecord(record AvatarRecord) error
// 	GetAvatarHistory(userID string) ([]AvatarRecord, error)
// 	GetSuspiciousChanges(hours int) ([]AvatarRecord, error)
// }

// // NewAvatarMonitorDB cria monitor com persistência
// func NewAvatarMonitorDB(alertChannelID string, db Database) *AvatarMonitorDB {
// 	return &AvatarMonitorDB{
// 		AvatarMonitor: NewAvatarMonitor(alertChannelID),
// 		db:            db,
// 	}
// }

// // handleAvatarChange sobrescreve para incluir persistência
// func (amdb *AvatarMonitorDB) handleAvatarChange(event discordcore.Event) {
// 	// Chama implementação base
// 	amdb.AvatarMonitor.handleAvatarChange(event)

// 	// Persiste no banco
// 	data := event.GetData()
// 	record := AvatarRecord{
// 		UserID:    event.GetUserID(),
// 		Username:  data["username"].(string),
// 		Avatar:    data["new_avatar"].(string),
// 		AvatarURL: data["avatar_url"].(string),
// 		ChangedAt: data["changed_at"].(time.Time),
// 		GuildID:   event.GetGuildID(),
// 	}

// 	if err := amdb.db.SaveAvatarRecord(record); err != nil {
// 		log.Printf("Failed to save avatar record: %v", err)
// 	}
// }

// // ================================================================================
// // 5. DICAS DE IMPLEMENTAÇÃO
// // ================================================================================

// /*
// IMPLEMENTAÇÃO RECOMENDADA:

// 1. Separe as responsabilidades:
//    - discordcore: Apenas eventos e infraestrutura
//    - Seu código: Lógica de negócio específica

// 2. Use injeção de dependência:
//    - Passe interfaces, não implementações concretas
//    - Facilita testes e manutenção

// 3. Implemente cache inteligente:
//    - Use o AvatarCacheManager do discordcore
//    - Configure salvamento automático periódico

// 4. Monitore performance:
//    - Limite histórico de avatares por usuário
//    - Implemente cleanup automático de dados antigos

// 5. Segurança:
//    - Valide tokens e permissões
//    - Implemente rate limiting
//    - Log todas as ações suspeitas

// EXEMPLO DE ESTRUTURA DE PROJETO:

// meu-avatar-monitor/
// ├── main.go              # Ponto de entrada
// ├── avatar_monitor.go    # Lógica principal
// ├── database.go          # Camada de persistência
// ├── discord_client.go    # Integração com Discord
// ├── config.go           # Configurações
// └── models/             # Estruturas de dados
//     ├── avatar.go
//     └── stats.go

// */
