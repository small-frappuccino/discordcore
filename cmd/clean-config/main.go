package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
)

func main() {
	dbc := persistence.Config{
		Driver:      "postgres",
		DatabaseURL: "postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable",
	}

	db, err := persistence.Open(context.Background(), dbc)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	configStore := files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey, slog.Default())
	configManager := files.NewConfigManagerWithStore(configStore, slog.Default())
	if err := configManager.LoadConfig(); err != nil {
		log.Fatalf("load config: %v", err)
	}

	cfg := configManager.Config()
	found := false
	for i, guild := range cfg.Guilds {
		if guild.GuildID == "1512582051172319453" {
			if _, exists := guild.BotInstanceTokens["sandrone"]; exists {
				fmt.Println("Removing sandrone from old guild 1512582051172319453")
				delete(guild.BotInstanceTokens, "sandrone")
				cfg.Guilds[i] = guild
				found = true
			}
			break
		}
	}

	if found {
		configManager.ApplyConfig(cfg)
		if err := configManager.SaveConfig(); err != nil {
			log.Fatalf("save config: %v", err)
		}
		fmt.Println("Successfully cleaned up old config!")
	} else {
		fmt.Println("No changes needed.")
	}
}
