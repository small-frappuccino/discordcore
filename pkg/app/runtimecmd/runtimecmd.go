package runtimecmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
	discordcommands "github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// QOTDDevelopmentTokenEnv defines qotddevelopment token env.
// QOTDProductionTokenEnv defines qotdproduction token env.
// MainDevelopmentTokenEnv defines main development token env.
// MainProductionTokenEnv defines main production token env.
// QOTDRuntimeAppName defines qotdruntime app name.
// MainRuntimeAppName defines main runtime app name.
// QOTDBotInstanceID defines qotdbot instance id.
// MainBotInstanceID defines main bot instance id.
const (
	MainBotInstanceID       = "main"
	QOTDBotInstanceID       = "companion"
	MainRuntimeAppName      = "discordmain"
	QOTDRuntimeAppName      = "discordqotd"
	MainProductionTokenEnv  = "ALICE_BOT_PRODUCTION_TOKEN"
	MainDevelopmentTokenEnv = "ALICE_BOT_DEVELOPMENT_TOKEN"
	QOTDProductionTokenEnv  = "QOTD_BOT_PRODUCTION_TOKEN"
	QOTDDevelopmentTokenEnv = "QOTD_BOT_DEVELOPMENT_TOKEN"
)

// Spec describes a runtime entrypoint command: its name, the token environment
// variables for production and development (test) modes, and a factory that
// builds the RunOptions once the primary token env has been selected.
type Spec struct {
	CommandName         string
	RuntimeAppName      string
	ProductionTokenEnv  string
	DevelopmentTokenEnv string
	KnownTokenEnvs      []string
	BuildRunOptions     func(primaryTokenEnv string) discordcoreapp.RunOptions
}

// Runner starts a runtime app with the resolved name, token env, and options.
// It is the injection seam that lets Run be tested without a live runtime.
type Runner func(appName, tokenEnv string, opts discordcoreapp.RunOptions) error

// Run runs.
func Run(args []string, output io.Writer, spec Spec, runner Runner) error {
	fs := flag.NewFlagSet(spec.CommandName, flag.ContinueOnError)
	fs.SetOutput(output)
	testMode := fs.Bool(
		"testing",
		false,
		fmt.Sprintf(
			"Run %s in test mode (uses %s instead of %s)",
			spec.CommandName,
			spec.DevelopmentTokenEnv,
			spec.ProductionTokenEnv,
		),
	)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("Run: %w", err)
	}

	primaryTokenEnv := SelectTokenEnv(*testMode, spec)
	loadKnownTokenEnvs(spec.KnownTokenEnvs)
	discordcoreapp.SetAppVersion(discordcoreapp.Version)

	if err := runner(spec.RuntimeAppName, primaryTokenEnv, spec.BuildRunOptions(primaryTokenEnv)); err != nil {
		return normalizeRunError(err, *testMode, spec)
	}
	return nil
}

// SelectTokenEnv selects token env.
func SelectTokenEnv(testMode bool, spec Spec) string {
	if testMode {
		return spec.DevelopmentTokenEnv
	}
	if env := availableTokenEnv(spec); env != "" {
		return env
	}

	if _, err := files.LoadEnvWithLocalBinFallback(spec.ProductionTokenEnv); err != nil {
		log.ApplicationLogger().Debug("Failed to load environment from local bin fallback", "err", err)
	}
	if env := availableTokenEnv(spec); env != "" {
		return env
	}

	return spec.ProductionTokenEnv
}

// MainSpec mains spec.
func MainSpec(commandName string) Spec {
	return Spec{
		CommandName:         commandName,
		RuntimeAppName:      MainRuntimeAppName,
		ProductionTokenEnv:  MainProductionTokenEnv,
		DevelopmentTokenEnv: MainDevelopmentTokenEnv,
		KnownTokenEnvs: []string{
			MainProductionTokenEnv,
			MainDevelopmentTokenEnv,
		},
		BuildRunOptions: buildMainRunOptions,
	}
}

