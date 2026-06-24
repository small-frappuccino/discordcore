package files

import (
	"errors"
	"strings"
)

type CustomEmbedFieldConfig struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type CustomEmbedPostingConfig struct {
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
	WebhookID    string `json:"webhook_id,omitempty"`
	WebhookToken string `json:"webhook_token,omitempty"`
}

func (p CustomEmbedPostingConfig) IsZero() bool {
	return strings.TrimSpace(p.ChannelID) == "" &&
		strings.TrimSpace(p.MessageID) == "" &&
		strings.TrimSpace(p.WebhookID) == "" &&
		strings.TrimSpace(p.WebhookToken) == ""
}

type CustomEmbedConfig struct {
	Key           string                     `json:"key"`
	Title         string                     `json:"title,omitempty"`
	Description   string                     `json:"description,omitempty"`
	Color         int                        `json:"color,omitempty"`
	AuthorName    string                     `json:"author_name,omitempty"`
	AuthorIconURL string                     `json:"author_icon_url,omitempty"`
	FooterText    string                     `json:"footer_text,omitempty"`
	FooterIconURL string                     `json:"footer_icon_url,omitempty"`
	ImageURL      string                     `json:"image_url,omitempty"`
	ThumbnailURL  string                     `json:"thumbnail_url,omitempty"`
	Fields        []CustomEmbedFieldConfig   `json:"fields,omitempty"`
	Postings      []CustomEmbedPostingConfig `json:"postings,omitempty"`
}

func (cfg CustomEmbedConfig) IsZero() bool {
	return strings.TrimSpace(cfg.Key) == "" &&
		strings.TrimSpace(cfg.Title) == "" &&
		strings.TrimSpace(cfg.Description) == "" &&
		cfg.Color == 0 &&
		strings.TrimSpace(cfg.AuthorName) == "" &&
		strings.TrimSpace(cfg.AuthorIconURL) == "" &&
		strings.TrimSpace(cfg.FooterText) == "" &&
		strings.TrimSpace(cfg.FooterIconURL) == "" &&
		strings.TrimSpace(cfg.ImageURL) == "" &&
		strings.TrimSpace(cfg.ThumbnailURL) == "" &&
		len(cfg.Fields) == 0 &&
		len(cfg.Postings) == 0
}

var ErrCustomEmbedPostingNotFound = errors.New("custom embed posting not found")

func cloneCustomEmbeds(in []CustomEmbedConfig) []CustomEmbedConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]CustomEmbedConfig, 0, len(in))
	for _, ce := range in {
		out = append(out, cloneCustomEmbed(ce))
	}
	return out
}

func cloneCustomEmbed(in CustomEmbedConfig) CustomEmbedConfig {
	out := CustomEmbedConfig{
		Key:           in.Key,
		Title:         in.Title,
		Description:   in.Description,
		Color:         in.Color,
		AuthorName:    in.AuthorName,
		AuthorIconURL: in.AuthorIconURL,
		FooterText:    in.FooterText,
		FooterIconURL: in.FooterIconURL,
		ImageURL:      in.ImageURL,
		ThumbnailURL:  in.ThumbnailURL,
	}

	if len(in.Fields) > 0 {
		out.Fields = make([]CustomEmbedFieldConfig, len(in.Fields))
		copy(out.Fields, in.Fields)
	}

	if len(in.Postings) > 0 {
		out.Postings = make([]CustomEmbedPostingConfig, len(in.Postings))
		copy(out.Postings, in.Postings)
	}

	return out
}
