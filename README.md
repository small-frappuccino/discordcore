# Contrato de Agente `discordcore`

Este documento atua como a diretriz unificada para a operação do agente de IA no repositório `discordcore`. Ele funde o protocolo de comunicação humano-IA com os invariantes estruturais e arquiteturais do sistema, exigindo execução mecânica estrita, determinismo de hardware e anulação de incertezas especulativas.

## 1. Persona Central e Protocolo de Execução

* **Assunção e Execução Imediata (`Chunked Pre-Fill` Conceitual):** Maximize a ocupação vetorial de respostas. Diante de variáveis ausentes ou parâmetros ambíguos, rejeite a paralisação do ciclo de execução para requerer clarificação. Assuma instantaneamente a topologia técnica mais lógica e de ponta, declare essa premissa em uma única frase e entregue o bloco de código executável.
* **Objetividade Clínica e Rigor de Engenharia:** Opere como um arquiteto de sistemas sênior. Comunique-se sob estrita neutralidade analítica. Subtraia preâmbulos motivacionais, adjetivos de aprovação (e.g., "perfeitamente", "com sucesso") e encerramentos parabenizadores. A progressão de estado deve ser demonstrada exclusivamente através das ações executadas e do código gerado.
* **Isolamento de Carga Útil (`Zero-Shot Payload`):** Suprima o histórico basal e o contexto teórico. Omitir a latência na inicialização da resposta. O primeiro *token* gerado deve mapear diretamente o invariante de carga, a falha estrutural ou a mecânica de alocação requisitada.
* **Certeza Epistêmica Binária:** A incerteza opera sob limites binários. Entregue fatos técnicos com absoluta convicção. Caso atinja uma barreira de conhecimento, isole a lacuna explicitamente e forneça a rota determinística de validação, obliterando qualquer viés probabilístico.
* **Rigor Transacional e Rejeição a Marcadores:** O código emitido deve ser completo, estritamente executável e destituído de marcadores silenciados (`// ... existing code ...`). Ignore formatações superficiais e concentre-se na integridade estrutural, *race conditions* e anomalias de acesso concorrente.
* **Mecânica de Tradução Restrita:** As respostas descritivas e analíticas exigem uso formal e estruturado da língua portuguesa. Identificadores de código, APIs, `flags` de compilador, estruturas e topologias do sistema mantêm-se rigorosamente confinados ao idioma inglês para preservar a paridade de execução.
* **Validação de Fator de Causa (`Burden of Proof`):** Afirmações arquiteturais exigem prova empírica. Valide caminhos de código com saídas de teste, matrizes de alocação ou mapeamento de rotinas. Descrições narrativas são insuficientes.

## 2. Invariantes Mecânicos de Linguagem Go e Determinismo Topológico

A avaliação e geração de código Go transcende a sintaxe superficial. A linguagem deve ser interpretada através de uma lente de mecânica analítica estrita, operando como uma negociação de limites físicos com o compilador, o agendador de *runtime* e o *Garbage Collector* (`GC`), em paralelo à eficiência observada em arquiteturas `TPU` e transformadores dinâmicos.

* **Disciplina de Alocação e Fronteiras de Memória (`XLA Allocation Parity`):**
* O *heap* representa um vazamento de estado. Toda alocação transbordada impõe uma latência de bloqueio sobre o `GC` (análogo à saturação do barramento `CXL`).
* **Zero-Alocação por Padrão:** As funções operam estritamente sobre memória alocada pelo chamador. Fatias (`slices`) e `buffers` fluem pela `stack`, preservando as fronteiras físicas de isolamento.
* **Erradicação de `any` e `reflect`:** A tipagem forte é inegociável. A utilização de `any` força o *boxing* na alocação de memória e degrada a integridade da análise estática em tempo de execução.
* **Dimensionamento Estrito:** Estruturas contíguas e matrizes exigem mapeamento prévio de capacidade (`make([]T, 0, capacity)`). Reallocações ocultas durante o *runtime* constituem falhas estruturais.
* **Compressão via `sync.Pool`:** Rotas de alto processamento que requerem *buffers* efêmeros utilizam o *pooling* estratégico para amortizar a complexidade espacial, estabilizando a retenção $\mathcal{O}(1)$.


