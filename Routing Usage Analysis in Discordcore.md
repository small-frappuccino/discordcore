## Assistant

Aqui está um raio‑X do ”routing” que você já tem (slash commands e tasks) e o que ainda não está usando, com estimativa de cobertura e caminhos práticos para aumentar o uso.

Resumo
- Existem dois tipos de “routing” na codebase:
  - Slash Command Router: CommandRouter em internal/discord/commands/core.
  - Task Router: TaskRouter em internal/task (com NotificationAdapters).
- Uso atual estimado:
  - Linhas de código usando diretamente CommandRouter ou TaskRouter: ~2.372 LOC de 9.672 LOC no repo ≈ 24,5%.
  - Se incluir também a implementação do próprio framework de routing (arquivos do core e task/router), o total relacionado a routing sobe para ~4.648 LOC ≈ 48,1%.
- Cobertura por módulo/feature:
  - Comandos (slash): usando o router para o grupo admin (OK, mas pouco abrangente).
  - Monitoramento/Logs: usando TaskRouter via NotificationAdapters (OK).
  - Automod: não usa TaskRouter (apenas handler direto do Discord).

Como calculei
- Total de Go LOC no repo: 9.672.
- LOC que “usam” routing (referenciam ou integram CommandRouter/TaskRouter):
  - internal/discord/commands/admin/service_commands.go: 626 (registra subcomandos no CommandRouter)
  - internal/discord/commands/handler.go: 78 (cria CommandManager/CommandRouter)
  - internal/discord/logging/monitoring.go: 424 (cria TaskRouter, agenda cron e injeta NotificationAdapters)
  - internal/discord/logging/message_events.go: 446 (usa adapters.EnqueueMessageEdit/Delete)
  - internal/discord/logging/member_events.go: 221 (usa adapters.EnqueueMemberJoin/Leave)
  - internal/task/adapters.go: 399 (registra handlers e faz Dispatch/Enqueue)
  - cmd/discordcore/main.go: 178 (registra comandos no router e inicializa adapters via Monitoring)
  - Soma: 2.372 LOC ≈ 24,5% do total.
- Implementações do framework de routing (não contei como “uso”, mas podem ser somadas como “relacionadas a routing”):
  - internal/discord/commands/core/*.go: 2.276 LOC
  - internal/task/router.go: 556 LOC
  - Total relacionado a routing (uso + implementação): 2.372 + 2.832 = 4.648 LOC ≈ 48,1%.

Onde NÃO está usando routing (e o que dá para fazer)
1) Automod
- Hoje: internal/discord/logging/automod.go registra handler direto no Session e envia embed via NotificationSender. Não passa pelo TaskRouter.
- O que dá para fazer (100% viável com APIs atuais):
  - Introduzir um task type para automod (ex.: notifications.automod_action) e registrar handler nos NotificationAdapters.
  - No handleAutoModerationAction, em vez de enviar diretamente, fazer um adapters.Router.Dispatch(...) (ou criar um adapters.EnqueueAutomodAction(...) de conveniência, seguindo o padrão dos outros métodos).
  - Benefícios: retry, backoff, idempotência, serialização por guild, telemetria consistente do pipeline.
- Trabalho estimado: baixo. Criar payload + registrar handler + alterar o producer.

2) Slash Commands além de “admin”
- Hoje: só o grupo admin é registrado no router. O handler.go sugere “config commands” automáticos, mas não encontrei registro implícito no CommandManager. Os exemplos em core/examples.go (ping/echo/config) não estão ativos.
- O que dá para fazer (com APIs atuais):
  - Trazer os comandos de exemplo para produção:
    - Criar um pacote internal/discord/commands/config (seguindo o padrão dos admin) e registrar no router.
    - Opcional: adicionar ping/echo para validação do fluxo.
  - Ajustar CommandHandler.registerConfigCommands() para efetivamente registrar os comandos desejados no router (em vez de só logar).
- Trabalho estimado: baixo a médio. A infraestrutura já existe e os exemplos estão prontos.

3) Event “routing” (opcional, adição nova)
- Hoje: os serviços de logging registram vários session.AddHandler(...) diretamente.
- Se quiser ampliar o conceito de “routing” para eventos do Discord, dá para criar um EventRouter:
  - Interface simples para registrar handlers por tipo de evento + filtros (guild, canal, feature flags).
  - Centraliza a inscrição no Session e permite enable/disable por guild/feature sem mexer nos serviços.
- Trabalho estimado: médio. Benefícios de organização e toggles por guild/feature.

Gaps e ajustes rápidos recomendados
- Ativar os “config commands” (ou remover o comentário que diz que são automáticos):
  - Registrar de fato o comando de config no CommandHandler.registerConfigCommands() ou no main.
- Padronizar notificação via TaskRouter no Automod:
  - Criar TaskTypeNotificationsAutomod e handler correspondente em NotificationAdapters.
  - Alterar automod.go para produzir tasks (com GroupKey por guild e idempotência curta).

Roadmap sugerido
1) Curto prazo (maior ROI, 1–2 dias):
- Mover config (e opcionalmente ping/echo) de examples.go para um pacote real e registrar no router.
- Integrar Automod ao TaskRouter com novo task e handler.
- Resultado esperado: % de uso de routing sobe de ~24,5% para algo perto de 30–35% (medindo por LOC), e 100% das notificações passam por um pipeline resiliente.

2) Médio prazo:
- Introduzir EventRouter opcional para centralizar AddHandler e habilitar toggles por guild.
- Unificar fallbacks “adapters == nil” (hoje ainda existe em member_events.go e message_events.go) se no runtime sempre configuramos adapters no MonitoringService.

Checklist de aceitação
- Slash commands (admin + config) sincronizados com Discord e roteados pelo CommandRouter.
- Automod enviando logs via TaskRouter (retries, backoff).
- “adapters.Stats()” e “router.Stats()” (ou logs) mostrando handlers registrados e tasks fluindo.
- Logs mostram menos envios diretos e mais “Dispatch”/“Enqueue”.

Se quiser, eu abro PRs incrementais para:
- Promover “config commands” de examples para produção e registrar no router.
- Integrar Automod ao TaskRouter criando o task e o handler nos NotificationAdapters.
