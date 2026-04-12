# Backlog de Implementacao Acionavel do Discordcore

Consolidado historico desta lista:
- fonte atual de verdade para priorizacao acionavel dentro de `discordcore`
- as notas detalhadas usadas para montar esta lista nao estao todas presentes no worktree atual, entao a priorizacao deve continuar baseada neste arquivo e no source vigente

Escopo deste arquivo:
- manter apenas trabalho implementavel em `discordcore`
- priorizar por impacto e clareza de execucao
- separar `back end Go` de `front end React/Bun`
- registrar tambem questoes transversais de repositorio/operacao quando elas aumentarem risco de manutencao ou drift entre front e back
- evitar reabrir itens ja resolvidos no source atual

Estado atual do audit de `2026-04-12`:
- `go test ./...`, `go vet ./...`, `bun run test`, `bun run lint` e `bun run build` passaram no source atual
- o backlog mais urgente hoje eh estrutural: hotspots centrais, superficie de dashboard incompleta para `maintenance`, e ruido de repositorio que atrapalha review, audit e tooling

## Prioridade 0

### Front end React/Bun

- [ ] Fechar o gap entre `featureAreaDefinitions` e as rotas reais do dashboard para a area `maintenance`.
  Arquivos principais: `ui/src/features/features/areas.ts`, `ui/src/app/navigation.ts`, `ui/src/app/AppRoutes.tsx`, novo page/helper em `ui/src/pages/` ou `ui/src/features/features/`.
  Resultado esperado: `services.monitoring`, `message_cache.cleanup_on_startup`, `message_cache.delete_on_log`, `maintenance.db_cleanup`, `backfill.enabled` e `user_prune` deixam de ficar numa area `advanced` sem superficie roteada; ou passam a ter pagina/navegacao curada, ou viram diagnostico explicito fora da navegação padrao.

- [ ] Separar as responsabilidades hoje concentradas em `DashboardSessionContext`.
  Arquivos principais: `ui/src/context/DashboardSessionContext.tsx`, `ui/src/context/DashboardSessionContext.test.tsx`, `ui/src/features/features/guildResourceCache.ts`, novos hooks/helpers em `ui/src/context/` ou `ui/src/features/features/`.
  Resultado esperado: auth/session, selecao de guild, `baseUrl`, revalidacao em `focus/visibility` e `prefetch` de recursos deixam de morar no mesmo provider; futuras mudancas de sessao deixam de arrastar junto roteamento e preload de workspace.

- [ ] Quebrar os megafiles de workspace antes de continuar expandindo a dashboard.
  Arquivos principais: `ui/src/pages/RolesPage.tsx`, `ui/src/pages/ModerationPage.tsx`, `ui/src/pages/CommandsPage.tsx`, `ui/src/pages/LoggingCategoryPage.tsx`, `ui/src/pages/StatsPage.tsx`, `ui/src/App.test.tsx`.
  Resultado esperado: paginas de rota viram composicao enxuta; drafts, blocos repetidos, parsing de `feature.details`, pickers e resumos saem para `ui/src/features/features/*` ou subcomponentes page-local menores, reduzindo risco de regressao e conflito de merge.

### Back end Go

- [ ] Desmembrar `pkg/control/features_routes.go` em catalogo, montagem de workspace, readiness/blockers e patching.
  Arquivos principais: `pkg/control/features_routes.go`, novos siblings em `pkg/control/`, `pkg/control/features_routes_test.go`.
  Resultado esperado: o contrato de features deixa de depender de um arquivo unico com catalogo, detalhes, mapeamentos, blockers e mutacao misturados; diffs futuros ficam menores e os testes podem isolar leitura, readiness e patch application por feature/area.

- [ ] Fatiar `pkg/control/discord_oauth.go` em modulos menores de provider, session store, callback e guild access.
  Arquivos principais: `pkg/control/discord_oauth.go`, novos siblings em `pkg/control/`, `pkg/control/server_discord_oauth_test.go`.
  Resultado esperado: a superficie de auth deixa de concentrar cookies, refresh de token, persistencia de sessao, chamada a Discord e controle de acesso num unico hotspot; o fluxo fica mais facil de revisar e de testar em isolamento.

