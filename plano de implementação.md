# discordcore — Plano de Implementação (safra 2026-07)

Escopo: materializar o repositório que o `CLAUDE.md` governa, com execução agentic via Fable 5. Fora de escopo: engenharia de prompt do próprio `CLAUDE.md` (fase posterior).

Princípio ordenador: **gates antes de features**. O manifesto é inteiro condicionado a oráculos (§2 Directive-Gate); portanto o substrato de verificação (contador de alocações, criterion, Miri/sanitizers, llvm-lines) é o M0, não um apêndice. Cada milestone só fecha quando seu gate roda em CI — isso é também o que torna a execução agentic auditável: Fable 5 produz diffs, o CI produz o veredito.

---

## 1. Árvore de arquivos (workspace Cargo)

```
discordcore/
├── CLAUDE.md
├── Cargo.toml                      # [workspace]; lints compartilhados; deny(unsafe_op_in_unsafe_fn) via [workspace.lints]
├── rust-toolchain.toml             # stable 1.96 pinned; nightly pinned à parte p/ Miri/sanitizers (override em CI)
├── .cargo/config.toml
├── deny.toml                       # cargo-deny: licenças, advisories, duplicatas
├── .github/workflows/
│   ├── ci.yml                      # fmt, clippy, check, test (stable)
│   ├── soundness.yml               # jobs SEPARADOS: miri | asan | msan (-Zbuild-std)  — §6
│   └── perf.yml                    # criterion threshold, alloc-count regression, llvm-lines diff — §7
├── xtask/
│   └── src/main.rs                 # cargo xtask: bench-diff, alloc-regression-check, llvm-lines-report
├── docs/
│   └── decisions/                  # decision log exigido pelo Directive-Gate (derivações de thresholds)
│       ├── 0001-channel-depth.md
│       ├── 0002-governor-vs-twilight-limiter.md
│       └── ...
├── crates/
│   ├── core-types/                 # zero deps async. Feature (#[repr(u8)] fieldless), GuildId/BotId newtypes, erros de domínio
│   │   └── src/{lib.rs, feature.rs, ids.rs, error.rs}
│   ├── testkit/                    # oráculo §7: #[global_allocator] contador (cfg(test)), harness assert_allocs!(0, { .. })
│   │   └── src/lib.rs
│   ├── telemetry/                  # fixture §11 promovida a crate real: TelemetrySink, serialize_into, put/put_u64
│   │   ├── src/lib.rs
│   │   └── benches/serialize.rs    # criterion + black_box; gate alloc==0
│   ├── secrets/                    # zeroize + secrecy; tipos BotToken etc. — §8
│   │   └── src/lib.rs
│   ├── routing/                    # (GuildID, Feature) → BotInstance; single-writer; invariantes §8 (unicidade, cap ≤5)
│   │   └── src/{lib.rs, table.rs, invariants.rs}
│   │       └── tests/properties.rs # proptest sobre as duas cardinalidades + rejeição de reassinatura
│   ├── persistence/                # sqlx/PostgreSQL; hydration §5/§8.1; estado latente vs. efetivo
│   │   ├── src/{lib.rs, schema.rs, hydrate.rs}
│   │   └── migrations/
│   ├── ingest/                     # Gateway-RX. twilight-gateway + twilight-model; simd-json; intents mínimos
│   │   ├── src/{lib.rs, shard.rs, intents.rs, dispatch.rs, buffers.rs}
│   │   └── benches/dispatch.rs     # hot path sob falsificador: allocs==0
│   ├── egress/                     # REST-TX. Wrapper sobre twilight-http
│   │   └── src/{lib.rs, client.rs, global_limit.rs, breaker.rs, perimeter.rs, buckets.rs}
│   ├── features/                   # 18 linhas do registro §10
│   │   └── src/{lib.rs,
│   │        logging/{lifecycle.rs, mutation.rs, messages.rs, automod.rs},
│   │        moderation/{actions.rs, suite.rs, tickets.rs, modmail.rs},
│   │        identity/{stats_channels.rs, roles.rs, embeds.rs},
│   │        community/{qotd.rs, partners.rs}}
│   └── app/                        # §12: Slint + root JoinSet + ponte IPC
│       ├── src/{main.rs, orchestrator.rs, bridge.rs, state.rs}
│       ├── ui/{main.slint, bots.slint, features.slint}
│       └── build.rs                # slint-build
└── README.md
```

Racional dos cortes de crate: `core-types` e `testkit` sem dependência de tokio/twilight mantêm o oráculo de alocação e os invariantes de domínio compiláveis e testáveis em segundos — o loop interno do agente. `ingest`/`egress` separados espelham o regime dual do §2 (fronteira RX/TX vira fronteira de crate, e o gate de alocação se aplica a `ingest` inteiro, não a símbolos escolhidos a dedo). `features` depende de ambos mas não o inverso.

