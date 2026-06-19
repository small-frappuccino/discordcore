package core

import (
	"testing"

	"github.com/small-frappuccino/discordgo"
)

func TestOptions(t *testing.T) {
	opts := OptionList{
		{Name: "str", Type: discordgo.ApplicationCommandOptionString, Value: "value"},
		{Name: "int", Type: discordgo.ApplicationCommandOptionInteger, Value: float64(42)},
		{Name: "bool", Type: discordgo.ApplicationCommandOptionBoolean, Value: true},
		{Name: "float", Type: discordgo.ApplicationCommandOptionNumber, Value: 3.14},
	}
	if opts.String("str") != "value" {
		t.Fatal("String")
	}
	if opts.String("missing") != "" {
		t.Fatal("String missing")
	}

	val, err := opts.StringRequired("str")
	if err != nil || val != "value" {
		t.Fatal("StringRequired")
	}
	_, err = opts.StringRequired("missing")
	if err == nil {
		t.Fatal("StringRequired missing")
	}
	if opts.Int("int") != 42 {
		t.Fatal("Int")
	}
	if opts.Int("missing") != 0 {
		t.Fatal("Int missing")
	}
	if opts.Bool("bool") != true {
		t.Fatal("Bool")
	}
	if opts.Bool("missing") != false {
		t.Fatal("Bool missing")
	}
	if opts.Float("float") != 3.14 {
		t.Fatal("Float")
	}
	if opts.Float("missing") != 0.0 {
		t.Fatal("Float missing")
	}
	if !opts.HasOption("str") {
		t.Fatal("HasOption")
	}
	if opts.HasOption("missing") {
		t.Fatal("HasOption missing")
	}
	if len(opts.GetAllOptions()) != 4 {
		t.Fatal("GetAllOptions")
	}
	optStr := GetStringOption(opts, "str")
	if optStr != "value" {
		t.Fatal("GetStringOption")
	}
	optInt := GetIntegerOption(opts, "int")
	if optInt != 42 {
		t.Fatal("GetIntegerOption")
	}
	optBool := GetBooleanOption(opts, "bool")
	if optBool != true {
		t.Fatal("GetBooleanOption")
	}
	// Context wraps options for subcommands, but OptionList doesn't natively recurse in String().
	// We'll just test that we can extract standard options properly.
}