- [ ] Quebrar os hotspots operacionais de runtime e persistencia antes de ampliar mais fluxo neles.
  Arquivos principais: `pkg/discord/logging/monitoring.go`, `pkg/storage/postgres_store.go`, testes vizinhos em `pkg/discord/logging/` e `pkg/storage/`.
  Resultado esperado: separar por familia de evento/workflow de storage, reduzir `blast radius` por arquivo e melhorar a capacidade de testar, observar e mudar uma trilha sem reler milhares de linhas nao relacionadas.

### Repositorio e Operacao

- [ ] Remover artefatos rastreados que nao sao source de verdade e blindar o repo contra drift recorrente.
  Arquivos principais: `.gitignore`, `discordcore.exe`, `diff.txt`, `node_modules/.vite/vitest/da39a3ee5e6b4b0d3255bfef95601890afd80709/results.json`.
  Resultado esperado: cache, binario local e diff auxiliar deixam de poluir `git status`, audits e indexacao; preservar apenas o que for realmente canonico para embed/dashboard, incluindo o placeholder exigido em `ui/dist/index.html`.

- [ ] Transformar gaps de contrato descobertos no audit em testes do proprio repo.
  Arquivos principais: `pkg/control/features_routes_test.go`, `ui/src/features/features/areas.test.ts`, `ui/src/App.test.tsx`.
  Resultado esperado: CI falha quando o catalogo expor feature sem superficie roteada ou diagnostica definida, quando `editable_fields` ou coverage de area divergirem, ou quando uma area declarada em `areas.ts` ficar sem navegacao real.

## Prioridade 1

### Front end React/Bun

- [x] Simplificar a `ModerationPage` para o padrao de `direct settings`.
  Arquivos principais: `ui/src/pages/ModerationPage.tsx`, `ui/src/features/features/moderation.ts`, `ui/src/components/groupedSettings.tsx`, `ui/src/App.test.tsx`.
  Resultado esperado: remover status agregado redundante no topo, reduzir `Automod service` para `titulo curto + toggle`, transformar `Mute role` em fluxo `select-first`, reduzir `Moderation routes` para `nome + toggle + select`, e exibir texto secundario apenas quando houver bloqueio acionavel.

- [x] Revalidar e alinhar a experiencia de `QOTD` com as regras atuais de UI.
  Arquivos principais: `ui/src/app/navigation.ts`, `ui/src/pages/HomePage.tsx`, `ui/src/app/AppRoutes.tsx`, `ui/src/features/qotd/QOTDLayout.tsx`, `ui/src/features/qotd/QOTDSettingsPage.tsx`, `ui/src/features/qotd/QOTDQuestionsPage.tsx`, `ui/src/features/qotd/QOTDPages.test.tsx`, `ui/src/index.css`.
  Resultado esperado: manter a feature existente, mas cortar descricoes instrucionais, `meta pills`, resumos redundantes e texto de orientacao que substitui hierarquia visual; aplicar a taxonomia pedida nas notas para o acesso do `QOTD` no sidebar e no `Home`, ou consolidar uma unica taxonomia canonica antes de continuar a limpeza.

- [ ] Fatiar `ui/src/api/control.ts` por dominio sem perder um cliente canonico unico.
  Arquivos principais: `ui/src/api/control.ts`, novos siblings em `ui/src/api/`, consumidores em `ui/src/context/` e `ui/src/features/`.
  Resultado esperado: contratos e requests de sessao, guilds, features, `Partner Board` e `QOTD` deixam de competir no mesmo arquivo de **800+** linhas; o front preserva uma superficie publica unica, mas reduz churn e conflito de merge por area.

### Back end Go

- [ ] Extrair o modelo de `features` e helpers de resolucao de `pkg/files/types.go`.
  Arquivos principais: `pkg/files/types.go`, novos siblings em `pkg/files/`, testes em `pkg/files/`.
  Resultado esperado: `FeatureToggles`, `ResolvedFeatureToggles`, canais correlatos e helpers de resolucao deixam de dividir um hotspot unico com tipos nao relacionados; o `pkg/files` continua sendo fonte canonica, mas com fronteiras internas mais legiveis.

## Prioridade 2

### Front end React/Bun