// QOTDSpec qotdspecs.
func QOTDSpec(commandName string) Spec {
	return Spec{
		CommandName:         commandName,
		RuntimeAppName:      QOTDRuntimeAppName,
		ProductionTokenEnv:  QOTDProductionTokenEnv,
		DevelopmentTokenEnv: QOTDDevelopmentTokenEnv,
		KnownTokenEnvs: []string{
			QOTDProductionTokenEnv,
			QOTDDevelopmentTokenEnv,
		},
		BuildRunOptions: buildQOTDRunOptions,
	}
}

func buildMainRunOptions(primaryTokenEnv string) discordcoreapp.RunOptions {
	return discordcoreapp.RunOptions{
		Profile: discordcoreapp.RunProfileDiscordMain,
		Control: discordcoreapp.ControlOptions{
			LocalHTTPS: discordcoreapp.ControlLocalHTTPSOptions{
				Enabled:   true,
				AutoTrust: true,
			},
		},
		BotCatalog: []discordcoreapp.BotInstanceDefinition{{
			ID:       MainBotInstanceID,
			TokenEnv: primaryTokenEnv,
			Optional: true,
		}},
		DefaultOwnerBotInstanceID: MainBotInstanceID,
		KnownBotInstanceIDs:       []string{QOTDBotInstanceID},
		SupportedDomains:          []string{"default"},
		CommandCatalogRegistrars: []discordcommands.CommandCatalogRegistrar{
			discordcommands.BaseCommandCatalogRegistrar(),
			discordcommands.AdminCommandCatalogRegistrar(),
		},
	}
}

func buildQOTDRunOptions(primaryTokenEnv string) discordcoreapp.RunOptions {
	return discordcoreapp.RunOptions{
		Profile: discordcoreapp.RunProfileDiscordQOTD,
		// The persisted QOTD owner still uses the legacy companion instance id.
		BotCatalog: []discordcoreapp.BotInstanceDefinition{{
			ID:       QOTDBotInstanceID,
			TokenEnv: primaryTokenEnv,
			Optional: true,
		}},
		DefaultOwnerBotInstanceID: MainBotInstanceID,
		KnownBotInstanceIDs:       []string{MainBotInstanceID},
		SupportedDomains:          []string{files.BotDomainQOTD},
		CommandCatalogRegistrars: []discordcommands.CommandCatalogRegistrar{
			discordcommands.QOTDCommandCatalogRegistrar(),
		},
		DisableControl: true,
	}
}

func availableTokenEnv(spec Spec) string {
	if files.EnvString(spec.ProductionTokenEnv, "") != "" {
		return spec.ProductionTokenEnv
	}
	if files.EnvString(spec.DevelopmentTokenEnv, "") != "" {
		return spec.DevelopmentTokenEnv
	}
	return ""
}

func loadKnownTokenEnvs(tokenEnvs []string) {
	for _, tokenEnv := range tokenEnvs {
		if strings.TrimSpace(tokenEnv) == "" {
			continue
		}
		if _, err := files.LoadEnvWithLocalBinFallback(tokenEnv); err != nil {
			log.ApplicationLogger().Debug("Failed to load environment from local bin fallback", "env", tokenEnv, "err", err)
		}
	}
}

func normalizeRunError(err error, testMode bool, spec Spec) error {
	if err == nil {
		return nil
	}
	if !errors.Is(err, discordcoreapp.ErrNoBotTokensConfigured) {
		return err
	}

	checkedEnvs := append([]string(nil), spec.KnownTokenEnvs...)
	if testMode && len(checkedEnvs) > 0 && checkedEnvs[0] == spec.ProductionTokenEnv {
		checkedEnvs = checkedEnvs[1:]
	}
	return fmt.Errorf("no bot token configured; checked %s: %w", strings.Join(checkedEnvs, ", "), err)
}
