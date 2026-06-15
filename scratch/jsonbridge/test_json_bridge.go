package main

import (
	"encoding/json"
	"fmt"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordgo"
)

func main() {
	opts := []discord.CommandOption{
		&discord.StringOption{
			OptionName:  "test",
			Description: "desc",
			Required:    true,
		},
	}
	b, err := json.Marshal(opts)
	if err != nil {
		panic(err)
	}

	var dgoOpts []*discordgo.ApplicationCommandOption
	if err := json.Unmarshal(b, &dgoOpts); err != nil {
		panic(err)
	}

	fmt.Printf("Parsed: %+v\n", dgoOpts[0])
}
