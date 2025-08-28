package example

// =================================================================================
// EXEMPLO COMPLETO: Sistema de Slash Commands em Outro Repositório
// =================================================================================
//
// Este arquivo demonstra como implementar um sistema completo de slash commands
// usando o discordcore como base.
//
// Cenário: Você quer criar um bot que:
// 1. Registra comandos slash personalizados
// 2. Processa interações de usuários
// 3. Integra com o sistema de monitoramento de eventos
// 4. Mantém modularidade para extensões futuras

// Arquitetura:
// - discordcore: Fornece infraestrutura genérica para comandos
// - Seu repositório: Contém lógica específica do negócio

// import (
// 	"fmt"
// 	"log"
// 	"os"
// 	"os/signal"
// 	"syscall"
// 	"time"

// 	"github.com/alice-bnuy/discordcore/v2"
// 	"github.com/bwmarrin/discordgo"
// )

// // ================================================================================
// // 1. COMANDOS SLASH PERSONALIZADOS
// // ================================================================================

// // PingCommand é um exemplo simples de comando slash
// type PingCommand struct{}

// // GetName retorna o nome do comando
// func (pc *PingCommand) GetName() string {
// 	return "ping"
// }

// // GetDescription retorna a descrição do comando
// func (pc *PingCommand) GetDescription() string {
// 	return "Responde com pong e informações do servidor"
// }

// // Execute processa a execução do comando
// func (pc *PingCommand) Execute(session *discordgo.Session, interaction *discordgo.InteractionCreate) error {
// 	// Obter informações do servidor
// 	guild, err := session.Guild(interaction.GuildID)
// 	if err != nil {
// 		return err
// 	}

// 	memberCount := guild.MemberCount
// 	channelCount := len(guild.Channels)

// 	response := fmt.Sprintf("🏓 Pong! Servidor tem %d membros e %d canais.", memberCount, channelCount)

// 	return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
// 		Data: &discordgo.InteractionResponseData{
// 			Content: response,
// 		},
// 	})
// }

// // UserInfoCommand exemplo de comando com parâmetros
// type UserInfoCommand struct {
// 	monitoringService *discordcore.MonitoringService
// }

// // NewUserInfoCommand cria um novo comando
// func NewUserInfoCommand(ms *discordcore.MonitoringService) *UserInfoCommand {
// 	return &UserInfoCommand{monitoringService: ms}
// }

// // GetName retorna o nome do comando
// func (uic *UserInfoCommand) GetName() string {
// 	return "userinfo"
// }

// // GetDescription retorna a descrição do comando
// func (uic *UserInfoCommand) GetDescription() string {
// 	return "Exibe informações sobre um usuário"
// }

// // Execute processa a execução do comando
// func (uic *UserInfoCommand) Execute(session *discordgo.Session, interaction *discordgo.InteractionCreate) error {
// 	// Extrair opções do comando (exemplo simplificado)
// 	data := interaction.ApplicationCommandData()
// 	var userID string

// 	for _, option := range data.Options {
// 		if option.Name == "user" {
// 			userID = option.UserValue(session).ID
// 		}
// 	}

// 	if userID == "" {
// 		userID = interaction.Member.User.ID
// 	}

// 	// Buscar informações do usuário no servidor
// 	member, err := session.GuildMember(interaction.GuildID, userID)
// 	if err != nil {
// 		return err
// 	}

// 	// Calcular tempo no servidor
// 	var joinedAt time.Time
// 	if member.JoinedAt != "" {
// 		joinedAt, _ = time.Parse(time.RFC3339, member.JoinedAt)
// 	}
// 	timeInGuild := time.Since(joinedAt)

// 	response := fmt.Sprintf("👤 **%s**#%s\n📅 Entrou há: %v\n🎭 Cargos: %d",
// 		member.User.Username, member.User.Discriminator, timeInGuild.Truncate(time.Hour), len(member.Roles))

// 	return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
// 		Data: &discordgo.InteractionResponseData{
// 			Content: response,
// 		},
// 	})
// }

