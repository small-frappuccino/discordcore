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

// Register implements the ArikawaRegisterer interface.
func (s *SpyRouter) Register(cmd ArikawaCommand) {
	data := api.CreateCommandData{
		Name:        cmd.Name(),
		Description: cmd.Description(),
		Options:     cmd.Options(),
	}
	s.RegisterArikawa(data)
}

// RegisterComponent implements the ArikawaRegisterer interface.
func (s *SpyRouter) RegisterComponent(customIDPrefix string, handler ComponentHandler) {
	// No-op for command assertions
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
