Aqui está a implementação exata de como estruturar o `SpyRouter` e o sistema de `Capabilities` usando os padrões idiomáticos do Go (como *bitmasking* e concorrência segura) para suportar o ecossistema do Arikawa.

### 1. Implementação do `SpyRouter`

O `SpyRouter` serve como um *test double* em memória. Em vez de abrir conexões WebSocket ou fazer chamadas HTTP reais para a API do Discord, ele intercepta as estruturas `api.CreateCommandData` enviadas pelo registrador e as armazena de forma segura em um mapa para posterior asserção nos testes.

```go
package commands

import (
	"sync"

	"github.com/diamondburned/arikawa/v3/api"
)

// SpyRouter intercepta os registros de comandos para asserção em testes
type SpyRouter struct {
	mu       sync.RWMutex
	commands map[string]api.CreateCommandData
}

// NewSpyRouter inicializa o spy com o mapa interno alocado
func NewSpyRouter() *SpyRouter {
	return &SpyRouter{
		commands: make(map[string]api.CreateCommandData),
	}
}

// RegisterArikawa simula o roteamento real, guardando o payload em memória
func (s *SpyRouter) RegisterArikawa(data api.CreateCommandData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands[data.Name] = data
}

// HasCommand verifica se um comando específico foi registrado pelo catálogo
func (s *SpyRouter) HasCommand(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.commands[name]
	return exists
}

// GetCommandData retorna o payload completo do Arikawa para validação de sub-options ou descrições
func (s *SpyRouter) GetCommandData(name string) api.CreateCommandData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.commands[name]
}

// GetRegisteredArikawaCommands retorna a lista de todos os comandos capturados
func (s *SpyRouter) GetRegisteredArikawaCommands() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.commands))
	for name := range s.commands {
		names = append(names, name)
	}
	return names
}

```

### 2. Implementação do Sistema de `Capabilities`

Em arquiteturas de bots de alta performance, usar um *bitmask* baseado em `uint64` com atribuição via `iota` é a abordagem padrão do mercado (inclusive é o padrão que o próprio ecossistema do Discord usa para permissões). Isso permite combinar múltiplas permissões ou requisitos exigidos por um catálogo em um único inteiro, reduzindo alocações de memória e simplificando asserções binárias.

```go
package app

import (
	"strings"
)

// CommandCatalogCapabilities define um bitmask para permissões e pré-requisitos do catálogo
type CommandCatalogCapabilities uint64

const (
	// CapNone representa a ausência de requisitos especiais
	CapNone CommandCatalogCapabilities = 0

	// Definição das flags usando bitwise shifting
	CapBanMembers CommandCatalogCapabilities = 1 << iota
	CapKickMembers
	CapManageMessages
	CapStatsRead
	CapQOTDAdmin
)

// Has valida se o bitmask atual contém a capacidade informada
func (c CommandCatalogCapabilities) Has(target CommandCatalogCapabilities) bool {
	return (c & target) == target
}

// String provê uma representação legível por humanos para logs e relatórios de falhas de testes
func (c CommandCatalogCapabilities) String() string {
	if c == CapNone {
		return "CapNone"
	}

	var parts []string
	
	// Mapeamento explícito para fins de stringify
	flags := map[CommandCatalogCapabilities]string{
		CapBanMembers:     "CapBanMembers",
		CapKickMembers:    "CapKickMembers",
		CapManageMessages: "CapManageMessages",
		CapStatsRead:      "CapStatsRead",
		CapQOTDAdmin:      "CapQOTDAdmin",
	}

	for flag, name := range flags {
		if c.Has(flag) {
			parts = append(parts, name)
		}
	}

	if len(parts) == 0 {
		return "CapUnknown"
	}

	return strings.Join(parts, "|")
}

```

### Como esses dois componentes se consolidam no Teste:

Usando as estruturas acima, a asserção no seu arquivo de teste (`catalog_registrars_test.go`) fica limpa, rápida e livre de efeitos colaterais de rede:

```go
func TestModerationCommandCatalogRegistrar_Contracts(t *testing.T) {
	t.Parallel()

	handler := &app.CommandHandler{}
	spyRouter := commands.NewSpyRouter()

	registrar := app.ModerationCommandCatalogRegistrar(handler)

	// 1. Validando o Bitmask de Capabilities de forma direta
	expectedCaps := app.CapBanMembers | app.CapKickMembers
	if !registrar.RequiredCapabilities.Has(expectedCaps) {
		t.Fatalf("Expected capabilities %s, got %s", expectedCaps, registrar.RequiredCapabilities)
	}

	// 2. Validando a mutação de estado através do Spy
	registrar.RegisterArikawa(handler, spyRouter)

	if !spyRouter.HasCommand("ban") {
		t.Error("Expected command 'ban' to be registered in Arikawa router")
	}
}

```