* **Diagnóstico de Anomalias de Concorrência e Estabilização:**
* Isolamento de estado dita a resiliência do sistema. Rejeite incondicionalmente *goroutines* nuas; instanciar blocos assíncronos sem um ciclo de vida externamente orquestrado incorre em hemorragia de memória irrecuperável.
* **Bloqueio e Fome de Escrita (`Write-Starvation`):** A utilização de `sync.RWMutex` para acessos simultâneos não é estocasticamente gratuita. Sob ocupação vetorial severa, a esteira de leitura estrangula os ciclos de escrita, gerando picos de latência. Quando a proporção leitura/escrita não for excepcionalmente assimétrica, recorra à contenção primária com `sync.Mutex` ou variáveis atômicas `lock-free`.
* **Dimensionamento de `Channels`:** Os canais atuam como protocolos de sinalização, não topologias de enfileiramento genéricas. Requeira canais não-bufferizados para impor sincronização absoluta. Canais bufferizados exigem justificativa matemática rigorosa limitando a profundidade da fila estritamente para a absorção de micro-picos previstos.


* **Orquestração de Ciclo de Vida e Validação Causal (`Errgroup & Context`):**
* O contexto (`context.Context`) atua como o árbitro absoluto da demolição de instâncias. O pacote flui apenas pelo grafo descendente das chamadas, rejeitando estritamente o confinamento estático em `structs`. Ele comanda a cedência dos ciclos e impede que o afunilamento de entrada trave todo o sistema.
* A concorrência deve ser envelopada pelo `golang.org/x/sync/errgroup`. Ancorando a invalidação no primeiro erro assíncrono emitido, o sistema executa o *teardown* simultâneo de processos paralelos nativos, impedindo o aquecimento ocioso de *requests* já rompidas (operando em paridade à máscara $M_{\text{tree}}$ em validações divergentes de estado).


* **Desacoplamento Estrutural e Topologia Isolada:**
* **Injeção Explícita de Dependências:** Remova as variáveis globais e as mutações ocultas providas pelo comando `init()`. Elas fundem lógicas desconectadas e obstruem testes determinísticos $\mathcal{O}(1)$. Adote matrizes guiadas exclusivamente por construtores estritos.
* **Inicialização `Fail-Fast`:** A topologia primária valida todas as âncoras na inicialização de rotina (`main()`). Se variáveis geográficas, topológicas ou conexões atômicas estiverem ausentes, invoque o `panic` instantâneo no limite estrutural do núcleo, recusando falhas em cascata fragmentadas durante o processamento de usuários em *runtime*.



## 3. Arquitetura Estrutural e Fronteiras Físicas

O arquivo `ARCHITECTURE.md` reflete um mapeamento 1:1 do grafo de dependências do Go. Utilize-o para forçar limites rígidos de isolamento:

* **`cmd/*`**: Pontos de entrada do ciclo de vida. Bloqueada a inserção de lógica reutilizável.
* **`pkg/app`**: Motor de inicialização de *bootstrap*. Conecta parâmetros de fiação, persistência matricial e sessões do Discord.
* **`pkg/control` & `pkg/task**`: APIs `HTTP`, entrega do `dashboard` e roteamento de operações assíncronas. O vetor deve ser isolado do fluxo de eventos contínuos de Gateway do Discord.
* **`pkg/discord/*`**: Receptores do Discord. Mapeiam o estado do `DiscordGo SDK` para a lógica interna. Comandos de varredura (*Slash-commands*) operam como vetores primários; o `dashboard` atua apenas em camada complementar diagnóstica.
* **Módulos Verticais (`pkg/automod`, `pkg/qotd`, etc.)**: Operações exclusivas de domínio. Exigem isolamento para testes determinísticos sem interligação acoplada à matriz síncrona do Discord.
* **`pkg/files` & `pkg/storage**`: Subsistema de fundação para parâmetros e persistência vetorial de discos em `Postgres`.
* **`pkg/service`**: Demolição e controle de execução. Todos os processos em plano de fundo devem operar sob `ServiceIdentity`, `ServiceLifecycle` e `ServiceObservability`, confinados no orquestrador `ServiceManager`.

## 4. Invariantes de Engenharia e Operação

Regras estabelecidas nas malhas centrais (`pkg/service/manager.go`, `pkg/storage/store.go`):

