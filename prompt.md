# Refatoração Guiada — `discordcore`

Este é um plano de ação detalhado para refatorar e limpar a base de código do projeto `discordcore`, baseado em uma auditoria de design recente.
Como um assistente IA avançado focado em engenharia (Gemini 3.1 Pro High), sua tarefa é executar estas refatorações seguindo rigorosamente as diretrizes do repositório (`AGENTS.md` e outras regras de arquitetura).

Você deve atuar nas tarefas abaixo. Foque em completar as mudanças com abordagens sistemáticas (ex: um conjunto de refatoração por vez), mantendo a estabilidade.
**Atenção:** Não altere a lógica de negócios da aplicação. O foco deve ser estritamente na refatoração estrutural, eliminação de god-objects, vazamentos de abstração e code-smells descritos.

---

## 🔴 Alta Prioridade (Fazer Primeiro)

### 1. Limpeza de Defensividade Redundante no Storage (Micro - m1)
Existem dezenas de checagens inúteis `if s.db == nil` e `if ctx == nil` em todos os métodos de `pkg/storage/*.go` (como `qotd_store.go`, `postgres_store.go`, etc.).
- **Ação:** Remova **todas** as guardas `s.db == nil` e `ctx == nil` internas do pacote. O `db` já é validado no construtor `NewStore` no boot e o `ctx` é passado de forma correta e garantida pelos callers internos.
- **Objetivo:** Limpar o código e remover a falsa semântica de que métodos de storage podem ser chamados com inicialização parcial.

### 2. Remoção de Campos `Deprecated` da Configuração Pública (Micro - m7)
Em `pkg/files/types.go`, há campos marcados como `Deprecated` ou mantidos apenas para retrocompatibilidade no struct público (ex: em `UserPruneConfig`, `ModerationLogMode`, `WebhookEmbedUpdate`).
- **Ação:** Mova a definição dessas chaves JSON legadas para structs internos exclusivos de unmarshal (ex: `rawXxxConfig`). Remova os campos do struct exportado público e faça a migração de compatibilidade no tempo de decode (`UnmarshalJSON`). Com isso, chaves antigas não serão reescritas ao se fazer o marshal das configurações.

### 3. Decomposição de `moderation_commands.go` (Macro - M3)
O arquivo `pkg/discord/commands/moderation/moderation_commands.go` possui todos os subcomandos e utilitários agrupados em ~1600 linhas.
- **Ação:** Siga o padrão já validado por `clean_command.go`. Separe cada comando em seu próprio arquivo:
  - `moderation_ban_command.go`
  - `moderation_massban_command.go`
  - `moderation_kick_command.go`
  - `moderation_timeout_command.go`
  - `moderation_mute_command.go`
  - `moderation_warn_command.go`
  - `moderation_warnings_command.go`
  - Extraia os mapas estáticos de labels/aliases e funções de helpers para `moderation_audit_actions.go` e `moderation_helpers.go`.

### 4. Decomposição Parcial de `MonitoringService` (Macro - M1)
O arquivo `pkg/discord/logging/monitoring.go` contém métodos colossais, sendo que a função `Start` mistura desde ciclo de vida até fechamentos anônimos (closures) enormes para backfill.
- **Ação:** Extraia a lógica dos handlers de backfill (`TaskTypeMonitorBackfillEntryExitDay` e `TaskTypeMonitorBackfillEntryExitRange`) alocados na função `Start` para um arquivo à parte: `monitoring_backfill.go` no mesmo pacote. Avalie fatiar partes de estado (ex: `rolesCache`, `presenceWatch`) para facilitar a legibilidade se possível, sem causar quebras.

### 5. Forte Tipagem em Payloads do Router (Micro - m10)
Em `monitoring.go`, o consumo de mensagens de backfill através do router usa sucessivos type-assertions e fallbacks genéricos de `map[string]any` ou structs anônimos.
- **Ação:** Defina structs DTO explícitos e exportados em `pkg/task` (ou local, se couber perfeitamente apenas no escopo do monitoramento). Assegure que quem despacha o evento passe essa estrutura concreta e o handler fará apenas um single cast limpo para a estrutura.

