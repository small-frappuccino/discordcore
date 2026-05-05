package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
)

func setTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestSelectTokenEnv(t *testing.T) {
	setTempHome(t)
	t.Setenv(productionTokenEnv, "")
	t.Setenv(developmentTokenEnv, "")

	if got := selectTokenEnv(false); got != productionTokenEnv {
		t.Fatalf("expected production token env, got %s", got)
	}
	if got := selectTokenEnv(true); got != developmentTokenEnv {
		t.Fatalf("expected development token env, got %s", got)
	}
}

func TestSelectTokenEnvPrefersDevelopmentEnvWhenProductionIsMissing(t *testing.T) {
	setTempHome(t)
	t.Setenv(productionTokenEnv, "")
	t.Setenv(developmentTokenEnv, "dev-token")

	if got := selectTokenEnv(false); got != developmentTokenEnv {
		t.Fatalf("expected development token env when only dev token is set, got %s", got)
	}
}

func TestRunUsesPreferredAliceEnvByDefault(t *testing.T) {
	setTempHome(t)

	called := struct {
		name string
		env  string
		opts discordcoreapp.RunOptions
	}{}

	orig := runDiscordCore
	runDiscordCore = func(name, tokenEnv string, opts discordcoreapp.RunOptions) error {
		called.name = name
		called.env = tokenEnv
		called.opts = opts
		return nil
	}
	t.Cleanup(func() { runDiscordCore = orig })

	if err := run(nil, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called.name != runtimeAppName || called.env != productionTokenEnv {
		t.Fatalf("unexpected call args: %+v", called)
	}
	if !called.opts.Control.LocalHTTPS.Enabled || !called.opts.Control.LocalHTTPS.AutoTrust {
		t.Fatalf("expected local https run options, got %+v", called.opts)
	}
	if called.opts.Control.BindAddr != localControlAddr || called.opts.Control.PublicOrigin != localControlOrigin {
		t.Fatalf("unexpected control options: %+v", called.opts.Control)
	}
	if called.opts.DefaultBotInstanceID != "" {
		t.Fatalf("expected runtime to resolve default bot automatically, got %+v", called.opts)
	}
}

func TestRunUsesDevelopmentWhenTestingFlagIsPresent(t *testing.T) {
	setTempHome(t)

	called := struct {
		env  string
		opts discordcoreapp.RunOptions
	}{}

	orig := runDiscordCore
	runDiscordCore = func(name, tokenEnv string, opts discordcoreapp.RunOptions) error {
		called.env = tokenEnv
		called.opts = opts
		return nil
	}
	t.Cleanup(func() { runDiscordCore = orig })

	if err := run([]string{"-testing"}, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called.env != developmentTokenEnv {
		t.Fatalf("expected development token env, got %s", called.env)
	}
}

func TestRunPropagatesGenericErrors(t *testing.T) {
	setTempHome(t)

	orig := runDiscordCore
	runDiscordCore = func(name, tokenEnv string, opts discordcoreapp.RunOptions) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { runDiscordCore = orig })

	if err := run(nil, io.Discard); err == nil {
		t.Fatal("expected error from runDiscordCore")
	}
}

func TestRunExplainsMissingKnownTokens(t *testing.T) {
	setTempHome(t)

	orig := runDiscordCore
	runDiscordCore = func(name, tokenEnv string, opts discordcoreapp.RunOptions) error {
		return discordcoreapp.ErrNoBotTokensConfigured
	}
	t.Cleanup(func() { runDiscordCore = orig })

	err := run(nil, io.Discard)
	if err == nil {
		t.Fatal("expected missing token error")
	}
	for _, tokenEnv := range []string{productionTokenEnv, developmentTokenEnv, companionTokenEnv} {
		if !strings.Contains(err.Error(), tokenEnv) {
			t.Fatalf("expected error to mention %s, got %v", tokenEnv, err)
		}
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	setTempHome(t)

	if err := run([]string{"-unknown"}, io.Discard); err == nil {
		t.Fatal("expected flag parse error for unknown flag")
	}
}

func TestConfiguredBotCatalogAllowsAliceAndCompanionFallback(t *testing.T) {
	catalog := configuredBotCatalog(productionTokenEnv)
	if len(catalog) != 2 {
		t.Fatalf("expected alice+companion catalog, got %+v", catalog)
	}
	if catalog[0].ID != "alice" || catalog[0].TokenEnv != productionTokenEnv || !catalog[0].Optional {
		t.Fatalf("unexpected alice catalog entry: %+v", catalog[0])
	}
	if catalog[1].ID != "companion" || catalog[1].TokenEnv != companionTokenEnv || !catalog[1].Optional {
		t.Fatalf("unexpected companion catalog entry: %+v", catalog[1])
	}
}

func TestDefaultRunOptionsConfiguresMultiBotCatalogWithoutFixedDefault(t *testing.T) {
	setTempHome(t)

	opts := defaultRunOptions(productionTokenEnv)
	if opts.DefaultBotInstanceID != "" {
		t.Fatalf("expected empty default bot instance, got %+v", opts)
	}
	if len(opts.BotCatalog) != 2 {
		t.Fatalf("expected two bot catalog entries, got %+v", opts.BotCatalog)
	}
	if !opts.BotCatalog[0].Optional || !opts.BotCatalog[1].Optional {
		t.Fatalf("expected both bots to remain optional, got %+v", opts.BotCatalog)
	}
}
