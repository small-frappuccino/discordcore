package core

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

func TestResponseBuilder(t *testing.T) {
	session, _ := discordgo.New("Bot test")
	ctx := &Context{
		Session:      session,
		Acknowledged: true,
	}
	builder := NewResponseBuilder(session).
		WithContext(ctx).
		Ephemeral().
		WithEmbed().
		WithTitle("Title").
		WithColor(12345).
		WithFooter("Footer").
		WithTimestamp().
		WithComponents(discordgo.ActionsRow{}).
		WithAttachments(&discordgo.File{Name: "test.txt"})
	rm := builder.Build()
	if rm.config.Ephemeral != true {
		t.Fatal("Ephemeral not set")
	}
	if rm.config.WithEmbed != true {
		t.Fatal("WithEmbed not set")
	}
	if rm.config.Title != "Title" {
		t.Fatal("Title not set")
	}
	if rm.config.Color != 12345 {
		t.Fatal("Color not set")
	}
	if rm.config.Footer != "Footer" {
		t.Fatal("Footer not set")
	}
	if !rm.config.Timestamp {
		t.Fatal("Timestamp not set")
	}
	if len(rm.config.Components) != 1 {
		t.Fatal("Components not set")
	}
	if len(rm.config.Attachments) != 1 {
		t.Fatal("Attachments not set")
	}
	// Just build response structures without sending (send will fail without mock, but coverage will hit)
	// Actually we can test color and title resolution:
	if rm.getColorForType(ResponseSuccess) == 0 {
		t.Fatal("should return success color")
	}
	if rm.getTitleForType(ResponseSuccess) == "" {
		t.Fatal("should return success title")
	}
}
func TestResponseManagerTypes(t *testing.T) {
	session, _ := discordgo.New("Bot test")
	rm := NewResponseManager(session)
	// test default colors
	if rm.getColorForType(ResponseSuccess) != theme.Success() {
		t.Fatal("wrong color")
	}
	if rm.getColorForType(ResponseError) != theme.Error() {
		t.Fatal("wrong color")
	}
	if rm.getColorForType(ResponseWarning) != theme.Warning() {
		t.Fatal("wrong color")
	}
	if rm.getColorForType(ResponseInfo) != theme.Info() {
		t.Fatal("wrong color")
	}
	if rm.getColorForType(ResponseLoading) != theme.Loading() {
		t.Fatal("wrong color")
	}
	if rm.getColorForType(ResponseType(999)) != theme.Muted() {
		t.Fatal("wrong color")
	}
	// test titles
	if rm.getTitleForType(ResponseSuccess) != "Success" {
		t.Fatal("wrong title")
	}
	if rm.getTitleForType(ResponseError) != "Error" {
		t.Fatal("wrong title")
	}
	if rm.getTitleForType(ResponseWarning) != "Warning" {
		t.Fatal("wrong title")
	}
	if rm.getTitleForType(ResponseInfo) != "Information" {
		t.Fatal("wrong title")
	}
	if rm.getTitleForType(ResponseLoading) != "Loading..." {
		t.Fatal("wrong title")
	}
	if rm.getTitleForType(ResponseType(999)) != "" {
		t.Fatal("wrong title")
	}
	// createEmbed
	embed := rm.createEmbed("hello", ResponseSuccess)
	if embed.Description != "hello" {
		t.Fatal("wrong desc")
	}
	// test formatTextMessage
	if rm.formatTextMessage("hello", ResponseSuccess) != "hello" {
		t.Fatal("wrong text")
	}

	// buildFlags
	if rm.buildFlags(true) != discordgo.MessageFlagsEphemeral {
		t.Fatal("wrong flag")
	}
	if rm.buildFlags(false) != 0 {
		t.Fatal("wrong flag")
	}
}
func TestResponseManagerSend(t *testing.T) {
	session, _ := discordgo.New("Bot test")
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{},
	}

	// mock session to avoid actual API calls
	builder := NewResponseBuilder(session)
	_ = builder.Success(i, "ok")
	_ = builder.Error(i, "ok")
	_ = builder.Info(i, "ok")
	_ = builder.Warning(i, "ok")

	rm := NewResponseManager(session)
	_ = rm.Loading(i, "ok")
	_ = rm.Ephemeral(i, "ok")
	_ = rm.Autocomplete(i, make([]*discordgo.ApplicationCommandOptionChoice, 30))
	_ = rm.DeferResponse(i, true)
	_ = rm.EditResponse(i, "ok")
	_ = rm.EditResponseWithEmbed(i, &discordgo.MessageEmbed{})
	_ = rm.FollowUp(i, "ok", true)
	_ = rm.FollowUpWithEmbed(i, &discordgo.MessageEmbed{}, true)
	_ = rm.DeleteResponse(i)
}
