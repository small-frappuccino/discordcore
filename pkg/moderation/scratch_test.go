package moderation

import (
	"context"
	"fmt"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

func TestScratch(t *testing.T) {
	registry := &dummyRegistry{
		bot: core.BotInstance{
			ApplicationID: "123456789",
			GuildID:       "987654321",
			Token:         "dummy",
		},
	}
	router := NewRouter(registry)

	payload := []byte(`{
		"guild_id": "987654321",
		"application_id": "123456789",
		"data": {
			"name": "ban",
			"options": [
				{"name": "target_id", "value": "111222333"},
				{"name": "delete_days", "value": 7},
				{"name": "reason", "value": "spam"}
			]
		}
	}`)

	name := extractCommandNameFast(payload)
	fmt.Printf("EXTRACTED NAME: '%s'\n", name)

	_, err := router.ParseInteraction(context.Background(), payload)
	fmt.Printf("PARSE INTERACTION ERR: %v\n", err)
}