// // ModerationCommand exemplo de comando com opções avançadas
// type ModerationCommand struct{}

// // GetName retorna o nome do comando
// func (mc *ModerationCommand) GetName() string {
// 	return "mod"
// }

// // GetDescription retorna a descrição do comando
// func (mc *ModerationCommand) GetDescription() string {
// 	return "Comandos de moderação"
// }

// // Execute processa a execução do comando
// func (mc *ModerationCommand) Execute(session *discordgo.Session, interaction *discordgo.InteractionCreate) error {
// 	data := interaction.ApplicationCommandData()

// 	// Verificar se o usuário tem permissões de moderação
// 	if !hasModerationPermissions(interaction.Member) {
// 		return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
// 			Type: discordgo.InteractionResponseChannelMessageWithSource,
// 			Data: &discordgo.InteractionResponseData{
// 				Content: "❌ Você não tem permissão para usar este comando.",
// 				Flags:   discordgo.MessageFlagsEphemeral,
// 			},
// 		})
// 	}

// 	// Processar subcomandos
// 	if len(data.Options) == 0 {
// 		return mc.showHelp(session, interaction)
// 	}

// 	subcommand := data.Options[0]
// 	switch subcommand.Name {
// 	case "kick":
// 		return mc.handleKick(session, interaction, subcommand)
// 	case "ban":
// 		return mc.handleBan(session, interaction, subcommand)
// 	case "warn":
// 		return mc.handleWarn(session, interaction, subcommand)
// 	default:
// 		return mc.showHelp(session, interaction)
// 	}
// }

// // showHelp mostra ajuda do comando
// func (mc *ModerationCommand) showHelp(session *discordgo.Session, interaction *discordgo.InteractionCreate) error {
// 	embed := &discordgo.MessageEmbed{
// 		Title: "🛡️ Comandos de Moderação",
// 		Color: 0xff0000,
// 		Fields: []*discordgo.MessageEmbedField{
// 			{
// 				Name:   "/mod kick <usuário> [motivo]",
// 				Value:  "Expulsa um usuário do servidor",
// 				Inline: false,
// 			},
// 			{
// 				Name:   "/mod ban <usuário> [motivo]",
// 				Value:  "Bane um usuário do servidor",
// 				Inline: false,
// 			},
// 			{
// 				Name:   "/mod warn <usuário> [motivo]",
// 				Value:  "Adverte um usuário",
// 				Inline: false,
// 			},
// 		},
// 	}

// 	return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
// 		Data: &discordgo.InteractionResponseData{
// 			Embeds: []*discordgo.MessageEmbed{embed},
// 			Flags:  discordgo.MessageFlagsEphemeral,
// 		},
// 	})
// }

// // handleKick processa comando de kick
// func (mc *ModerationCommand) handleKick(session *discordgo.Session, interaction *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) error {
// 	// Implementação simplificada
// 	user := subcommand.Options[0].UserValue(session)
// 	reason := "Sem motivo especificado"

// 	if len(subcommand.Options) > 1 {
// 		reason = subcommand.Options[1].StringValue()
// 	}

// 	err := session.GuildMemberDeleteWithReason(interaction.GuildID, user.ID, reason)
// 	if err != nil {
// 		return err
// 	}

// 	response := fmt.Sprintf("✅ Usuário %s foi expulso. Motivo: %s", user.Username, reason)
// 	return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
// 		Data: &discordgo.InteractionResponseData{
// 			Content: response,
// 		},
// 	})
// }

// // handleBan processa comando de ban
// func (mc *ModerationCommand) handleBan(session *discordgo.Session, interaction *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) error {
// 	// Implementação simplificada
// 	user := subcommand.Options[0].UserValue(session)
// 	reason := "Sem motivo especificado"

// 	if len(subcommand.Options) > 1 {
// 		reason = subcommand.Options[1].StringValue()
// 	}

// 	err := session.GuildBanCreateWithReason(interaction.GuildID, user.ID, reason, 0)
// 	if err != nil {
// 		return err
// 	}