- [ ] Remover drift de UI nas paginas de logging e stats.
  Arquivos principais: `ui/src/pages/LoggingCategoryPage.tsx`, `ui/src/pages/StatsPage.tsx`, `ui/src/App.test.tsx`.
  Resultado esperado: cortar `workspaceDescription`, descricoes longas por bloco, `Current signal`, tiras de metricas decorativas e notas repetitivas quando o proprio controle ja mostra estado e proxima acao.

- [ ] Fechar a limpeza de drift nas paginas de roles, commands e control panel.
  Arquivos principais: `ui/src/pages/RolesPage.tsx`, `ui/src/pages/CommandsPage.tsx`, `ui/src/pages/ControlPanelPage.tsx`, `ui/src/App.test.tsx`.
  Resultado esperado: remover tags e metadados sobrando, reduzir texto auxiliar desnecessario, manter `grouped settings` coesos e deixar a UI padrao focada em controle, valor atual e bloqueio acionavel.

### Back end Go

- [ ] Criar superficie de dashboard para warnings de moderacao usando a capacidade ja existente.
  Arquivos principais: `pkg/control/features_routes.go`, `pkg/storage/postgres_store.go`, `pkg/discord/commands/moderation/moderation_commands.go`, `ui/src/api/control.ts`.
  Resultado esperado: expor leitura/listagem de warnings no painel sem duplicar regra de dominio no front; manter o primeiro corte limitado ao que ja existe em storage e runtime, a menos que edicao seja realmente necessaria.

- [ ] Decidir e, se fizer sentido, curar controles avancados de logging hoje espalhados em runtime/config.
  Arquivos principais: `pkg/control/features_routes.go`, `pkg/control/server.go`, `pkg/files/types.go`, `ui/src/api/control.ts`.
  Resultado esperado: avaliar se `runtime_config.moderation_logging`, `disable_automod_logs` e `disable_clean_log` viram superficie curada, ficam em diagnostico explicito ou permanecem fora da UI padrao.

## Prioridade 3

### Front end React/Bun

- [ ] Aplicar a mesma direcao de limpeza fora do dashboard principal apenas depois das paginas centrais.
  Arquivos principais: `ui/src/pages/LandingPage.tsx`.
  Resultado esperado: se a regra de `no-tags` tambem valer para a entrada publica, eliminar pills e copy ornamental que ainda desviam do padrao atual.

### Back end Go

- [ ] Adicionar toggles por acao de moderacao se a necessidade continuar valida.
  Arquivos principais: `pkg/files/types.go`, `pkg/control/features_routes.go`, `pkg/discord/commands/moderation/moderation_commands.go`, `ui/src/api/control.ts`.
  Resultado esperado: separar configuracao para `ban`, `massban`, `kick`, `mute`, `timeout`, `warn` e `warnings` sem quebrar o gating atual por `services.commands` e sem espalhar regra de negocio no front.

- [ ] Criar painel real de regras de AutoMod apenas como feature nova isolada.
  Arquivos principais: `pkg/control/features_routes.go`, `pkg/files/types.go`, `pkg/discord/commands/moderation/moderation_commands.go`, `ui/src/api/control.ts`.
  Resultado esperado: sair do estado atual de `logging/listener only` para uma modelagem propria de regras, persistencia, blockers e UX dedicados; nao tratar isso como simples polish da pagina de moderacao.

## Revalidacoes Feitas

- `QOTD` ja existe no repo. O trabalho aberto eh refinamento de UX, hierarquia e taxonomia; nao criacao da feature.
- `UI_RULES.md` ja foi reescrito. O trabalho aberto eh manter o documento sincronizado com o source final depois das limpezas acima.
- O gap antigo sobre `moderation.massban` em `pkg/files/types.go` nao esta mais aberto no source atual; nao tratar isso como follow-up.
- `logging.clean_action` ja esta catalogado no backend, persistido em `pkg/files`, coberto por testes em `pkg/control/features_routes_test.go` e exposto na UI atual; nao tratar isso como follow-up aberto.
- O audit de `2026-04-12` encontrou os checks executaveis verdes; tratar os itens acima como reducao de risco estrutural e de drift, nao como resposta a build quebrado no momento.
