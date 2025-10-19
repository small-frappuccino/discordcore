package main

import (
	"log/slog"
	"os"

	"github.com/small-frappuccino/discordcore/pkg/app"
)

// main is the entry point of the Discord bot.
func main() {
	if err := app.Run("discordcore", "ALICE_BOT_DEVELOPMENT_TOKEN"); err != nil {
		slog.Error("Fatal", "err", err)
		os.Exit(1)
	}
}
