# DOCUMENTO DE ARQUITETURA E REFATORAÇÃO: MOTOR DE EVENTOS E CONCORRÊNCIA

Este documento consolida a transição estrutural da aplicação, estabelecendo rigorosos limites de memória, isolamento de estado e gerenciamento determinístico do ciclo de vida das *goroutines*. O plano de execução está segmentado em 7 *Pull Requests* (PRs) sequenciais.

---

### **PR 1: Fundações de Infraestrutura e Event Bus**

**Objetivo:** Estabelecer as interfaces de padronização na raiz da aplicação e o barramento de eventos para mutação de configuração, garantindo tolerância a falhas na comunicação entre publicadores e consumidores.

**Ações Consolidadas:**
* **Contratos Unificados:** Definir as interfaces `CommandGroup` (métodos `Register` e `Handle`) e `FeatureService` (métodos `Start`, `Stop` e `WatchConfig`) no arquivo `pkg/app/contracts.go`.
* **Mecanismo de Pub/Sub:** Implementar o padrão `Observer` em `pkg/files/config_mutation.go`, utilizando um *Ring Buffer* ou descarte tático via `select { case ch <- event: default: log.Warn("event dropped") }` para erradicar o risco de *read-starvation* no canal.
* **Segurança de Concorrência:** Isolar a memória global com `sync.RWMutex` ou *atomic pointers*, despachando estruturas `ConfigEvent` estritamente como leitura (valores por cópia ou ponteiros congelados) para impedir mutações acidentais no nível do `AST`.

**Expansão Arquitetural:**
* **Context Propagation:** A estrutura `ConfigEvent` deve carregar um `context.Context` originado pelo *Slash Command*, propagando o `TraceID` (ex: ID da Interação do Discord) por toda a malha da aplicação.
* **Dead-Letter Queue (DLQ) em Memória:** Ao ocorrer um descarte tático (`default` no `select`), registrar ativamente a falha em métricas atômicas (`metrics.DroppedConfigEvents.Add(1)`), permitindo que a camada de observabilidade (Prometheus) dispare alertas sobre saturação crônica do barramento.

**Invariante de Sistema:** Foco estrito no motor de eventos e nas fronteiras das interfaces. Nenhum serviço de domínio sofrerá mutação neste estágio.

---

### **PR 2: Padronização da Camada de Comandos (Command Layer)**

**Objetivo:** Isolar o registro e o roteamento dos comandos, purificando as assinaturas dos construtores e injetando a árvore de execução diretamente no núcleo.

**Ações Consolidadas:**
* **Padronização de Construtores:** Homogeneizar as assinaturas em todas as verticais: `clean.NewCommand`, `embeds.NewCommand`, `logging.NewCommands`, `moderation.NewCommands`, `partners.NewCommand`, `qotd.NewCommand`, `roles.NewCommand` e `stats.NewCommand`.
* **Refatoração de Roteamento:** Modificar `commands_registrar.go` e `commands_handler.go` para consumir exclusivamente um *slice* de `[]app.CommandGroup`.
* **Aceleração de Resolução:** Compilar a matriz dinâmica de comandos em um mapa de inicialização *lazy* (`map[string]CommandHandler`), substituindo buscas lineares de $\mathcal{O}(N)$ por um roteamento em tempo constante de $\mathcal{O}(1)$ no *Feature Router*.
* **Middleware Chain:** Viabilizar injeção nativa de *middlewares* (*Rate Limiting*, controle de permissões) diretamente nos contratos do `CommandGroup` na camada *Adapter*.

**Expansão Arquitetural:**
* **State Syncing via Hashing:** O `CommandGroup` deve expor a estrutura `ApplicationCommand` do Discord. No *boot*, calcular um *hash* da árvore gerada e confrontar com o estado persistido. Acionar a chamada de `Bulk Overwrite` na API externa **apenas** se houver mutação, reduzindo o *boot* de segundos para milissegundos.
* **Contexto Enriquecido:** Injetar no `CommandHandler` um `cmd.Context` modificado, transportando o *logger* da guilda, injeção de dependências e controle transacional de banco de dados por padrão.

**Invariante de Sistema:** A delegação da configuração para persistência permanece isolada; a operação em tempo real dos comandos não deve sofrer qualquer regressão.

