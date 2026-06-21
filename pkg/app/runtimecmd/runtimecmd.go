package runtimecmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/small-frappuccino/discordcore/pkg/app"
	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
)

// MainRuntimeAppName is the canonical identifier for the primary Discord bot process.
const (
	MainRuntimeAppName = "discordmain"
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

// Run parses CLI flags, attempts to load a local .env file from the system PATH,
// and invokes the provided runner with the resolved execution options.
func Run(args []string, output io.Writer, spec Spec, runner Runner) error {
	fs := flag.NewFlagSet(spec.CommandName, flag.ContinueOnError)
	fs.SetOutput(output)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("Run: %w", err)
	}

	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		pathEnv = os.Getenv("Path")
	}
	// Scan dynamic environment path boundaries to isolate local developer configurations before execution.
	for _, dir := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			if err := godotenv.Load(envPath); err != nil {
				return fmt.Errorf("failed to load .env from PATH (%s): %w", envPath, err)
			}
			break
		}
	}

	discordcoreapp.SetAppVersion(discordcoreapp.Version)

	if err := runner(spec.RuntimeAppName, spec.BuildRunOptions()); err != nil {
		return err
	}
	return nil
}

// MainSpec constructs the execution specification for the primary bot process.
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
		CommandCatalogRegistrars: app.DefaultCommandCatalogRegistrars(),
	}
}
