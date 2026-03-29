package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

var runDiscordCore = discordcoreapp.RunWithOptions

const (
	commandName         = "discordcore"
	runtimeAppName      = "alicebot"
	productionTokenEnv  = "ALICE_BOT_PRODUCTION_TOKEN"
	developmentTokenEnv = "ALICE_BOT_DEVELOPMENT_TOKEN"
	yuzuhaTokenEnv      = "YUZUHA_BOT_TOKEN"
	localControlAddr    = "127.0.0.1:8443"
	localControlOrigin  = "https://alice.localhost:8443"
)

// main is the entry point of the Discord bot runtime hosted in discordcore.
func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		slog.Error("Fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	fs.SetOutput(output)
	testMode := fs.Bool("testing", false, "Run discordcore in test mode (uses ALICE_BOT_DEVELOPMENT_TOKEN instead of ALICE_BOT_PRODUCTION_TOKEN)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	primaryTokenEnv := selectTokenEnv(*testMode)
	discordcoreapp.SetAppVersion(discordcoreapp.Version)

	if err := runDiscordCore(runtimeAppName, primaryTokenEnv, defaultRunOptions(primaryTokenEnv)); err != nil {
		return normalizeRunError(err, *testMode)
	}
	return nil
}

func defaultRunOptions(primaryTokenEnv string) discordcoreapp.RunOptions {
	loadKnownBotTokenEnvs(primaryTokenEnv)

	return discordcoreapp.RunOptions{
		Control: discordcoreapp.ControlOptions{
			BindAddr:     localControlAddr,
			PublicOrigin: localControlOrigin,
			LocalHTTPS: discordcoreapp.ControlLocalHTTPSOptions{
				Enabled:   true,
				AutoTrust: true,
			},
		},
		BotCatalog: configuredBotCatalog(primaryTokenEnv),
	}
}

func configuredBotCatalog(primaryTokenEnv string) []discordcoreapp.BotInstanceDefinition {
	return []discordcoreapp.BotInstanceDefinition{
		{
			ID:       "alice",
			TokenEnv: primaryTokenEnv,
			Optional: true,
		},
		{
			ID:       "yuzuha",
			TokenEnv: yuzuhaTokenEnv,
			Optional: true,
		},
	}
}

func selectTokenEnv(testMode bool) string {
	if testMode {
		return developmentTokenEnv
	}
	if env := availableTokenEnv(); env != "" {
		return env
	}

	// Attempt to load the fallback .env so we can see tokens defined there.
	_, _ = util.LoadEnvWithLocalBinFallback(productionTokenEnv)
	if env := availableTokenEnv(); env != "" {
		return env
	}

	return productionTokenEnv
}

func availableTokenEnv() string {
	if util.EnvString(productionTokenEnv, "") != "" {
		return productionTokenEnv
	}
	if util.EnvString(developmentTokenEnv, "") != "" {
		return developmentTokenEnv
	}
	return ""
}

func loadKnownBotTokenEnvs(primaryTokenEnv string) {
	_, _ = util.LoadEnvWithLocalBinFallback(primaryTokenEnv)
	_, _ = util.LoadEnvWithLocalBinFallback(yuzuhaTokenEnv)
}

func normalizeRunError(err error, testMode bool) error {
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no bot instances have a configured token") {
		return err
	}

	checkedEnvs := []string{developmentTokenEnv, yuzuhaTokenEnv}
	if !testMode {
		checkedEnvs = append([]string{productionTokenEnv}, checkedEnvs...)
	}
	return fmt.Errorf("no bot token configured; checked %s: %w", strings.Join(checkedEnvs, ", "), err)
}
