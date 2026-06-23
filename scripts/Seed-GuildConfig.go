//go:build ignore

package main

import (
	"database/sql"
	"flag"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func main() {
	dbURL := flag.String("db", "", "Postgres database URL to seed")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("Must provide -db flag with database URL")
	}

	db, err := sql.Open("pgx", *dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store := files.NewPostgresConfigStore(db, "primary")
	mgr := files.NewConfigManagerWithStore(store)
	if err := mgr.LoadConfig(); err != nil {
		log.Fatal("LoadConfig:", err)
	}

	cfg := files.GuildConfig{
		GuildID:       "1375650791251120179",
		BotInstanceID: "main",
		DomainBotInstanceIDs: map[string]string{
			"qotd": "companion",
		},
		Channels: files.ChannelsConfig{
			RoleUpdate:    "1397390179492171806",
			AvatarLogging: "1397390179492171806",
			MemberJoin:    "1413465672708657216",
			MemberLeave:   "1413465672708657216",
			MessageEdit:   "1396973382372687983",
			MessageDelete: "1396973382372687983",
			AutomodAction: "1396973495715631287",
		},
		Features: files.FeatureToggles{
			Logging: files.FeatureLoggingToggles{
				RoleUpdate:    boolPtr(true),
				AvatarLogging: boolPtr(true),
				MemberJoin:    boolPtr(true),
				MemberLeave:   boolPtr(true),
				MessageEdit:   boolPtr(true),
				MessageDelete: boolPtr(true),
				AutomodAction: boolPtr(true),
			},
			Services: files.FeatureServiceToggles{
				Automod: boolPtr(false),
			},
		},
		RuntimeConfig: files.RuntimeConfig{
			DisableMessageLogs:   false,
			MessageCacheTTLHours: 24,
			MessageCacheCleanup:  true,
			DisableAutomodLogs:   false,
		},
		Stats: files.StatsConfig{
			Enabled:            true,
			UpdateIntervalMins: 30, // Default recommended
			Channels: []files.StatsChannelConfig{
				{
					ChannelID:    "1379653952639074374",
					Label:        "Total Proxies",
					NameTemplate: "{label}: {count}",
					MemberType:   "all",
				},
				{
					ChannelID:    "1395994541324238848",
					Label:        "Bunny Boosters",
					NameTemplate: "{label}: {count}",
					MemberType:   "humans",
					RoleID:       "1375851519819124907",
				},
				{
					ChannelID:    "1379653956376199228",
					Label:        "Proxies",
					NameTemplate: "{label}: {count}",
					MemberType:   "humans",
				},
				{
					ChannelID:    "1379653960272449688",
					Label:        "Bangboos",
					NameTemplate: "{label}: {count}",
					MemberType:   "bots",
				},
			},
		},
		QOTD: files.QOTDConfig{
			ActiveDeckID: "default",
			Decks: []files.QOTDDeckConfig{
				{
					ID:        "default",
					Name:      "Default",
					Enabled:   true,
					ChannelID: "1376373100622512148",
				},
			},
			Schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(12),
				MinuteUTC: intPtr(43),
			},
		},
	}

	if err := mgr.SaveGuildConfig(cfg); err != nil {
		log.Fatal("SaveGuildConfig:", err)
	}

	log.Println("Successfully seeded guild config into database!")
}
