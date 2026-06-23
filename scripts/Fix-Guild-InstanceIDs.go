//go:build ignore

package main

import (
	"database/sql"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func main() {
	_ = godotenv.Load(`D:\Users\alice\.local\bin\.env`)

	dbURL := files.EnvString("ALICE_DATABASE_URL", "")
	if dbURL == "" {
		log.Fatal("ALICE_DATABASE_URL not set in env")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := files.NewPostgresConfigStore(db, "primary")
	mgr := files.NewConfigManagerWithStore(store)
	if err := mgr.LoadConfig(); err != nil {
		log.Fatal("LoadConfig:", err)
	}

	cfg := mgr.GuildConfig("1375650791251120179")
	if cfg == nil {
		log.Fatal("Guild 1375650791251120179 not found in config")
	}

	// Assign the guild to the main instance and the qotd domain to the qotd instance
	cfg.BotInstanceID = "discordmain"
	if cfg.DomainBotInstanceIDs == nil {
		cfg.DomainBotInstanceIDs = make(map[string]string)
	}
	cfg.DomainBotInstanceIDs["qotd"] = "discordqotd"

	if err := mgr.SaveGuildConfig(*cfg); err != nil {
		log.Fatal("SaveGuildConfig:", err)
	}

	log.Println("Successfully assigned BotInstanceIDs to the guild!")
}
