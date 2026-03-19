package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type orderedOptionsCommand struct{}

func (orderedOptionsCommand) Name() string        { return "ordered" }
func (orderedOptionsCommand) Description() string { return "ordered command" }
func (orderedOptionsCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "optional_opt",
			Description: "optional",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "required_opt",
			Description: "required",
			Required:    true,
		},
	}
}
func (orderedOptionsCommand) Handle(*Context) error { return nil }
func (orderedOptionsCommand) RequiresGuild() bool   { return false }
func (orderedOptionsCommand) RequiresPermissions() bool {
	return false
}

func TestCompareCommands_NormalizesRequiredOptionOrderRecursively(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "cfg",
		Description: "config",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "set value",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "optional",
						Description: "optional value",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "required",
						Description: "required value",
						Required:    true,
					},
				},
			},
		},
	}

	b := &discordgo.ApplicationCommand{
		Name:        "cfg",
		Description: "config",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "set value",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "required",
						Description: "required value",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "optional",
						Description: "optional value",
						Required:    false,
					},
				},
			},
		},
	}

	if !CompareCommands(a, b) {
		t.Fatalf("expected commands to compare equal after required-first normalization")
	}
}

func TestNormalizeCommandOptions_DoesNotMutateOriginalOrder(t *testing.T) {
	original := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "optional_opt",
			Description: "optional",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "required_opt",
			Description: "required",
			Required:    true,
		},
	}

	normalized := normalizeCommandOptions(original)
	if len(normalized) != 2 {
		t.Fatalf("expected 2 normalized options, got %d", len(normalized))
	}
	if normalized[0].Name != "required_opt" || normalized[1].Name != "optional_opt" {
		t.Fatalf("unexpected normalized order: %s, %s", normalized[0].Name, normalized[1].Name)
	}
	if original[0].Name != "optional_opt" || original[1].Name != "required_opt" {
		t.Fatalf("expected original order to remain unchanged, got %s, %s", original[0].Name, original[1].Name)
	}
}

func TestCommandManagerSetupCommands_NormalizesOptionOrderBeforeSync(t *testing.T) {
	var posted discordgo.ApplicationCommand
	session := newCommandManagerSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode posted command: %v", err)
			}
			if posted.ID == "" {
				posted.ID = "created-id"
			}
			_ = json.NewEncoder(w).Encode(&posted)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}
	})

	cfgMgr := files.NewMemoryConfigManager()
	cm := NewCommandManager(session, cfgMgr)
	cm.GetRouter().RegisterCommand(orderedOptionsCommand{})

	if err := cm.SetupCommands(); err != nil {
		t.Fatalf("setup commands: %v", err)
	}

	if len(posted.Options) != 2 {
		t.Fatalf("expected 2 posted command options, got %d", len(posted.Options))
	}
	if posted.Options[0].Name != "required_opt" || !posted.Options[0].Required {
		t.Fatalf("expected required option first, got %+v", posted.Options[0])
	}
	if posted.Options[1].Name != "optional_opt" || posted.Options[1].Required {
		t.Fatalf("expected optional option second, got %+v", posted.Options[1])
	}
}

func validateRequiredBeforeOptional(
	options []*discordgo.ApplicationCommandOption,
	path string,
) error {
	seenOptional := false

	for _, opt := range options {
		if opt == nil {
			continue
		}

		isContainer :=
			opt.Type == discordgo.ApplicationCommandOptionSubCommand ||
				opt.Type == discordgo.ApplicationCommandOptionSubCommandGroup
		if !isContainer {
			if !opt.Required {
				seenOptional = true
			} else if seenOptional {
				return fmt.Errorf("%s: required option %q appears after optional options", path, opt.Name)
			}
		}

		if len(opt.Options) > 0 {
			nextPath := path + "/" + opt.Name
			if err := validateRequiredBeforeOptional(opt.Options, nextPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func TestValidateRequiredBeforeOptional_SubCommandGroupNested(t *testing.T) {
	invalid := []*discordgo.ApplicationCommandOption{
		{
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Name: "group",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Name: "set",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:     discordgo.ApplicationCommandOptionString,
							Name:     "optional_first",
							Required: false,
						},
						{
							Type:     discordgo.ApplicationCommandOptionString,
							Name:     "required_second",
							Required: true,
						},
					},
				},
			},
		},
	}

	if err := validateRequiredBeforeOptional(invalid, "/root"); err == nil {
		t.Fatal("expected validation error for nested SubCommandGroup with optional-before-required")
	}

	valid := []*discordgo.ApplicationCommandOption{
		{
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Name: "group",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Name: "set",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:     discordgo.ApplicationCommandOptionString,
							Name:     "required_first",
							Required: true,
						},
						{
							Type:     discordgo.ApplicationCommandOptionString,
							Name:     "optional_second",
							Required: false,
						},
					},
				},
			},
		},
	}

	if err := validateRequiredBeforeOptional(valid, "/root"); err != nil {
		t.Fatalf("expected nested SubCommandGroup to be valid, got error: %v", err)
	}
}