// 	response := fmt.Sprintf("✅ Usuário %s foi banido. Motivo: %s", user.Username, reason)
// 	return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
// 		Data: &discordgo.InteractionResponseData{
// 			Content: response,
// 		},
// 	})
// }

// // handleWarn processa comando de warn
// func (mc *ModerationCommand) handleWarn(session *discordgo.Session, interaction *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) error {
// 	user := subcommand.Options[0].UserValue(session)
// 	reason := "Sem motivo especificado"

// 	if len(subcommand.Options) > 1 {
// 		reason = subcommand.Options[1].StringValue()
// 	}

// 	response := fmt.Sprintf("⚠️ Usuário %s foi advertido. Motivo: %s", user.Username, reason)
// 	return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
// 		Data: &discordgo.InteractionResponseData{
// 			Content: response,
// 		},
// 	})
// }

// // hasModerationPermissions verifica se o usuário tem permissões de moderação
// func hasModerationPermissions(member *discordgo.Member) bool {
// 	// Verificar se tem cargo de administrador ou moderador
// 	for _, roleID := range member.Roles {
// 		// Aqui você implementaria a lógica específica do seu servidor
// 		// Por exemplo, verificar se o cargo tem permissões de moderação
// 		if roleID == "ADMIN_ROLE_ID" || roleID == "MOD_ROLE_ID" {
// 			return true
// 		}
// 	}
// 	return false
// }

// // ================================================================================
// // 2. GERENCIADOR DE COMANDOS PERSONALIZADO
// // ================================================================================

// // CustomCommandManager gerencia comandos específicos do seu bot
// type CustomCommandManager struct {
// 	*discordcore.SlashCommandManager
// 	customCommands []discordcore.SlashCommand
// }

// // NewCustomCommandManager cria um gerenciador personalizado
// func NewCustomCommandManager(session *discordgo.Session) *CustomCommandManager {
// 	return &CustomCommandManager{
// 		SlashCommandManager: discordcore.NewSlashCommandManager(session),
// 		customCommands:      []discordcore.SlashCommand{},
// 	}
// }

// // AddCustomCommand adiciona um comando personalizado
// func (ccm *CustomCommandManager) AddCustomCommand(command discordcore.SlashCommand) {
// 	ccm.customCommands = append(ccm.customCommands, command)
// 	ccm.RegisterCommand(command)
// }

// // AddCustomCommandWithOptions adiciona comando com opções
// func (ccm *CustomCommandManager) AddCustomCommandWithOptions(command discordcore.SlashCommand, options []*discordgo.ApplicationCommandOption) {
// 	ccm.customCommands = append(ccm.customCommands, command)
// 	ccm.RegisterCommandWithOptions(command, options)
// }

