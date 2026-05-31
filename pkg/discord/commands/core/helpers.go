package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// GenerateID generates a unique ID based on the current timestamp
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// FormatOptions format options for logging
func FormatOptions(options []*discordgo.ApplicationCommandOption) string {
	if len(options) == 0 {
		return ""
	}
	var parts []string
	for _, opt := range options {
		parts = append(parts, fmt.Sprintf("%s (%s)", opt.Name, opt.Type.String()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
