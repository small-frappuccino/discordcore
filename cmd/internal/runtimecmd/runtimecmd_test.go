package runtimecmd

import (
	"errors"
	"io"
	"strings"
	"testing"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func setTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestSelectTokenEnvPrefersConfiguredDevelopmentTokenWhenProductionIsMissing(t *testing.T) {
	setTempHome(t)
	t.Setenv(MainProductionTokenEnv, "")
	t.Setenv(MainDevelopmentTokenEnv, "dev-token")

	if got := SelectTokenEnv(false, MainSpec("discordmain")); got != MainDevelopmentTokenEnv {
		t.Fatalf("expected development env when production token is missing, got %s", got)
	}
	if got := SelectTokenEnv(true, MainSpec("discordmain")); got != MainDevelopmentTokenEnv {
		t.Fatalf("expected explicit testing mode to use development env, got %s", got)
	}
}

func TestRunUsesMainProfileOptions(t *testing.T) {
	setTempHome(t)

	called := struct {
		name string
		env  string
		opts discordcoreapp.RunOptions
	}{}

	err := Run(nil, io.Discard, MainSpec("discordmain"), func(name, tokenEnv string, opts discordcoreapp.RunOptions) error {
		called.name = name
		called.env = tokenEnv
		called.opts = opts
		return nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called.name != MainRuntimeAppName || called.env != MainProductionTokenEnv {
		t.Fatalf("unexpected call args: %+v", called)
	}
	if called.opts.DefaultOwnerBotInstanceID != MainBotInstanceID {
		t.Fatalf("expected main as the default owner, got %+v", called.opts)
	}
	if called.opts.DisableControl {
		t.Fatalf("expected control plane to stay enabled for main runtime, got %+v", called.opts)
	}
	if len(called.opts.BotCatalog) != 1 || called.opts.BotCatalog[0].ID != MainBotInstanceID || !called.opts.BotCatalog[0].Optional {
		t.Fatalf("unexpected main bot catalog: %+v", called.opts.BotCatalog)
	}
	if len(called.opts.KnownBotInstanceIDs) != 1 || called.opts.KnownBotInstanceIDs[0] != QOTDBotInstanceID {
		t.Fatalf("unexpected known bot ids: %+v", called.opts.KnownBotInstanceIDs)
	}
	if len(called.opts.SupportedDomains) != 1 || called.opts.SupportedDomains[0] != "default" {
		t.Fatalf("unexpected supported domains: %+v", called.opts.SupportedDomains)
	}
	if !called.opts.Control.LocalHTTPS.Enabled || !called.opts.Control.LocalHTTPS.AutoTrust {
		t.Fatalf("expected local https control options, got %+v", called.opts.Control)
	}
	if called.opts.Control.BindAddr != LocalControlAddr || called.opts.Control.PublicOrigin != LocalControlOrigin {
		t.Fatalf("unexpected control options: %+v", called.opts.Control)
	}
}

func TestRunUsesQOTDProfileOptions(t *testing.T) {
	setTempHome(t)

	called := struct {
		name string
		env  string
		opts discordcoreapp.RunOptions
	}{}

	err := Run(nil, io.Discard, QOTDSpec("discordqotd"), func(name, tokenEnv string, opts discordcoreapp.RunOptions) error {
		called.name = name
		called.env = tokenEnv
		called.opts = opts
		return nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called.name != QOTDRuntimeAppName || called.env != QOTDProductionTokenEnv {
		t.Fatalf("unexpected call args: %+v", called)
	}
	if called.opts.DefaultOwnerBotInstanceID != MainBotInstanceID {
		t.Fatalf("expected qotd runtime to keep main as the global default owner, got %+v", called.opts)
	}
	if !called.opts.DisableControl {
		t.Fatalf("expected qotd runtime to keep control disabled, got %+v", called.opts)
	}
	if len(called.opts.BotCatalog) != 1 || called.opts.BotCatalog[0].ID != QOTDBotInstanceID || !called.opts.BotCatalog[0].Optional {
		t.Fatalf("unexpected qotd bot catalog: %+v", called.opts.BotCatalog)
	}
	if len(called.opts.KnownBotInstanceIDs) != 1 || called.opts.KnownBotInstanceIDs[0] != MainBotInstanceID {
		t.Fatalf("unexpected known bot ids: %+v", called.opts.KnownBotInstanceIDs)
	}
	if len(called.opts.SupportedDomains) != 1 || called.opts.SupportedDomains[0] != files.BotDomainQOTD {
		t.Fatalf("unexpected supported domains: %+v", called.opts.SupportedDomains)
	}
}

func TestRunExplainsMissingTokensWithProfileSpecificEnvNames(t *testing.T) {
	setTempHome(t)

	err := Run(nil, io.Discard, QOTDSpec("discordqotd"), func(name, tokenEnv string, opts discordcoreapp.RunOptions) error {
		return discordcoreapp.ErrNoBotTokensConfigured
	})
	if err == nil {
		t.Fatal("expected missing token error")
	}
	for _, tokenEnv := range []string{QOTDProductionTokenEnv, QOTDDevelopmentTokenEnv} {
		if !strings.Contains(err.Error(), tokenEnv) {
			t.Fatalf("expected error to mention %s, got %v", tokenEnv, err)
		}
	}
	if !errors.Is(err, discordcoreapp.ErrNoBotTokensConfigured) {
		t.Fatalf("expected wrapped ErrNoBotTokensConfigured, got %v", err)
	}
}
