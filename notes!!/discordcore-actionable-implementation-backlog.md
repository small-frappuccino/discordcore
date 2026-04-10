# Backlog de Implementacao Acionavel do Discordcore

Consolidado historico desta lista:
- fonte atual de verdade para priorizacao acionavel dentro de `discordcore`
- as notas detalhadas usadas para montar esta lista nao estao todas presentes no worktree atual, entao a priorizacao deve continuar baseada neste arquivo e no source vigente

Escopo deste arquivo:
- manter apenas trabalho implementavel em `discordcore`
- priorizar por impacto e clareza de execucao
- separar `back end Go` de `front end React/Bun`
- evitar reabrir itens ja resolvidos no source atual

## Prioridade 1

### Front end React/Bun

- [x] Simplificar a `ModerationPage` para o padrao de `direct settings`.
  Arquivos principais: `ui/src/pages/ModerationPage.tsx`, `ui/src/features/features/moderation.ts`, `ui/src/components/groupedSettings.tsx`, `ui/src/App.test.tsx`.
  Resultado esperado: remover status agregado redundante no topo, reduzir `Automod service` para `titulo curto + toggle`, transformar `Mute role` em fluxo `select-first`, reduzir `Moderation routes` para `nome + toggle + select`, e exibir texto secundario apenas quando houver bloqueio acionavel.

- [ ] Revalidar e alinhar a experiencia de `QOTD` com as regras atuais de UI.
  Arquivos principais: `ui/src/app/navigation.ts`, `ui/src/pages/HomePage.tsx`, `ui/src/app/AppRoutes.tsx`, `ui/src/features/qotd/QOTDLayout.tsx`, `ui/src/features/qotd/QOTDSettingsPage.tsx`, `ui/src/features/qotd/QOTDQuestionsPage.tsx`, `ui/src/features/qotd/QOTDPages.test.tsx`, `ui/src/index.css`.
  Resultado esperado: manter a feature existente, mas cortar descricoes instrucionais, `meta pills`, resumos redundantes e texto de orientacao que substitui hierarquia visual; aplicar a taxonomia pedida nas notas para o acesso do `QOTD` no sidebar e no `Home`, ou consolidar uma unica taxonomia canonica antes de continuar a limpeza.

- [ ] Expor `logging.clean_action` na UI como controle de primeira classe, depois do catalogo backend existir.
  Arquivos principais: `ui/src/api/control.ts`, `ui/src/features/features/areas.ts`, `ui/src/pages/LoggingCategoryPage.tsx`, `ui/src/App.test.tsx`.
  Resultado esperado: a capacidade ja existente em runtime/config passa a aparecer no catalogo, na area correta e com o mesmo tratamento de readiness, toggle e canal usado pelas demais rotas de logging.

### Back end Go

- [ ] Catalogar `logging.clean_action` como feature oficial do dashboard.
  Arquivos principais: `pkg/control/features_routes.go`, `pkg/files/types.go`, `pkg/discord/logging/event_policy.go`.
  Resultado esperado: promover a capacidade ja existente para `featureDefinitions`, com `editable_fields`, blockers e route binding coerentes com as outras rotas de logging, sem criar modelagem nova desnecessaria.

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
