package main

import (
	"io"
	"log/slog"
	"os"

	runtimecmd "github.com/small-frappuccino/discordcore/cmd/internal/runtimecmd"
	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
)

var runDiscordQOTD = discordcoreapp.RunWithOptions

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		slog.Error("Fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	return runtimecmd.Run(args, output, runtimecmd.QOTDSpec("discordqotd"), runDiscordQOTD)
}
