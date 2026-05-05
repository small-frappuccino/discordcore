package runtimecmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

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

type Spec struct {
	CommandName         string
	RuntimeAppName      string
	ProductionTokenEnv  string
	DevelopmentTokenEnv string
	KnownTokenEnvs      []string
	BuildRunOptions     func(primaryTokenEnv string) discordcoreapp.RunOptions
}

type Runner func(appName, tokenEnv string, opts discordcoreapp.RunOptions) error

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
		return err
	}

	primaryTokenEnv := SelectTokenEnv(*testMode, spec)
	loadKnownTokenEnvs(spec.KnownTokenEnvs)
	discordcoreapp.SetAppVersion(discordcoreapp.Version)

	if err := runner(spec.RuntimeAppName, primaryTokenEnv, spec.BuildRunOptions(primaryTokenEnv)); err != nil {
		return normalizeRunError(err, *testMode, spec)
	}
	return nil
}

func SelectTokenEnv(testMode bool, spec Spec) string {
	if testMode {
		return spec.DevelopmentTokenEnv
	}
	if env := availableTokenEnv(spec); env != "" {
		return env
	}

	_, _ = util.LoadEnvWithLocalBinFallback(spec.ProductionTokenEnv)
	if env := availableTokenEnv(spec); env != "" {
		return env
	}

	return spec.ProductionTokenEnv
}

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
		DisableControl:            true,
	}
}

func availableTokenEnv(spec Spec) string {
	if util.EnvString(spec.ProductionTokenEnv, "") != "" {
		return spec.ProductionTokenEnv
	}
	if util.EnvString(spec.DevelopmentTokenEnv, "") != "" {
		return spec.DevelopmentTokenEnv
	}
	return ""
}

func loadKnownTokenEnvs(tokenEnvs []string) {
	for _, tokenEnv := range tokenEnvs {
		if strings.TrimSpace(tokenEnv) == "" {
			continue
		}
		_, _ = util.LoadEnvWithLocalBinFallback(tokenEnv)
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
