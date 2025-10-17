package main

import (
	"os"

	"github.com/small-frappuccino/discordcore/pkg/app"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// main is the entry point of the Discord bot.
func main() {
	if err := app.Run("discordcore", "ALICE_BOT_DEVELOPMENT_TOKEN"); err != nil {
		log.Error().Errorf("Fatal: %v", err)
		os.Exit(1)
	}
}