* **Monitoramento e Resiliência de Estado:** Proteja a reescrita matricial de memória utilizando as barreiras de `sync.RWMutex`. Limite as seções críticas a nanossegundos; jamais aplique *I/O* em redes de tráfego com registros acoplados. Evite estrangulamento serial de memória por *locks*.
* **Contrato de Injeção de Segurança:** Reverta as rotas silenciadas (`logger` nulo) delegando estado para `slog.Default()`. Envie mensagens de erro determinísticas diante de pacotes faltantes em vez de forçar interrupção térmica via `panic` fora do ambiente de *bootstrap*. Variáveis estruturais, como barramentos de métricas, precisam estar travadas antes da injeção no ciclo central.
* **Validação de Exceções:** Todo erro inspecionável requer empacotamento restrito contextual (`fmt.Errorf("operation: %w", err)`). Limite a avaliação condicional ao mecanismo estrito de validação com `errors.Is`/`errors.As`.
* **Topologia de Observabilidade (`slog`):**
* **Debug:** Estados efêmeros, volumetria integral de `payloads`, desvios vetoriais brutos. Ausente do ambiente produtivo.
* **Info:** Telemetria fundamental e transições topológicas confirmadas.
* **Warn:** Sobrecarga mitigada (e.g., retrocesso de conexão, taxa limítrofe batendo em $80\%$).
* **Error:** Falhas na matriz de fundação, interrompendo o ciclo primário. Obriga o envio explícito de IDs de requisição e rastro de memória (*stack traces*), invocando métricas de alerta.
* Estritamente proibido o trânsito aberto de senhas, conexões OAuth, chaves assimétricas e eventos de canal privado. Formate tensores de estatísticas nas pontas geradoras (`ServiceMetric`).



## 5. Modernização Contínua e Refatoração Autorizada

**Matriz de Modernização para Go 1.26:**

* Intercepte vetores isolados fatiados da base de dados e retornos em *array* obsoletos, substituindo-os pelo mecanismo iterativo nativo `iter.Seq` e `iter.Seq2`. O pipeline da camada Postgres deve escoar retornos de forma assíncrona cruzando a fronteira do `sql.Rows`.
* Substitua a ancoragem sintética de `structs` que encerram blocos cruzados por mapeamentos de alias genéricos estritos (`type Alias[T any] = OriginalType[T]`).
* Aplique transições topológicas de `runtime.SetFinalizer` alterando o bloco funcional restrito para `runtime.AddCleanup`.
* Comprima as rotas dispersas de mapeamento por ponteiros para as instruções compactadas nativas (`new(expr)`).

**Classe Restrita de Refatorações:**
Refatorações exigem confirmação absoluta da utilidade sem acúmulo genérico especulativo:

1. Migração restrita de algoritmos `iter.Seq` / `iter.Seq2`.
2. Supressão definitiva de ramificações inoperantes (com provação restrita em `gopls references`).
3. Composição unitária de injeção por `new(expr)`.
4. Rotação sistêmica para `AddCleanup`.
5. Convergência da geometria de tipos genéricos (`Type Alias`).
6. Remoção de blocos `sync.Map` legados confiando no `HAMT` introduzido na versão 1.24.

*Todas as otimizações validadas devem ocorrer integralmente através da malha de conectores do sistema de uma única via atômica do repositório em um commit fechado.*

## 6. UI, `Dashboard` e Contratos de `TypeScript`

* **Rota Primária de Interface:** O limite base opera em `/manage/` (`/dashboard/` é arquitetura fóssil).
* **Tipagem de Camada Externa:** Imponha mapeamento forte em `ui/src/api/control.ts`.
* **Topologia de Recuperação Termodinâmica:** Aplique limites rígidos em malhas com restrições exponenciais de latência e entropia assimétrica (*randomized network jitter*) para absorver rupturas HTTP 502/504.
* **Confinamento de Taxa de Estado:** Um comando acionado sobre um domínio individual (Guild) jamais invocará atualizações matriciais sistêmicas no `ConfigManager` ou causará repaginação universal na camada de interface gráfica. Restrinja as mudanças globais aos esquemas do sistema nativo de repouso global.
* **Topologia de UI:** Aplique chamadas por `DashboardSessionContext` preservando os atalhos de camada (`PageHeader`, `SurfaceCard`). Evite falhas latentes descartando `any`. O trânsito em vias de rota ativas deve usar restritamente `import.meta.env.BASE_URL` para o carregamento dos pacotes da raiz instalada.

## 7. Padrões de Evolução Geométricas de Schema

Quando os parâmetros primários alocados na memória de banco cruzarem para um novo formato estrito:

1. Adicione a chave nova declarada com publicidade vetorial contida no arquivo `pkg/files/types.go`.
2. Preservar o ponteiro fóssil fatiado de recepção estrutural estritamente à matriz nativa interna na operação transitória `raw*`.
3. Executar o alinhamento das chaves legadas e matrizes canônicas ao transitar pela via decodificadora; cancele qualquer persistência da chave obsoleta nas escritas ativas subjacentes.
4. Garanta isolamento simultâneo na replicação, calibração local e validação nativa de ausência (`IsZero`).

* **Controle Concorrente Otimista:** Toda decodificação desprovida do limite matricial `config_version` incorre em corte imediato de verificação. Os emissores de origem devem impor retração em ciclo exponencial frente a blocos falhos da malha (`HTTP 412/428`).

## 8. Segurança de Sentinela e Invariantes de Carga

* **Paradigma Base de Roteamento de Identidade:** O motor interno opera estritamente encapsulando o estado "genérico da unidade neural base". O identificador nulo `""` aponta para alvos genéricos reversos (fallback). Ele bloqueia sua rota nativa, impedindo absorção universal arbitrária.
* **Bloqueio Topológico Restritivo (`Sentinel Safety`):** Implemente sinalizadores restritos como `<unrouted>` garantindo o limite para travas de conexão sem alocação ou zonas perdidas presentes no dicionário de ramificação nativa, obliterando a utilização permissível de chaves nulas `""`.
* **Resolução de Malha Dinâmica:** Os cálculos associativos de operação devem avaliar as instâncias baseadas estritamente em `FeatureRouting`, derivando dados restritos de fronteira simples de forma imutável antes da absorção reativa, compondo a lógica base em infraestruturas distribuídas.
* **Isolamento de Estado (`Discord Logic`):** Comandos centrais matriciais (como as subrotinas Estatísticas e `QOTD`) operam na camada mecânica e devem estar em total distanciamento da decodificação primária bruta contida em `pkg/discord`. As alocações nativas do *gateway* requerem roteadores não ligados a pacotes operacionais.
* **Orquestração Assíncrona Estrita (Idempotência e Frequências de Base):** Reescrita do barramento principal e atualizações assíncronas do núcleo requerem que se trave a alocação nativa via mapa primário limitante `inflight` restrito ao pacote `pkg/task`, e se limite estritamente a fila reativa nativa ligada por `heap` a fim de absorver ramificações especulativas errôneas. Rejeite o uso arbitrário de operações `goroutines` desassociadas para funções com estado local mutável.

### Subsistema `QOTD`

* **Tolerância Absoluta de Transmissão:** O isolamento mecânico é sustentado por tripla verificação de tráfego: uma chave base `Nonce` 16-hex na malha `QOTDOfficialPostRecord`, fragmentação parcial do banco de repouso subjacente `(guild_id, deck_id, publish_date_utc, publish_mode='scheduled')` e trilha de auditoria e contingência para restabelecimento através dos eixos parciais de reabastecimento via threads do aplicativo primário (`Discord`).
* **Estados Termodinâmicos de Integração:** O avanço unificado por chaves na ponta paralela do fluxo requer estrita progressão canalizada por `applyOfficialPostThreadTransition`. Identifique as instâncias locais divergentes corretamente (`isMissingDiscordThreadError`, etc.).
* **Isolamento Repetitivo:** Em chamadas recorrentes ligadas ao ciclo vital assíncrono nativo de alocação de malha externa, o módulo central contido em `pkg/discord/qotd/RuntimeService` obrigatoriamente aplica reinicialização no barramento de trava nativa com acionamento nos ponteiros subjacentes estritos em `stopCh` e `stopOnce` engatados durante a função `Start` restrita nas ramificações que absorveram um encerramento temporal (`Stop`).

## 9. Anotações e Documentação de Arquitetura

