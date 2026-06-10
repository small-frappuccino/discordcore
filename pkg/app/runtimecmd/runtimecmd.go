package runtimecmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joho/godotenv"
	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
	discordcommands "github.com/small-frappuccino/discordcore/pkg/discord/commands"
)

// MainRuntimeAppName defines main runtime app name.
// DefaultBotInstanceID defines the default bot instance id.
const (
	DefaultBotInstanceID = "main"
	MainRuntimeAppName   = "discordmain"
)

// Spec describes a runtime entrypoint command: its name, and a factory that
// builds the RunOptions.
type Spec struct {
	CommandName     string
	RuntimeAppName  string
	BuildRunOptions func() discordcoreapp.RunOptions
}

// Runner starts a runtime app with the resolved name and options.
// It is the injection seam that lets Run be tested without a live runtime.
type Runner func(appName string, opts discordcoreapp.RunOptions) error

// Run runs.
func Run(args []string, output io.Writer, spec Spec, runner Runner) error {
	fs := flag.NewFlagSet(spec.CommandName, flag.ContinueOnError)
	fs.SetOutput(output)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("Run: %w", err)
	}

	// Tenta carregar as variáveis de ambiente localmente caso o arquivo .env exista
	home, err := os.UserHomeDir()
	if err == nil {
		if len(home) >= 2 && home[1] == ':' {
			home = "D:" + home[2:]
		}
		home = strings.Replace(home, "enzok", "smallfrappuccino", 1)
		_ = godotenv.Load(home + `\.local\bin\.env`)
	}

	discordcoreapp.SetAppVersion(discordcoreapp.Version)

	if err := runner(spec.RuntimeAppName, spec.BuildRunOptions()); err != nil {
		return err
	}
	return nil
}

// MainSpec mains spec.
func MainSpec(commandName string) Spec {
	return Spec{
		CommandName:     commandName,
		RuntimeAppName:  MainRuntimeAppName,
		BuildRunOptions: buildMainRunOptions,
	}
}

func buildMainRunOptions() discordcoreapp.RunOptions {
	return discordcoreapp.RunOptions{
		Profile: discordcoreapp.RunProfileDiscordMain,
		Control: discordcoreapp.ControlOptions{
			LocalHTTPS: discordcoreapp.ControlLocalHTTPSOptions{
				Enabled:   true,
				AutoTrust: true,
			},
		},
		DefaultOwnerBotInstanceID: DefaultBotInstanceID,

		CommandCatalogRegistrars: []discordcommands.CommandCatalogRegistrar{
			discordcommands.BaseCommandCatalogRegistrar(),
			discordcommands.AdminCommandCatalogRegistrar(),
			discordcommands.QOTDCommandCatalogRegistrar(),
		},
	}
}
