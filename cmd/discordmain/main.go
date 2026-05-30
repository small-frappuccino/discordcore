package main

import (
	"io"
	"log/slog"
	"os"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
	runtimecmd "github.com/small-frappuccino/discordcore/pkg/app/runtimecmd"
)

var runDiscordMain = discordcoreapp.RunWithOptions

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		slog.Error("Fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	return runtimecmd.Run(args, output, runtimecmd.MainSpec("discordmain"), runDiscordMain)
}
