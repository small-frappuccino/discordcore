# DiscordCore

Uma biblioteca modular em Go para bots do Discord que oferece monitoramento abrangente de eventos e gerenciamento de configurações.

## 🚀 Funcionalidades

### ✅ Implementadas

- **📸 Monitoramento de Avatar**: Detecta e registra mudanças de avatar dos usuários
- **🛡️ Logs de AutoMod**: Registra ações do sistema de moderação automática nativo do Discord
- **👋 Eventos de Membros**: Monitora entrada e saída de usuários com informações detalhadas
- **💬 Logs de Mensagens**: Rastreia edições e deleções de mensagens
- **⚙️ Gerenciamento de Configurações**: Sistema flexível de configuração por servidor
- **🔧 Sistema de Comandos**: Framework para slash commands do Discord

### 📋 Características dos Logs

#### Entrada de Usuários
- ✅ Mostra há quanto tempo a conta foi criada no Discord
- ✅ Avatar do usuário
- ✅ Informações de menção e ID

#### Saída de Usuários  
- ✅ Tempo no servidor (limitado - sem dados históricos por padrão)
- ✅ Avatar do usuário
- ✅ Informações de menção e ID

#### Mensagens Editadas
- ✅ Conteúdo antes e depois da edição
- ✅ Canal onde foi editada
- ✅ Autor da mensagem
- ✅ Timestamp da edição
- ✅ Canal separado para logs de mensagens

#### Mensagens Deletadas
- ✅ Conteúdo da mensagem original
- ✅ Canal onde foi deletada
- ✅ Autor da mensagem
- ✅ Indicação de quem deletou (limitado pela API do Discord)
- ✅ Canal separado para logs de mensagens

## 🏗️ Arquitetura

### Componentes Principais

```
discordcore/
├── internal/
│   ├── discord/
│   │   ├── commands/         # Sistema de comandos slash
│   │   ├── logging/          # Serviços de logging e monitoramento
│   │   │   ├── monitoring.go      # Serviço principal de monitoramento
│   │   │   ├── member_events.go   # Eventos de entrada/saída
│   │   │   ├── message_events.go  # Eventos de mensagens
│   │   │   ├── notifications.go   # Sistema de embeds/notificações
│   │   │   └── automod.go         # Logs de automod
│   │   └── session/          # Gerenciamento de sessão Discord
│   ├── files/                # Gerenciamento de arquivos e cache
│   └── util/                 # Utilitários gerais
└── cmd/discordcore/          # Exemplo de implementação
```

## 📦 Instalação

```bash
go get github.com/alice-bnuy/discordcore/v2
```

## 🔧 Uso Básico

### Implementação Simples

```go
package main

import (
    "github.com/alice-bnuy/discordcore/v2/internal/discord/commands"
    "github.com/alice-bnuy/discordcore/v2/internal/discord/logging"
    "github.com/alice-bnuy/discordcore/v2/internal/discord/session"
    "github.com/alice-bnuy/discordcore/v2/internal/files"
    "github.com/alice-bnuy/discordcore/v2/internal/util"
)

func main() {
    // Configurar token
    token := os.Getenv("DISCORD_BOT_TOKEN")
    
    // Inicializar componentes
    configManager := files.NewConfigManager()
    discordSession, err := session.NewDiscordSession(token)
    if err != nil {
        log.Fatal(err)
    }
    
    cache := files.NewAvatarCacheManager()
    cache.Load()
    
    // Inicializar serviços de monitoramento
    monitorService, err := logging.NewMonitoringService(discordSession, configManager, cache)
    if err != nil {
        log.Fatal(err)
    }
    
    // Inicializar automod
    automodService := logging.NewAutomodService(discordSession, configManager)
    
    // Inicializar comandos
    commandHandler := commands.NewCommandHandler(discordSession, configManager, cache, monitorService, automodService)
    
    // Iniciar tudo
    monitorService.Start()
    automodService.Start()
    commandHandler.SetupCommands()
    
    // Logs são enviados para canais separados:
    // - user_log_channel_id: avatares, entrada/saída
    // - message_log_channel_id: edições/deleções de mensagens
    // - automod_log_channel_id: ações de moderação
    
    defer func() {
        monitorService.Stop()
        automodService.Stop()
    }()
    
    // Aguardar interrupção
    util.WaitForInterrupt()
}
```

### Configuração por Servidor

```json
{
  "guilds": [
    {
      "guild_id": "123456789",
      "command_channel_id": "987654321",
      "user_log_channel_id": "111111111",
      "message_log_channel_id": "999999999",
      "automod_log_channel_id": "222222222",
      "allowed_roles": ["333333333"]
    }
  ]
}
```

## 🎯 Serviços Específicos

### MonitoringService
Coordena todos os serviços de monitoramento:

```go
// Inicializar
monitorService, err := logging.NewMonitoringService(session, configManager, cache)
if err != nil {
    return err
}

// Iniciar todos os serviços
err = monitorService.Start()
if err != nil {
    return err
}

// O MonitoringService gerencia automaticamente:
// - UserWatcher (mudanças de avatar)
// - MemberEventService (entrada/saída)
// - MessageEventService (edições/deleções)
```

### Serviços Individuais

#### MemberEventService
```go
// Uso direto (opcional - geralmente gerenciado pelo MonitoringService)
memberService := logging.NewMemberEventService(session, configManager, notifier)
memberService.Start()
```

#### MessageEventService
```go
// Uso direto (opcional)
messageService := logging.NewMessageEventService(session, configManager, notifier)
messageService.Start()

// Ver estatísticas do cache
stats := messageService.GetCacheStats()
fmt.Printf("Mensagens em cache: %d\n", stats["totalCached"])
```

## 🛠️ Personalização

### Implementando Novos Handlers

```go
// Estender o NotificationSender
func (ns *NotificationSender) SendCustomNotification(channelID string, data interface{}) error {
    embed := &discordgo.MessageEmbed{
        Title:       "🔔 Evento Customizado",
        Color:       0x5865F2,
        Description: "Sua lógica customizada aqui",
    }
    
    _, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
    return err
}
```

### Adicionando Novos Comandos

```go
// Implementar na estrutura de comandos existente
func (ch *CommandHandler) registerCustomCommands() error {
    // Sua lógica de comandos customizados
    return nil
}
```

## 🔍 Logs e Debugging

### Níveis de Log
- **Info**: Eventos principais (entrada/saída, mudanças de avatar)
- **Debug**: Cache de mensagens, detalhes internos
- **Error**: Falhas de envio de notificações, erros de API

### Estatísticas
```go
// Cache de mensagens
stats := messageService.GetCacheStats()

// Configurações por servidor
config := configManager.GuildConfig("guild_id")
```

## ⚡ Performance

### Cache de Mensagens
- Armazena mensagens por 24 horas para detectar edições
- Limpeza automática a cada hora
- Proteção thread-safe com RWMutex

### Debounce de Avatares
- Evita notificações duplicadas
- Cache temporal de 5 segundos
- Limpeza automática de entradas antigas

### Verificações Periódicas
- Checagem de avatares a cada 30 minutos
- Inicialização automática de cache para novos servidores

## 🔐 Permissões Necessárias

O bot precisa das seguintes permissões:
- `View Channels`
- `Send Messages` 
- `Embed Links`
- `Read Message History`
- `Use Slash Commands`

### 📝 Configuração de Canais

A biblioteca suporta canais separados para diferentes tipos de logs:

- **`user_log_channel_id`**: Entrada/saída de usuários e mudanças de avatar
- **`message_log_channel_id`**: Edições e deleções de mensagens  
- **`automod_log_channel_id`**: Ações do sistema de moderação automática

Isso permite organizar melhor os logs e configurar permissões específicas por tipo de evento.

## 📚 Limitações Conhecidas

1. **Tempo no Servidor**: Sem dados históricos, não é possível calcular com precisão quanto tempo usuários antigos estavam no servidor
2. **Quem Deletou**: A API do Discord não fornece informação direta sobre quem deletou uma mensagem
3. **Cache de Mensagens**: Mensagens enviadas antes do bot iniciar não são rastreadas para edições

## 🛣️ Roadmap

### Futuras Melhorias
- [ ] Integração com audit logs para detecção de moderadores
- [ ] Persistência de dados de entrada para cálculo preciso de tempo no servidor  
- [ ] Sistema de webhooks para notificações externas
- [ ] Dashboard web para configuração
- [ ] Métricas e analytics avançados

## 📄 Dependências

```go
require (
    github.com/alice-bnuy/errutil v1.1.0
    github.com/alice-bnuy/logutil v1.0.0
    github.com/bwmarrin/discordgo v0.29.0
    github.com/joho/godotenv v1.5.1
)
```

## 📖 Exemplos de Embeds

### Entrada de Usuário
```
👋 Membro entrou
@usuario (123456789)
Conta criada há: 2 anos, 5 meses
```

### Saída de Usuário  
```
👋 Membro saiu
@usuario (123456789)  
Tempo no servidor: Tempo desconhecido
```

### Mensagem Editada
```
✏️ Mensagem editada
@usuario editou uma mensagem em #geral

Antes: Olá mundo
Depois: Olá mundo!!!
```

### Mensagem Deletada
```
🗑️ Mensagem deletada
Mensagem de @usuario deletada em #geral

Conteúdo: Mensagem que foi deletada
Deletado por: Usuário
```

## 🤝 Contribuindo

1. Fork o projeto
2. Crie uma branch para sua feature
3. Commit suas mudanças
4. Abra um Pull Request

## 📝 Licença

Este projeto é uma biblioteca interna. Consulte os termos de uso apropriados.