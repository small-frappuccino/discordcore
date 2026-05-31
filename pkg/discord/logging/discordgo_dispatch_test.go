package logging

import (
	_ "unsafe"

	"github.com/bwmarrin/discordgo"
)

//go:linkname discordgoHandleEvent github.com/bwmarrin/discordgo.(*Session).handleEvent
func discordgoHandleEvent(session *discordgo.Session, eventType string, payload any)

func dispatchDiscordEvent(session *discordgo.Session, eventType string, payload any) {
	if session == nil {
		return
	}
	session.SyncEvents = true
	discordgoHandleEvent(session, eventType, payload)
}