### 6. Correção de Vazamento de Dependência `pkg/control` -> `pkg/discord/logging` (Macro - M5)
O `pkg/control` importa diretamente `discord/logging` (por exemplo em `features_readiness.go` e `features_catalog.go`) para tomar decisões se deve habilitar flags ou basear-se em `ShouldEmitLogEvent`.
- **Ação:** Isso quebra a camada de domínio ("do not move Discord runtime behavior into the dashboard layer"). Extraia o contrato ou modelo de decisão (`EmitDecision`, `EmitReason`) para algo neutro em `pkg/files` ou similar. Faça com que o controle e o runtime dependam desse meio-campo, revertendo a inversão errada.

---

## 🟡 Média Prioridade (Trabalho Contínuo)

### 7. Decomposição de `qotd_store.go` (Macro - M2)
O arquivo de banco `pkg/storage/qotd_store.go` possui ~1700 linhas juntando 3 grandes blocos não interligados de QOTD.
- **Ação:** Quebre as responsabilidades nos seguintes arquivos:
  - `qotd_questions_store.go` (métodos de reserva e CRUD)
  - `qotd_official_post_store.go` (postagens provisionadas)
  - `qotd_thread_archive_store.go` (thread tracking)

### 8. Decomposição de `qotd.Service` (Macro - M4)
O arquivo principal `pkg/qotd/service.go` está massivo (+1100 linhas).
- **Ação:** Semelhante à quebra do storage, distribua os métodos da struct `Service` em arquivos de implementação separados dentro do pacote: `service_settings.go`, `service_questions.go`, `service_publish.go`, `service_reconcile.go` e `service_helpers.go`. A interface primária continua intacta.

### 9. Vazamento de Camadas QOTD -> Storage em Control (Macro - M6)
Os arquivos `pkg/control/qotd_routes.go` e afins importam instâncias/tipos direto de `pkg/storage`.
- **Ação:** Refatore as rotas e injete os dados passando estritamente pelos contratos em `pkg/qotd`. A camada de control nunca deve enxergar diretamente os repositórios (DB) nem usar seus structs concretos.

### 10. Refatorações Livres e Estilísticas (Micro)
- **m2**: Use `defer` func com tratativas comuns para erros repetitivos (em storage) evitando o enorme boilerplate de `fmt.Errorf("prefixo repetido: %w", err)`.
- **m3**: Inline wrappers redundantes sem benefício real como `moderationCommandFeatureEnabled`.
- **m4**: Arrume comentários GoDoc quebrados (ex: `UnifiedCache`) para que sigam o padrão stdlib e do repositório (`// TypeName ...`).
- **m5**: Remova anotações de design ou de roadmap dos comentários em código-fonte de produção (ex: em `runtime_config_commands.go`). Apenas use git logs ou pull requests.
- **m6**: Evite nomes com prefixo `Get*` para getters idiomáticos puros em Go. (ex: use `TextChannels` em vez de `GetTextChannels`).
- **m8**: Retire Magic IDs enraizados no código (como IDs de canais, roles padrão de bots ou embeds do Mimu) e coloque em chaves na `RuntimeConfig`.
- **M7**: Em `ui/src/pages/RolesPage.tsx`, crie uma pasta `ui/src/pages/roles/` e isole os drawers e lógicas (ex: `PermissionMirrorDrawerBody`, `PresenceWatchBotDrawerBody`, etc) para resolver a superlotação do arquivo React.

---
## Fluxo e Critérios de Execução (Regras Gerais)
- Siga as etapas na ordem fornecida ou baseada em agrupamentos de alta coesão (ex: consertar todo o QOTD de uma vez).
- Não se esqueça de checar e resolver imports circulares.
- Verifique os testes unitários da respectiva camada se alterar retornos de contratos. Use os scripts corretos para rodar a bateria de validação ao final.
- Quando a tarefa for ambígua, pegue a menor fatia interpretativa e garanta que sua mudança permaneça focada e auditável.
