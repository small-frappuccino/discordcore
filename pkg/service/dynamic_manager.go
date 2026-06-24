package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
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
	eg       *errgroup.Group
	egCtx    context.Context
}

func NewManager(ctx context.Context) *Manager {
	eg, egCtx := errgroup.WithContext(ctx)
	return &Manager{
		services: make(map[string]*serviceState),
		eg:       eg,
		egCtx:    egCtx,
	}
}

func (m *Manager) RegisterAndStart(name string, svc ServiceWrapper) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[name]; exists {
		return errors.New("service already registered: " + name)
	}

	ctx, cancel := context.WithCancel(m.egCtx)
	state := &serviceState{
		wrapper:    svc,
		cancelFunc: cancel,
		runDone:    make(chan struct{}),
	}
	m.services[name] = state

	m.eg.Go(func() error {
		defer close(state.runDone)
		if err := svc.Start(ctx); err != nil {
			fmt.Printf("fatal: service %s stopped: %v\n", name, err)
			return fmt.Errorf("service %s failed: %w", name, err)
		}
		return nil
	})

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

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	var names []string
	for name := range m.services {
		names = append(names, name)
	}
	m.mu.Unlock()

	var errs []error
	for _, name := range names {
		if err := m.StopAndRemove(ctx, name); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to stop some services: %v", errs)
	}
	return nil
}

// Wait blocks until the underlying errgroup completes.
func (m *Manager) Wait() error {
	return m.eg.Wait()
}
