package core

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestAutocomplete(t *testing.T) {
	choices := []*discordgo.ApplicationCommandOptionChoice{
		CreateChoice("Apple", "apple"),
		CreateChoice("Banana", "banana"),
		CreateChoice("Cherry", "cherry"),
	}
	filtered := FilterChoices(choices, "app")
	if len(filtered) != 1 || filtered[0].Name != "Apple" {
		t.Fatal("FilterChoices app")
	}
	filtered = FilterChoices(choices, "AN")
	if len(filtered) != 1 || filtered[0].Name != "Banana" {
		t.Fatal("FilterChoices AN")
	}
	filtered = FilterChoices(choices, "")
	if len(filtered) != 3 {
		t.Fatal("FilterChoices empty")
	}
	strChoices := CreateChoicesFromStrings([]string{"Dog", "Cat"})
	if len(strChoices) != 2 || strChoices[0].Name != "Dog" || strChoices[0].Value != "Dog" {
		t.Fatal("CreateChoicesFromStrings")
	}
}
