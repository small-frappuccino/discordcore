**Alvo de Execução:** Auditoria topológica e refatoração estrutural de estado restrito.
**Escopo Topológico:** Matriz integral do repositório, impondo exclusão isolada absoluta ao diretório `pkg/app`.

Inicie a varredura analítica-mecânica sobre as ramificações de código adjacentes. Mapeie e corrija anomalias baseando-se nos seguintes invariantes de hardware e *runtime*:

1. **Fronteiras de Alocação:** Execute *escape analysis* nos caminhos críticos (*hot-paths*). Retenha variáveis efêmeras estritamente na *stack*. Estabilize a complexidade espacial em $\mathcal{O}(1)$ suprimindo alocações dinâmicas não mapeadas para o *heap*.
2. **Determinismo de Concorrência:** Isole e erradique *goroutines* desprovidas de supervisão de ciclo de vida atrelada a `context.Context` e `golang.org/x/sync/errgroup`. Desfaça estrangulamentos de *write-starvation* causados por saturação em blocos `sync.RWMutex`.
3. **Isolamento de Estado em Malhas de Teste:** Imponha paralelismo vetorial via `t.Parallel()`. Oblitere dependências estocásticas, estado global mutável e sincronização latente fundamentada em *polling* temporal (`time.Sleep`).

**Diretriz de Verificação de Fator de Causa (`Continuous Validation Loop`):**
Instancie a malha de teste do compilador engatando a execução ininterrupta do comando `go test -race -v ./...` sobre os pacotes refatorados. O ciclo de tokenização e consolidação da rotina só deve ser interrompido e declarado concluído quando o motor de execução confirmar matematicamente:
* Zero falhas de *data race* ou concorrência desordenada.
* Zero *goroutines* órfãs retidas em memória térmica.
* Zero fugas de alocação ineficiente da *stack* para as fronteiras do *GC*.

Emita o diagnóstico estrutural do gargalo em uma única linha descritiva e entregue imediatamente o bloco de refatoração final, completo e mecanicamente executável.