// // Start registra todos os comandos
// func (ccm *CustomCommandManager) Start() {
// 	for _, cmd := range ccm.customCommands {
// 		// Registrar comandos com opções se necessário
// 		switch cmd.GetName() {
// 		case "userinfo":
// 			options := []*discordgo.ApplicationCommandOption{
// 				{
// 					Type:        discordgo.ApplicationCommandOptionUser,
// 					Name:        "user",
// 					Description: "Usuário para ver informações",
// 					Required:    false,
// 				},
// 			}
// 			ccm.RegisterCommandWithOptions(cmd, options)
// 		case "mod":
// 			options := []*discordgo.ApplicationCommandOption{
// 				{
// 					Type:        discordgo.ApplicationCommandOptionSubCommand,
// 					Name:        "kick",
// 					Description: "Expulsar usuário",
// 					Options: []*discordgo.ApplicationCommandOption{
// 						{
// 							Type:        discordgo.ApplicationCommandOptionUser,
// 							Name:        "user",
// 							Description: "Usuário a ser expulso",
// 							Required:    true,
// 						},
// 						{
// 							Type:        discordgo.ApplicationCommandOptionString,
// 							Name:        "reason",
// 							Description: "Motivo da expulsão",
// 							Required:    false,
// 						},
// 					},
// 				},
// 				{
// 					Type:        discordgo.ApplicationCommandOptionSubCommand,
// 					Name:        "ban",
// 					Description: "Banir usuário",
// 					Options: []*discordgo.ApplicationCommandOption{
// 						{
// 							Type:        discordgo.ApplicationCommandOptionUser,
// 							Name:        "user",
// 							Description: "Usuário a ser banido",
// 							Required:    true,
// 						},
// 						{
// 							Type:        discordgo.ApplicationCommandOptionString,
// 							Name:        "reason",
// 							Description: "Motivo do ban",
// 							Required:    false,
// 						},
// 					},
// 				},
// 				{
// 					Type:        discordgo.ApplicationCommandOptionSubCommand,
// 					Name:        "warn",
// 					Description: "Advertir usuário",
// 					Options: []*discordgo.ApplicationCommandOption{
// 						{
// 							Type:        discordgo.ApplicationCommandOptionUser,
// 							Name:        "user",
// 							Description: "Usuário a ser advertido",
// 							Required:    true,
// 						},
// 						{
// 							Type:        discordgo.ApplicationCommandOptionString,
// 							Name:        "reason",
// 							Description: "Motivo da advertência",
// 							Required:    false,
// 						},
// 					},
// 				},
// 			}
// 			ccm.RegisterCommandWithOptions(cmd, options)
// 		default:
// 			ccm.RegisterCommand(cmd)
// 		}
// 	}
// 	ccm.SlashCommandManager.Start()
// }

// // ================================================================================
// // 3. PROCESSADOR DE EVENTOS PARA COMANDOS
// // ================================================================================

// // CommandAnalyticsProcessor rastreia uso de comandos
// type CommandAnalyticsProcessor struct {
// 	commandUsage map[string]int
// }

// // NewCommandAnalyticsProcessor cria um novo processador
// func NewCommandAnalyticsProcessor() *CommandAnalyticsProcessor {
// 	return &CommandAnalyticsProcessor{
// 		commandUsage: make(map[string]int),
// 	}
// }

// // ProcessEvent processa eventos relacionados a comandos
// func (cap *CommandAnalyticsProcessor) ProcessEvent(event discordcore.Event) {
// 	// Aqui você poderia processar eventos relacionados ao uso de comandos
// 	// Por exemplo, logar quando comandos são executados
// }

// // Start inicializa o processador
// func (cap *CommandAnalyticsProcessor) Start() {
// 	fmt.Println("📊 Command analytics processor started")
// }

// // Stop finaliza o processador
// func (cap *CommandAnalyticsProcessor) Stop() {
// 	fmt.Println("📊 Command analytics processor stopped")
// }

// // ================================================================================
// // 4. EXEMPLO DE USO COMPLETO
// // ================================================================================

// func main() {
// 	// ============================================================================
// 	// CONFIGURAÇÃO DO DISCORDCORE
// 	// ============================================================================

// 	// 1. Inicializar o core do Discord
// 	core, err := discordcore.NewDiscordCore(os.Getenv("DISCORD_TOKEN"))
// 	if err != nil {
// 		log.Fatal("Failed to create Discord core:", err)
// 	}

// 	// 2. Criar sessão
// 	session, err := core.NewDiscordSession()
// 	if err != nil {
// 		log.Fatal("Failed to create Discord session:", err)
// 	}
// 	defer session.Close()

// 	// ============================================================================
// 	// CONFIGURAÇÃO DO SISTEMA DE MONITORAMENTO
// 	// ============================================================================

// 	// 1. Criar serviço de monitoramento
// 	monitoring := discordcore.NewMonitoringService()

// 	// 2. Adicionar processadores
// 	analytics := NewCommandAnalyticsProcessor()
// 	monitoring.AddProcessor(analytics)

// 	// ============================================================================
// 	// CONFIGURAÇÃO DO SISTEMA DE SLASH COMMANDS
// 	// ============================================================================