---

## 2. Milestones e gates

**M0 — Substrato de verificação.** Workspace, toolchain pin, os três workflows, `xtask`, `testkit` (contador `#[global_allocator]`), e `telemetry` compilando a fixture do §11 com o falsificador provando `allocs == 0` em `serialize_into`. Entregável colateral: substituir a *user skill* `heap-escape-falsifier` (Go, declarada void pelo §7) por uma skill Rust `alloc-falsifier` que envelopa o harness do `testkit` — a pilha oráculo vira ferramenta reusável fora do repo.
*Gate:* CI verde nos três workflows; perf.yml falhando artificialmente ao injetar 1 alocação (teste do teste).

**M1 — Núcleo de domínio.** `core-types`, `routing` com property tests das cardinalidades, `secrets`, `persistence` com schema + hydration (§8.1: latente/efetivo, sem toggles booleanos). Nenhum I/O Discord ainda.
*Gate:* proptest das invariantes; Miri sobre `routing`.

**M2 — Gateway-RX.** Integração `twilight-gateway` (lifecycle de shard delegado, §9.1), intents mínimos com `MESSAGE_CONTENT` isolado, backend SIMD-JSON, decode em buffers de stack (§3), dispatch por `(GuildID, Feature)` via tabela hidratada, topologia `JoinSet` + árvore de `CancellationToken`.
*Verificar na entrada do milestone:* nome/estado da feature SIMD na versão corrente de `twilight-gateway` (deriva de API; não assumir).
*Gate:* `benches/dispatch.rs` com `allocs == 0`; Miri sobre os blocos `unsafe` de buffer.

**M3 — REST-TX.** Tarefa 0 é a auditoria mandada pelo §9.2: comportamento do `twilight-http-ratelimiting` corrente quanto ao global 50 req/s — o resultado decide se `governor` entra ou é latência redundante, registrado em `docs/decisions/0002`. Depois: breaker `{401,403,429}` com janela deslizante e exclusão de `X-RateLimit-Scope: shared`, limitador perimetral 2/10 min por canal (hard-coded, exceção sancionada), clustering de rotas major.
*Gate:* testes de simulação de bucket (mock de headers); decision log preenchido.

**M4 — Features (§10), em 4 lotes na ordem do registro:** logging (RX) → moderação (TX) → identidade/stats (depende do perimetral do M3) → comunidade. Cada lote fecha com verificação da tabela de não-regressão — nenhuma linha silenciosamente ausente.
*Gate:* lote RX passa no falsificador; lotes TX passam na simulação de bucket.

**M5 — App Slint (§12).** Ordem de init (tokio primeiro), ponte IPC sobre `mpsc` bounded, toggles travados até `BotState::Authenticated`, remoção de bot/rotação de token disparando o token da subárvore.
*Gate:* teste de teardown determinístico (nenhuma task sobrevive ao frame do root `JoinSet`).

**M6 — Hardening.** MSan com `-Zbuild-std` (job agendado, não por-PR — custo de build alto), `perf stat`/`cachegrind` nos símbolos quentes, simulação de contenção a 10⁴ ops/s, re-derivação do SLA de contenção (os 2137 TPS legados são void), fechamento do decision log.

**Caminho crítico:** M0 → M1 → {M2 ∥ M3} → M4 → M5; M6 sobrepõe a partir de M4.

---

## 3. Estimativa (planejamento, não compromisso — variância alta em execução agentic)

| Grandeza | Faixa |
| --- | --- |
| Rust (src + testes + benches) | ~14–20 kLOC |
| Slint + build glue | ~1–2 kLOC |
| CI/xtask/migrations | ~0,5–1 kLOC |
| Sessões Fable 5 | M0: 2–3 · M1: 3–4 · M2: 4–6 · M3: 3–4 · M4: 6–9 · M5: 3–5 · M6: 2–4 → **~23–35** |
| Revisão humana | concentrada em: blocos `unsafe` de `ingest`, decisão do M3.0, ponte Slint↔tokio; o resto o CI adjudica |
| Calendário | 4–7 semanas com cadência de revisão em tempo parcial |

Onde a variância mora: M5 é o trecho menos amigável ao agente (padrões Slint↔async têm menos massa de treino e menos oráculo mecânico — a revisão manual pesa aqui); M4 é largo mas raso (18 linhas homogêneas, o lote 1 calibra os demais); M2/M3 dependem das auditorias de versão do twilight marcadas acima — lacunas deliberadas, a resolver na entrada de cada milestone, não interpoladas agora.