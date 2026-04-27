package core

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestSlashAckPolicyDefersExactlyOnce(t *testing.T) {
	session, rec := newTestSession(t)
	router := NewCommandRouter(session, files.NewMemoryConfigManager())

	handlerCalls := 0
	router.RegisterSlashCommand(testCommand{
		name:      "deferred",
		ackPolicy: InteractionAckPolicy{Mode: InteractionAckModeDefer, Ephemeral: true},
		handler: func(ctx *Context) error {
			handlerCalls++
			responses := rec.all()
			if len(responses) != 1 {
				t.Fatalf("expected deferred slash ack before handler, got %d responses", len(responses))
			}
			if responses[0].Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
				t.Fatalf("unexpected slash ack type: %v", responses[0].Type)
			}
			return nil
		},
	})

	router.HandleInteraction(session, buildInteraction("deferred", "guild", "user"))

	responses := rec.all()
	if len(responses) != 1 {
		t.Fatalf("expected exactly one slash ack response, got %d", len(responses))
	}
	if responses[0].Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("unexpected slash ack type: %v", responses[0].Type)
	}
	if responses[0].Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected deferred slash ack to preserve ephemeral flag")
	}
	if handlerCalls != 1 {
		t.Fatalf("expected slash handler to run once, got %d", handlerCalls)
	}
}

func TestComponentAckPolicyDefersUpdateBeforeHandler(t *testing.T) {
	session, rec := newTestSession(t)
	router := NewCommandRouter(session, files.NewMemoryConfigManager())

	handlerCalls := 0
	router.RegisterInteractionRoute(InteractionRouteBinding{
		Path:      "runtimecfg:action:main",
		AckPolicy: InteractionAckPolicy{Mode: InteractionAckModeDefer},
		Component: ComponentHandlerFunc(func(ctx *Context) error {
			handlerCalls++
			responses := rec.all()
			if len(responses) != 1 {
				t.Fatalf("expected deferred component ack before handler, got %d responses", len(responses))
			}
			if responses[0].Type != discordgo.InteractionResponseDeferredMessageUpdate {
				t.Fatalf("unexpected component ack type: %v", responses[0].Type)
			}
			return nil
		}),
	})

	router.HandleInteraction(session, buildComponentInteraction("runtimecfg:action:main|state", "guild", "user"))

	responses := rec.all()
	if len(responses) != 1 {
		t.Fatalf("expected exactly one component ack response, got %d", len(responses))
	}
	if responses[0].Type != discordgo.InteractionResponseDeferredMessageUpdate {
		t.Fatalf("unexpected component ack type: %v", responses[0].Type)
	}
	if handlerCalls != 1 {
		t.Fatalf("expected component handler to run once, got %d", handlerCalls)
	}
}

func TestModalAckPolicyDefersUpdateBeforeHandler(t *testing.T) {
	session, rec := newTestSession(t)
	router := NewCommandRouter(session, files.NewMemoryConfigManager())

	handlerCalls := 0
	router.RegisterInteractionRoute(InteractionRouteBinding{
		Path:      "runtimecfg:modal:edit",
		AckPolicy: InteractionAckPolicy{Mode: InteractionAckModeDefer},
		Modal: ModalHandlerFunc(func(ctx *Context) error {
			handlerCalls++
			responses := rec.all()
			if len(responses) != 1 {
				t.Fatalf("expected deferred modal ack before handler, got %d responses", len(responses))
			}
			if responses[0].Type != discordgo.InteractionResponseDeferredMessageUpdate {
				t.Fatalf("unexpected modal ack type: %v", responses[0].Type)
			}
			return nil
		}),
	})

	router.HandleInteraction(session, buildModalInteraction("runtimecfg:modal:edit|state", "guild", "user"))

	responses := rec.all()
	if len(responses) != 1 {
		t.Fatalf("expected exactly one modal ack response, got %d", len(responses))
	}
	if responses[0].Type != discordgo.InteractionResponseDeferredMessageUpdate {
		t.Fatalf("unexpected modal ack type: %v", responses[0].Type)
	}
	if handlerCalls != 1 {
		t.Fatalf("expected modal handler to run once, got %d", handlerCalls)
	}
}

func TestComponentModalPathDoesNotPreAckBeforeOpeningModal(t *testing.T) {
	session, rec := newTestSession(t)
	router := NewCommandRouter(session, files.NewMemoryConfigManager())

	handlerCalls := 0
	router.RegisterInteractionRoute(InteractionRouteBinding{
		Path: "runtimecfg:action:edit",
		Component: ComponentHandlerFunc(func(ctx *Context) error {
			handlerCalls++
			if len(rec.all()) != 0 {
				t.Fatalf("expected no pre-ack before modal open, got %d responses", len(rec.all()))
			}
			return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{Title: "Edit runtime", CustomID: "runtimecfg:modal:edit|state"},
			})
		}),
	})

	router.HandleInteraction(session, buildComponentInteraction("runtimecfg:action:edit|state", "guild", "user"))

	responses := rec.all()
	if len(responses) != 1 {
		t.Fatalf("expected exactly one modal-open response, got %d", len(responses))
	}
	if responses[0].Type != discordgo.InteractionResponseModal {
		t.Fatalf("expected modal response without pre-ack, got %v", responses[0].Type)
	}
	if handlerCalls != 1 {
		t.Fatalf("expected modal-opening component handler to run once, got %d", handlerCalls)
	}
}