package partners

import "github.com/small-frappuccino/discordcore/pkg/files"

// BoardEmbed is a pure domain representation of a rendered partner board page.
type BoardEmbed struct {
	Title       string
	Description string
	Color       int
	FooterText  string
}

// BoardPublisher performs outbound edits to Discord.
type BoardPublisher interface {
	Publish(guildID string, postings []files.CustomEmbedPostingConfig, embeds []BoardEmbed) PartnerSyncResult
}

