package main

import (
	"io"
	"log/slog"
	"os"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
	runtimecmd "github.com/small-frappuccino/discordcore/pkg/app/runtimecmd"
)

var runDiscordMainDev = discordcoreapp.RunWithOptions

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		slog.Error("Fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	return runtimecmd.Run(append([]string{"-testing"}, args...), output, runtimecmd.MainSpec("discordmaindev"), runDiscordMainDev)
}
