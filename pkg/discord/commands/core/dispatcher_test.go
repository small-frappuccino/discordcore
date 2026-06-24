package core

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
)

func FuzzDispatcher_DispatchRaw(f *testing.F) {
	f.Add([]byte(`{"type":2,"data":{"id":"123","name":"test","type":1}}`))
	f.Add([]byte(`{"type":1}`))
	f.Add([]byte(`{"invalid":}`))
	f.Add([]byte(`{}`))

	registry := NewCommandRegistry()
	_ = registry.Register(&Command{
		Name: "test",
		Handler: func(ctx *InteractionContext) error {
			return nil
		},
	})
	registry.Seal()

	client := api.NewClient("Bot token")
	dispatcher := NewDispatcher(client, registry)

	f.Fuzz(func(t *testing.T, payload []byte) {
		_ = dispatcher.DispatchRaw(payload)
	})
}

func TestDispatcher_ValidCommand(t *testing.T) {
	t.Parallel()
	registry := NewCommandRegistry()
	called := false
	_ = registry.Register(&Command{
		Name: "test",
		Handler: func(ctx *InteractionContext) error {
			called = true
			return nil
		},
	})
	registry.Seal()

	client := api.NewClient("Bot token")
	dispatcher := NewDispatcher(client, registry)

	payload := []byte(`{"type":2,"data":{"id":"123","name":"test","type":1}}`)
	err := dispatcher.DispatchRaw(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}
