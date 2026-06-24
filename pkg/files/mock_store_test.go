package files

import "sync"

type mockConfigStore struct {
	mu  sync.Mutex
	cfg *BotConfig
}

func (m *mockConfigStore) Load() (*BotConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg == nil {
		return &BotConfig{Guilds: []GuildConfig{}}, nil
	}
	return m.cfg, nil
}

func (m *mockConfigStore) Save(cfg *BotConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	return nil
}

func (m *mockConfigStore) Exists() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg != nil, nil
}

func (m *mockConfigStore) Describe() string {
	return "mock://config"
}
