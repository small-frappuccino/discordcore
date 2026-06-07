package runtimecmd

import (
	"io"
	"testing"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
)

func setTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestRunUsesMainProfileOptions(t *testing.T) {
	setTempHome(t)

	called := struct {
		name string
		opts discordcoreapp.RunOptions
	}{}

	err := Run(nil, io.Discard, MainSpec("discordmain"), func(name string, opts discordcoreapp.RunOptions) error {
		called.name = name
		called.opts = opts
		return nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called.name != MainRuntimeAppName {
		t.Fatalf("unexpected call args: %+v", called)
	}
	if called.opts.DefaultOwnerBotInstanceID != MainBotInstanceID {
		t.Fatalf("expected main as the default owner, got %+v", called.opts)
	}
	if called.opts.Profile != discordcoreapp.RunProfileDiscordMain {
		t.Fatalf("expected main run profile, got %+v", called.opts)
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

	if len(called.opts.CommandCatalogRegistrars) != 2 {
		t.Fatalf("unexpected main command registrars: %+v", called.opts.CommandCatalogRegistrars)
	}
	if called.opts.CommandCatalogRegistrars[0].RequiredCapabilities.Admin {
		t.Fatalf("expected base registrar first, got %+v", called.opts.CommandCatalogRegistrars)
	}
	if !called.opts.CommandCatalogRegistrars[1].RequiredCapabilities.Admin {
		t.Fatalf("expected admin registrar second, got %+v", called.opts.CommandCatalogRegistrars)
	}
	if !called.opts.Control.LocalHTTPS.Enabled || !called.opts.Control.LocalHTTPS.AutoTrust {
		t.Fatalf("expected local https control options, got %+v", called.opts.Control)
	}
	if called.opts.Control.BindAddr != "" || called.opts.Control.PublicOrigin != "" {
		t.Fatalf("expected main profile to derive control defaults later, got %+v", called.opts.Control)
	}
}

func TestRunUsesQOTDProfileOptions(t *testing.T) {
	setTempHome(t)

	called := struct {
		name string
		opts discordcoreapp.RunOptions
	}{}

	err := Run(nil, io.Discard, QOTDSpec("discordqotd"), func(name string, opts discordcoreapp.RunOptions) error {
		called.name = name
		called.opts = opts
		return nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called.name != QOTDRuntimeAppName {
		t.Fatalf("unexpected call args: %+v", called)
	}
	if called.opts.DefaultOwnerBotInstanceID != MainBotInstanceID {
		t.Fatalf("expected qotd runtime to keep main as the global default owner, got %+v", called.opts)
	}
	if called.opts.Profile != discordcoreapp.RunProfileDiscordQOTD {
		t.Fatalf("expected qotd run profile, got %+v", called.opts)
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

	if len(called.opts.CommandCatalogRegistrars) != 1 {
		t.Fatalf("unexpected qotd command registrars: %+v", called.opts.CommandCatalogRegistrars)
	}
}