---

### **PR 3 e 4: Refatoração dos Serviços (Logging, Moderation, Clean, Embeds)**

**Objetivo:** Transicionar os serviços operacionais e de segurança para o padrão `Watcher`, estabelecendo um gerenciamento cirúrgico e livre de vazamentos do ciclo de vida das *goroutines*.

**Ações Consolidadas:**
* **Adequação de Contrato:** Integrar `logging.NewService`, `moderation.NewService`, `clean.NewService` e `embeds.NewService` à interface `FeatureService` e acoplá-los ao `Pub/Sub`.
* **Idempotência Operacional:** No laço `WatchConfig(ch <-chan ConfigEvent)`, comparar o estado mutante com o atual, aplicando um `noop` instantâneo se a configuração da guilda já estiver em paridade estrutural.
* **Erradicação de Naked Goroutines:** Encapsular todo *hot path* contínuo com um `context.Context` e sua respectiva função `cancel()`. Monitorar passivamente a preempção via `case <-ctx.Done(): return` no evento de desligamento.

**Expansão Arquitetural:**
* **Garantias de Desligamento (Graceful Teardown):** Acoplar um `sync.WaitGroup` interno em cada `FeatureService`. Incrementar `wg.Add(1)` ao instanciar o laço `WatchConfig` e assegurar o decaimento via `defer wg.Done()`. No método `Stop()`, acionar o `cancel()` seguido por `wg.Wait()`, bloqueando o encerramento do bot até que a rotina ateste sua autodestruição limpa.
* **Debounce na Mutação:** Para suprimir *thrashing* causado por rajadas de mutação via comandos administrativos, implementar *Debounce* na recepção dos eventos de recarga de estado utilizando `time.After`.

**Invariante de Sistema:** Transição de estado (inativo para ativo e vice-versa) mecanicamente segura, garantindo vazamento de memória zero atrelado aos *listeners* do *Gateway*.

---

### **PR 5: Gestão de Comunidade (Partners & Roles)**

**Objetivo:** Instaurar determinismo mecânico nas rotinas de alto impacto na rede externa, garantindo tolerância a falhas na `API` de terceiros e limites absolutos de contenção na memória.

**Ações Consolidadas:**
* **Reatividade Passiva:** Refatorar `partners.NewService` e `roles.NewService` para executarem processamento condicionado estritamente à validação prévia de aprovação pela camada de comandos superior.
* **Isolamento Físico de Estado (Bulkhead Pattern):** Segregar o `Worker Pool` de parceiros das rotinas de delegação de cargos, impedindo que degradações de tempo de resposta em um provedor contaminem o barramento do outro.
* **Limitação de Vazão de Rede:** Operar todas as requisições externas através de um algoritmo *Token Bucket* (`golang.org/x/time/rate`) sob gestão de um `sync.WaitGroup`, eliminando punições de infraestrutura como `HTTP 429`.

**Expansão Arquitetural:**
* **Circuit Breakers:** Transpassar o `rate.Limiter` em um disjuntor de falhas (`sony/gobreaker`). Na detecção de 5 erros sequenciais (`HTTP 5xx`), romper o circuito para rejeitar ingressos instantaneamente e estancar a utilização ociosa de `CPU` das *goroutines*.
* **Exponential Backoff com Jittering:** Ao esgotar taxas de limite legítimas (`HTTP 429`), recalcular tentativas com dispersão matemática não-linear (*Jitter*) para anular a colisão simultânea de repetição, evitando o colapso de "Manada Estouro" (*Thundering Herd*).

**Invariante de Sistema:** O mecanismo de engajamento horizontal da guilda deve expandir a ocupação sem violar restrições termodinâmicas ou de requisições de barramentos externos.

---

### **PR 6: Serviço de Telemetria (Stats)**

**Objetivo:** Coletar métricas microscópicas com preempção ativa, neutralizando as contenções de *cache* e mitigando picos esporádicos (*micro-bursts*) em rotas críticas da aplicação.