// 	// 1. Criar gerenciador de comandos
// 	commandManager := NewCustomCommandManager(session)

// 	// 2. Adicionar comandos personalizados
// 	commandManager.AddCustomCommand(&PingCommand{})
// 	commandManager.AddCustomCommand(NewUserInfoCommand(monitoring))
// 	commandManager.AddCustomCommand(&ModerationCommand{})

// 	// ============================================================================
// 	// INICIALIZAÇÃO E LOOP PRINCIPAL
// 	// ============================================================================

// 	// 1. Iniciar sistemas
// 	commandManager.Start()
// 	monitoring.Start()

// 	// 2. Conectar ao Discord
// 	err = session.Open()
// 	if err != nil {
// 		log.Fatal("Erro ao conectar:", err)
// 	}

// 	// 3. Loop principal
// 	fmt.Println("🤖 Bot com slash commands iniciado!")
// 	fmt.Println("   Comandos disponíveis:")
// 	fmt.Println("   - /ping")
// 	fmt.Println("   - /userinfo [user]")
// 	fmt.Println("   - /mod kick|ban|warn <user> [reason]")

// 	sc := make(chan os.Signal, 1)
// 	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
// 	<-sc

// 	// 4. Cleanup
// 	commandManager.Stop()
// 	monitoring.Stop()
// }

// // ================================================================================
// // 5. DICAS DE IMPLEMENTAÇÃO AVANÇADA
// // ================================================================================

// /*
// IMPLEMENTAÇÃO AVANÇADA RECOMENDADA:

// 1. Validação de Permissões:
//    - Sempre verifique permissões antes de executar comandos
//    - Use cargos específicos para diferentes níveis de acesso
//    - Implemente rate limiting para evitar abuso

// 2. Tratamento de Erros Robusto:
//    - Sempre responda às interações, mesmo em erro
//    - Use flags como MessageFlagsEphemeral para respostas privadas
//    - Log erros detalhadamente para debugging

// 3. Comandos com Opções Complexas:
//    - Use ApplicationCommandOptionChoice para opções pré-definidas
//    - Suporte a múltiplos tipos: string, integer, boolean, user, channel, role
//    - Valide entradas do usuário rigorosamente

// 4. Integração com Banco de Dados:
//    - Persista configurações de comandos
//    - Armazene histórico de uso
//    - Cache resultados para performance

// 5. Localização (i18n):
//    - Suporte a múltiplos idiomas
//    - Use arquivos de configuração para textos
//    - Adapte respostas baseadas na localização do usuário

// 6. Testabilidade:
//    - Separe lógica de negócio da integração Discord
//    - Use interfaces para facilitar testes unitários
//    - Implemente mocks para discordgo.Session

// EXEMPLO DE ESTRUTURA DE PROJETO AVANÇADA:

// meu-bot-slash/
// ├── main.go                    # Ponto de entrada
// ├── config/                    # Configurações
// │   ├── bot.go
// │   └── commands.go
// ├── commands/                  # Implementações de comandos
// │   ├── ping.go
// │   ├── userinfo.go
// │   ├── moderation/
// │   │   ├── kick.go
// │   │   ├── ban.go
// │   │   └── warn.go
// │   └── utility/
// │       ├── serverinfo.go
// │       └── help.go
// ├── handlers/                  # Handlers auxiliares
// │   ├── permissions.go
// │   ├── validation.go
// │   └── rate_limiter.go
// ├── services/                  # Serviços de negócio
// │   ├── user_service.go
// │   ├── moderation_service.go
// │   └── analytics_service.go
// ├── database/                  # Camada de persistência
// │   ├── models.go
// │   ├── repository.go
// │   └── migrations/
// ├── utils/                     # Utilitários
// │   ├── embed_builder.go
// │   ├── time_formatter.go
// │   └── string_utils.go
// ├── discord_client.go          # Cliente Discord
// └── tests/                     # Testes
//     ├── commands_test.go
//     ├── handlers_test.go
//     └── integration_test.go

// */
