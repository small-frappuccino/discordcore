package main

import (
	"log"
	"os"
	"strings"
)

func main() {
	b, err := os.ReadFile("pkg/stats/service.go")
	if err != nil {
		log.Fatal(err)
	}
	s := string(b)

	s = strings.Replace(s, "mu      sync.RWMutex\n\tguilds  map[string]*statsGuildState\n\tlastRun map[string]time.Time", "cancelMu sync.Mutex\n\tguilds   sync.Map\n\tlastRun  sync.Map", 1)

	s = strings.Replace(s, "\t\tguilds:               make(map[string]*statsGuildState),\n\t\tlastRun:              make(map[string]time.Time),\n", "", 1)

	s = strings.Replace(s, "type statsGuildState struct {\n", "type statsGuildState struct {\n\tmu              sync.Mutex\n", 1)

	s = strings.Replace(s, "func (s *StatsService) Start(ctx context.Context) error {\n\ts.mu.Lock()\n\tdefer s.mu.Unlock()", "func (s *StatsService) Start(ctx context.Context) error {\n\ts.cancelMu.Lock()\n\tdefer s.cancelMu.Unlock()", 1)

	s = strings.Replace(s, "func (s *StatsService) Stop(ctx context.Context) error {\n\ts.mu.Lock()\n\tif s.cancel == nil {\n\t\ts.mu.Unlock()", "func (s *StatsService) Stop(ctx context.Context) error {\n\ts.cancelMu.Lock()\n\tif s.cancel == nil {\n\t\ts.cancelMu.Unlock()", 1)
	s = strings.Replace(s, "\ts.cancel = nil\n\ts.mu.Unlock()", "\ts.cancel = nil\n\ts.cancelMu.Unlock()", 1)

	pruneOld := `func (s *StatsService) pruneStatsGuildState(activeGuilds map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for guildID := range s.lastRun {
		if _, ok := activeGuilds[guildID]; ok {
			continue
		}
		delete(s.lastRun, guildID)
	}
	for guildID := range s.guilds {
		if _, ok := activeGuilds[guildID]; ok {
			continue
		}
		delete(s.guilds, guildID)
	}
}`
	pruneNew := `func (s *StatsService) pruneStatsGuildState(activeGuilds map[string]struct{}) {
	s.lastRun.Range(func(key, value any) bool {
		guildID := key.(string)
		if _, ok := activeGuilds[guildID]; !ok {
			s.lastRun.Delete(guildID)
		}
		return true
	})
	s.guilds.Range(func(key, value any) bool {
		guildID := key.(string)
		if _, ok := activeGuilds[guildID]; !ok {
			s.guilds.Delete(guildID)
		}
		return true
	})
}`
	s = strings.Replace(s, pruneOld, pruneNew, 1)

	pattern1 := "s.mu.Lock()\n\tdefer s.mu.Unlock()\n\n\tstate := s.ensureStatsGuildStateLocked(guildID)"
	repl1 := "state := s.getOrInitStatsGuildState(guildID)\n\tstate.mu.Lock()\n\tdefer state.mu.Unlock()"
	s = strings.ReplaceAll(s, pattern1, repl1)

	pattern2 := "s.mu.Lock()\n\tdefer s.mu.Unlock()\n\tstate := s.ensureStatsGuildStateLocked(guildID)"
	repl2 := "state := s.getOrInitStatsGuildState(guildID)\n\tstate.mu.Lock()\n\tdefer state.mu.Unlock()"
	s = strings.ReplaceAll(s, pattern2, repl2)

	snapOld := `func (s *StatsService) statsSnapshot(guildID string) (statsGuildSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.guilds == nil {
		return statsGuildSnapshot{}, false
	}
	state := s.guilds[guildID]
	if state == nil || !state.initialized {
		return statsGuildSnapshot{}, false
	}`
	snapNew := `func (s *StatsService) statsSnapshot(guildID string) (statsGuildSnapshot, bool) {
	v, ok := s.guilds.Load(guildID)
	if !ok {
		return statsGuildSnapshot{}, false
	}
	state := v.(*statsGuildState)

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.initialized {
		return statsGuildSnapshot{}, false
	}`
	s = strings.Replace(s, snapOld, snapNew, 1)

	lastRunOld := `	s.mu.RLock()
	last, ok := s.lastRun[guildID]
	s.mu.RUnlock()`
	lastRunNew := `	var last time.Time
	if v, ok := s.lastRun.Load(guildID); ok {
		last = v.(time.Time)
	}`
	s = strings.ReplaceAll(s, lastRunOld, lastRunNew)

	lastRunUpdateOld := `	s.mu.Lock()
	if s.lastRun == nil {
		s.lastRun = make(map[string]time.Time)
	}
	s.lastRun[guildID] = time.Now()
	s.mu.Unlock()`
	lastRunUpdateNew := `	s.lastRun.Store(guildID, time.Now())`
	s = strings.ReplaceAll(s, lastRunUpdateOld, lastRunUpdateNew)

	ensureOld := `func (s *StatsService) ensureStatsGuildStateLocked(guildID string) *statsGuildState {
	if s.guilds == nil {
		s.guilds = make(map[string]*statsGuildState)
	}
	state := s.guilds[guildID]
	if state == nil {
		state = &statsGuildState{
			members:    make(map[string]statsMemberSnapshot),
			roleTotals: make(map[string]statsCounterBucket),
			published:  make(map[string]statsPublishedChannel),
		}
		s.guilds[guildID] = state
	}
	return state
}`
	ensureNew := `func (s *StatsService) getOrInitStatsGuildState(guildID string) *statsGuildState {
	if s == nil {
		return nil
	}
	v, ok := s.guilds.Load(guildID)
	if ok {
		return v.(*statsGuildState)
	}
	newState := &statsGuildState{
		members:    make(map[string]statsMemberSnapshot),
		roleTotals: make(map[string]statsCounterBucket),
		published:  make(map[string]statsPublishedChannel),
	}
	v, _ = s.guilds.LoadOrStore(guildID, newState)
	return v.(*statsGuildState)
}`
	s = strings.Replace(s, ensureOld, ensureNew, 1)

	err = os.WriteFile("pkg/stats/service.go", []byte(s), 0644)
	if err != nil {
		log.Fatal(err)
	}
}