**Ações Consolidadas:**
* **Coleta Preemptiva:** Refatorar o adaptador `arikawa_adapter` e o núcleo de `stats.NewService` para extrair telemetria passivamente na fronteira das integrações, respeitando escopos de encerramento.
* **Mitigação de Micro-Bursts (Batching Semantics):** Impedir engarrafamento de I/O em picos via contadores globais com estado atômico de memória RAM (`sync/atomic`), resolvendo mutações em complexidade estrutural linear instantânea.
* **Drenagem Periódica de Alta Performance:** Utilizar um `time.Ticker` para executar descargas do sumário atômico para rede/disco. Esse padrão comprime e neutraliza os passivos temporais, transmutando degradações sucessivas de $\mathcal{O}(N)$ para uma descarga resolvida em tempo constante de $\mathcal{O}(1)$ por clico.

**Expansão Arquitetural:**
* **Simpatia de Cache (Cache Alignment):** Estruturas expostas a mutações incessantes via `sync/atomic` estão suscetíveis ao cisma de *False Sharing* nas linhas de *cache* L1/L2. Isolar fisicamente os ponteiros sensíveis forçando *padding* em *structs* (ex: `_ [64]byte`), bloqueando a invalidação inter-núcleos.
* **Zero-Allocation Pipeline:** Envolver os *payloads* em lotes transitórios destinados à rede sob tutela de um `sync.Pool`. Absorver a pressão do *Garbage Collector* com empacotamento contínuo, reaproveitando alocações sem sacrificar CPU no descarte.

**Invariante de Sistema:** Coleta de telemetria operando paralelamente de maneira etérea, impondo latência e impacto igual a zero absoluto em barramentos de comando e respostas HTTP.

---

### **PR 7: The Big Switch (Runner & Matriz de Concorrência)**

**Objetivo:** Centralizar orquestração no bloco terminal da aplicação, suprimindo fiação manual de inicialização e instituindo a arquitetura definitiva para desligamento reativo à orquestração por *containers*.

**Ações Consolidadas:**
* **Purificação Central:** Esvaziar integrações e injeções acopladas de dentro de `pkg/app/runner.go`.
* **Inversão de Dependências Direta:** Prover os vetores abstratos consolidados `[]app.FeatureService` e `[]app.CommandGroup` via contêiner mestre de injeção de dependência (`DI`).
* **Malha via Supervisor Tree:** Transferir concorrência terminal para o pacote padrão expandido `errgroup.Group`, alocando concomitantemente `.Start(ctx)` e `.WatchConfig()`. Toda e qualquer discrepância de inicialização instigará um *panic fail-fast* para forçar a recusa e queda limpa na orquestração.
* **Phased Graceful Shutdown:** Instituir interceptador central de `SIGTERM` contendo decaimento programado: bloquear fronteiras de rede, esgotar *Worker Pools* inflight, fechar conexões `Postgres`.
* **Terminação de Última Instância:** Incorporar toda a rotina de declínio dentro de um escopo blindado por `context.WithTimeout(ctx, 15*time.Second)`. Caso esgote a tolerância máxima, forçar a expiração violenta por intervenção nuclear via `os.Exit(1)`.

**Expansão Arquitetural:**
O isolamento assíncrono terminal sob premissa de contêineres (*Phased Teardown*) se comportará obrigatoriamente nesta cronologia atômica:
1.  **Fase 1 (Recusa de Ingestão):** `signal.NotifyContext` capta a anomalia (`SIGTERM`). O *socket* do Discord é cessado. Receptores de *Webhooks* retornam imediatamente `HTTP 503`. O barramento estanca recebimentos pendentes.
2.  **Fase 2 (Drenagem Ativa):** Injeção de `cancel()` desce em cascata para destruir os galhos das árvores de `Context`. O `errgroup.Wait()` mantém a paralisação do processo, supervisionando exaustivamente enquanto *FeatureServices* e rotinas processam a limpeza do estado mutante e o recuo dos laços `WatchConfig`.
3.  **Fase 3 (Colapso de Infraestrutura):** Condicionado ao desbloqueio final de `errgroup.Wait()`, executam-se as chamadas em cadeias persistentes, desligando pontes de `db.Close()`, Redis e afins, blindando contra desmoronamentos com conexões ativas ainda processadas por *handlers*.

**Invariante de Sistema:** Disponibilidade implacável sob isolamento completo; compiladas todas as peças e resolvido o `Supervisor Tree`, a matriz central estará isolada da entropia gerada por alterações de configuração via painéis de *gateway*.