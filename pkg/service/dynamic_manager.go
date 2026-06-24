package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type ServiceWrapper interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Done() <-chan struct{}
}

type serviceState struct {
	wrapper    ServiceWrapper
	cancelFunc context.CancelFunc
	runDone    chan struct{}
}

type Manager struct {
	mu       sync.Mutex
	services map[string]*serviceState
}

func NewManager() *Manager {
	return &Manager{
		services: make(map[string]*serviceState),
	}
}

func (m *Manager) RegisterAndStart(name string, svc ServiceWrapper) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[name]; exists {
		return errors.New("service already registered: " + name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	state := &serviceState{
		wrapper:    svc,
		cancelFunc: cancel,
		runDone:    make(chan struct{}),
	}
	m.services[name] = state

	go func() {
		defer close(state.runDone)
		if err := svc.Start(ctx); err != nil {
			fmt.Printf("fatal: service %s stopped: %v\n", name, err)
		}
	}()

	return nil
}

func (m *Manager) StopAndRemove(ctx context.Context, name string) error {
	m.mu.Lock()
	state, exists := m.services[name]
	if !exists {
		m.mu.Unlock()
		return errors.New("service not found: " + name)
	}
	delete(m.services, name)
	m.mu.Unlock()

	state.cancelFunc()

	if err := state.wrapper.Stop(ctx); err != nil {
		return fmt.Errorf("stop signal failed for %s: %w", name, err)
	}

	select {
	case <-state.runDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("drain timeout exceeded for %s: %w", name, ctx.Err())
	}
}

func (m *Manager) ForceRemove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, exists := m.services[name]; exists {
		delete(m.services, name)
		state.cancelFunc()
	}
}