* **Matriz Analítica Limitada:** Todas as sentenças limitadas nas esferas exportáveis exigem terminação total pontuada. As análises de origem se alinham e carregam o preâmbulo imediato usando somente os caracteres primários exatos correspondentes do sinalizador nomeado. Empregue chaves unitárias estruturadas com dupla barra in-line (`//`).
* **Comandos Subjacentes Externos:** Identifique estritamente a diretiva de roteadores isolados das malhas adjacentes, por exemplo `//go:build`, `//go:noescape`, `//go:linkname`, `#cgo`.
* **Esclarecimento Mecânico de Motivo:** Injeções comentadas nativas descrevem a mecânica topológica que ditou a restrição imposta (e.g., saturações evitadas, limite posicional), banindo narrativas supérfluas apontadas meramente para designações das variáveis de memória.
* **Sanitização Vetorial (Sem Fósseis):** Omitir inteiramente a matriz textual obsoleta de estado silenciado por desuso dos ciclos.
* **Orquestração Limitante de Rota (TODOs):** Comandos assíncronos não concluídos retêm alocação nominal estritamente anexando dono unitário (`TODO(user): ...`) ligado com alvo causador rastreável determinístico.

## 10. Barramento de Liberação, Teste e Validação

Todas as diretivas obsoletas conectadas nativamente à raiz livre dos vetores Git se encapsulam dentro da chave unificada controlada pela alocação CLI restrita do barramento `release`.

* **Autenticação Mecânica Estrita:** Lance as etapas unitárias locais primárias acionadas pela rota restrita do núcleo `release validate` (`go vet ./...`, `gofmt -w .`, integridade estrutural do repouso do barramento do sistema operacional nativo associado com git `eol`, e o roteador de limite estático `bun run lint`).
* **Inserção Síncrona:** Confirme a submissão nativa estrita operando sobre as malhas consolidadas limitadas no barramento chamando a subrotina `release "<conventional commit subject>"`.
* **Rigor na Verificação Base de Retenção Absoluta:** O validador integrado paralelo limitante (`release verify`) submete ao tráfego assíncrono vetorial severo a simulação da rotina da matriz operacional nativa juntando os mapeamentos pesados das raízes integrativas e avaliando a integridade do perímetro estático chamando as subrotinas restritas via pacote fundamental de barramento `govulncheck`.
* **Parâmetro Especulativo Local:** Os agentes de diagnóstico retêm a liberdade de testar o vetor topológico alocado subjacente, ancorando os fluxos paralelos via malha em: `postgres://alice:Cpu7zyuwBKdEtcq1QnBg@127.0.0.1:5432/alicemainsdevelopment?sslmode=disable`.
* **Diretriz Causal Paralela para Retenção Local:** Testes impulsionados limitados forçam chamadas puras nativas aplicadas sobre as lógicas da progressão matemática nativa isolando interfaces reduzidas (mockagem de complexidade linear e topológica estreita). Proximidade restrita aos barramentos de teste isolados locais.

## 11. Densidade Sistêmica e Zonas de Risco Estrutural

As chaves abaixo mapeiam os afunilamentos radiais onde ocorrem modificações de limite máximo de ocupação de máquina, determinando contenção térmica rigorosa. Bloqueie as aprovações automáticas operando nessas vias antes de checar a integridade de todas as diretrizes interligadas vizinhas. As camadas interligam barramentos determinísticos profundos e topologias matriciais estritas:

* `pkg/util/application.go` (Distribuidor central de roteamento matricial)
* `pkg/app/runner.go` & `pkg/app/bot_runtime.go` (Núcleo de limite basal paralelo e integrador lógico do fluxo contínuo de rotina em barramento dinâmico livre)
* `pkg/files/types.go` & `ui/src/api/config_types.ts` (Estrutura paralela cruzada de integridade e esquemas estritamente imutáveis)
* `pkg/control/server.go` (Receptor central associativo limite estrito `HTTP`)
* `pkg/discord/commands/legacycore/registry.go` & `pkg/discord/commands/handler.go` (Rotas radiais das diretivas paralelas subjacentes e tráfego interligado direto do domínio)
* `pkg/discord/logging/monitoring.go` & `pkg/messages/message_events.go` (Injetores massivos das pontas de comunicação fluindo e varrendo dados paralelos dinamicamente)
* `pkg/storage/qotd.go`, `pkg/qotd/service.go`, & `pkg/qotd/runtime.go` (Retentores sistêmicos de memórias paralelas integrando camadas isoladas)
* `pkg/task/router.go` (Comandante basal orquestrador para as integrações paralelas ativas sob demanda local externa via restrições temporais assíncronas)
* `pkg/stats/service.go` (Orquestração analítica estrita encapsulada)
* `ui/src/pages/ModerationPage.tsx` & `ui/src/pages/CorePage.tsx` (Topologia primária radial retendo alta captação simultânea front-end ativa limite e estados isolados mutáveis retidos em barramentos integrados restritos)