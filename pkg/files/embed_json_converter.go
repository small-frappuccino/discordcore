package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrEmbedJSONValidation = errors.New("embed json validation failed")
)

// DiscohookJSON represents the standard structure of a Discord message JSON
// commonly used by tools like Discohook.
type DiscohookJSON struct {
	Content string           `json:"content,omitempty"`
	Embeds  []DiscohookEmbed `json:"embeds,omitempty"`
}

// DiscohookEmbed mirrors a single Discord embed in the Discohook JSON schema.
// Color is the Discord decimal color value; pointer fields are absent when nil.
type DiscohookEmbed struct {
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Color       int              `json:"color,omitempty"`
	Author      *DiscohookAuthor `json:"author,omitempty"`
	Footer      *DiscohookFooter `json:"footer,omitempty"`
	Image       *DiscohookImage  `json:"image,omitempty"`
	Thumbnail   *DiscohookImage  `json:"thumbnail,omitempty"`
	Fields      []DiscohookField `json:"fields,omitempty"`
}

// DiscohookAuthor is the author block of a DiscohookEmbed.
type DiscohookAuthor struct {
	Name    string `json:"name,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscohookFooter is the footer block of a DiscohookEmbed.
type DiscohookFooter struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscohookImage is an image or thumbnail reference in a DiscohookEmbed.
type DiscohookImage struct {
	URL string `json:"url,omitempty"`
}

// DiscohookField is a single name/value field of a DiscohookEmbed; Inline lays
// the field alongside adjacent inline fields.
type DiscohookField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

// ParseAndValidateDiscohookJSON parses the raw JSON payload and strictly enforces
// Discord's embed limits, returning the first embed found or an error.
func ParseAndValidateDiscohookJSON(data []byte) (DiscohookEmbed, error) {
	var payload DiscohookJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return DiscohookEmbed{}, fmt.Errorf("%w: invalid JSON format: %w", ErrEmbedJSONValidation, err)
	}

	if len(payload.Embeds) == 0 {
		return DiscohookEmbed{}, fmt.Errorf("%w: no embeds found in JSON payload", ErrEmbedJSONValidation)
	}

	embed := payload.Embeds[0]

	if utf8.RuneCountInString(embed.Title) > CustomEmbedTitleMaxLen {
		return DiscohookEmbed{}, fmt.Errorf("%w: title exceeds %d characters", ErrEmbedJSONValidation, CustomEmbedTitleMaxLen)
	}
	if utf8.RuneCountInString(embed.Description) > CustomEmbedDescriptionMaxLen {
		return DiscohookEmbed{}, fmt.Errorf("%w: description exceeds %d characters", ErrEmbedJSONValidation, CustomEmbedDescriptionMaxLen)
	}
	if embed.Color < 0 || embed.Color > CustomEmbedColorMax {
		return DiscohookEmbed{}, fmt.Errorf("%w: color %d is out of bounds [0, %d]", ErrEmbedJSONValidation, embed.Color, CustomEmbedColorMax)
	}
	if embed.Author != nil && utf8.RuneCountInString(embed.Author.Name) > CustomEmbedAuthorMaxLen {
		return DiscohookEmbed{}, fmt.Errorf("%w: author name exceeds %d characters", ErrEmbedJSONValidation, CustomEmbedAuthorMaxLen)
	}
	if embed.Footer != nil && utf8.RuneCountInString(embed.Footer.Text) > CustomEmbedFooterMaxLen {
		return DiscohookEmbed{}, fmt.Errorf("%w: footer text exceeds %d characters", ErrEmbedJSONValidation, CustomEmbedFooterMaxLen)
	}

	if len(embed.Fields) > CustomEmbedMaxFields {
		return DiscohookEmbed{}, fmt.Errorf("%w: embed contains more than %d fields", ErrEmbedJSONValidation, CustomEmbedMaxFields)
	}

	for i, f := range embed.Fields {
		if strings.TrimSpace(f.Name) == "" {
			return DiscohookEmbed{}, fmt.Errorf("%w: field %d name is required", ErrEmbedJSONValidation, i+1)
		}
		if strings.TrimSpace(f.Value) == "" {
			return DiscohookEmbed{}, fmt.Errorf("%w: field %d value is required", ErrEmbedJSONValidation, i+1)
		}
		if utf8.RuneCountInString(f.Name) > CustomEmbedFieldNameMaxLen {
			return DiscohookEmbed{}, fmt.Errorf("%w: field %d name exceeds %d characters", ErrEmbedJSONValidation, i+1, CustomEmbedFieldNameMaxLen)
		}
		if utf8.RuneCountInString(f.Value) > CustomEmbedFieldValueMaxLen {
			return DiscohookEmbed{}, fmt.Errorf("%w: field %d value exceeds %d characters", ErrEmbedJSONValidation, i+1, CustomEmbedFieldValueMaxLen)
		}
	}

	totalLen := utf8.RuneCountInString(embed.Title) + utf8.RuneCountInString(embed.Description)
	if embed.Author != nil {
		totalLen += utf8.RuneCountInString(embed.Author.Name)
	}
	if embed.Footer != nil {
		totalLen += utf8.RuneCountInString(embed.Footer.Text)
	}
	for _, f := range embed.Fields {
		totalLen += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
	}

	if totalLen > CustomEmbedMaxTotalLen {
		return DiscohookEmbed{}, fmt.Errorf("%w: embed total character count (%d) exceeds the maximum of %d", ErrEmbedJSONValidation, totalLen, CustomEmbedMaxTotalLen)
	}

	return embed, nil
}

// ToCustomEmbedConfig converts a DiscohookEmbed into our internal CustomEmbedConfig format.
func ToCustomEmbedConfig(embed DiscohookEmbed, key string) CustomEmbedConfig {
	out := CustomEmbedConfig{
		Key:         key,
		Title:       embed.Title,
		Description: embed.Description,
		Color:       embed.Color,
	}

	if embed.Author != nil {
		out.AuthorName = embed.Author.Name
		out.AuthorIconURL = embed.Author.IconURL
	}
	if embed.Footer != nil {
		out.FooterText = embed.Footer.Text
		out.FooterIconURL = embed.Footer.IconURL
	}
	if embed.Image != nil {
		out.ImageURL = embed.Image.URL
	}
	if embed.Thumbnail != nil {
		out.ThumbnailURL = embed.Thumbnail.URL
	}

	if len(embed.Fields) > 0 {
		out.Fields = make([]CustomEmbedFieldConfig, 0, len(embed.Fields))
		for _, f := range embed.Fields {
			out.Fields = append(out.Fields, CustomEmbedFieldConfig{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return out
}

// FromCustomEmbedConfig exports a CustomEmbedConfig into a DiscohookJSON object.
func FromCustomEmbedConfig(ce CustomEmbedConfig) DiscohookJSON {
	embed := buildDiscohookEmbedBase(discohookEmbedBase{
		Title:       ce.Title,
		Description: ce.Description,
		Color:       ce.Color,
		AuthorName:  ce.AuthorName,
		AuthorIcon:  ce.AuthorIconURL,
		FooterText:  ce.FooterText,
		FooterIcon:  ce.FooterIconURL,
		ImageURL:    ce.ImageURL,
		ThumbURL:    ce.ThumbnailURL,
	})

	if len(ce.Fields) > 0 {
		embed.Fields = make([]DiscohookField, 0, len(ce.Fields))
		for _, f := range ce.Fields {
			embed.Fields = append(embed.Fields, DiscohookField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return DiscohookJSON{
		Embeds: []DiscohookEmbed{embed},
	}
}

// ToRolePanelConfig converts a DiscohookEmbed into our internal RolePanelConfig format.
func ToRolePanelConfig(embed DiscohookEmbed, key string) RolePanelConfig {
	out := RolePanelConfig{
		Key:         key,
		Title:       embed.Title,
		Description: embed.Description,
		Color:       embed.Color,
	}

	if embed.Author != nil {
		out.AuthorName = embed.Author.Name
		out.AuthorIconURL = embed.Author.IconURL
	}
	if embed.Footer != nil {
		out.FooterText = embed.Footer.Text
		out.FooterIconURL = embed.Footer.IconURL
	}
	if embed.Image != nil {
		out.ImageURL = embed.Image.URL
	}
	if embed.Thumbnail != nil {
		out.ThumbnailURL = embed.Thumbnail.URL
	}

	if len(embed.Fields) > 0 {
		out.Fields = make([]RolePanelEmbedFieldConfig, 0, len(embed.Fields))
		for _, f := range embed.Fields {
			out.Fields = append(out.Fields, RolePanelEmbedFieldConfig{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return out
}

// FromRolePanelConfig exports a RolePanelConfig into a DiscohookJSON object.
func FromRolePanelConfig(rp RolePanelConfig) DiscohookJSON {
	embed := buildDiscohookEmbedBase(discohookEmbedBase{
		Title:       rp.Title,
		Description: rp.Description,
		Color:       rp.Color,
		AuthorName:  rp.AuthorName,
		AuthorIcon:  rp.AuthorIconURL,
		FooterText:  rp.FooterText,
		FooterIcon:  rp.FooterIconURL,
		ImageURL:    rp.ImageURL,
		ThumbURL:    rp.ThumbnailURL,
	})

	if len(rp.Fields) > 0 {
		embed.Fields = make([]DiscohookField, 0, len(rp.Fields))
		for _, f := range rp.Fields {
			embed.Fields = append(embed.Fields, DiscohookField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return DiscohookJSON{
		Embeds: []DiscohookEmbed{embed},
	}
}

// ToPartnerBoardTemplate populates a PartnerBoardTemplateConfig from a DiscohookEmbed.
// It maps the embed title, description (Intro), color, and footer text.
func ToPartnerBoardTemplate(embed DiscohookEmbed, current PartnerBoardTemplateConfig) PartnerBoardTemplateConfig {
	out := current
	out.Title = embed.Title
	out.Intro = embed.Description
	out.Color = embed.Color
	if embed.Footer != nil {
		out.FooterTemplate = embed.Footer.Text
	} else {
		out.FooterTemplate = ""
	}
	return out
}

// FromPartnerBoardTemplate exports a PartnerBoardTemplateConfig into a mock DiscohookJSON object.
func FromPartnerBoardTemplate(tmpl PartnerBoardTemplateConfig) DiscohookJSON {
	embed := buildDiscohookEmbedBase(discohookEmbedBase{
		Title:       tmpl.Title,
		Description: tmpl.Intro,
		Color:       tmpl.Color,
		FooterText:  tmpl.FooterTemplate,
	})
	return DiscohookJSON{
		Embeds: []DiscohookEmbed{embed},
	}
}

// discohookEmbedBase carries the flat embed fields shared by the
// CustomEmbedConfig, RolePanelConfig, and PartnerBoardTemplateConfig
// exporters. buildDiscohookEmbedBase promotes the non-empty author, footer,
// image, and thumbnail values into their nested embed blocks.
type discohookEmbedBase struct {
	Title       string
	Description string
	Color       int
	AuthorName  string
	AuthorIcon  string
	FooterText  string
	FooterIcon  string
	ImageURL    string
	ThumbURL    string
}

func buildDiscohookEmbedBase(base discohookEmbedBase) DiscohookEmbed {
	embed := DiscohookEmbed{
		Title:       base.Title,
		Description: base.Description,
		Color:       base.Color,
	}

	if base.AuthorName != "" || base.AuthorIcon != "" {
		embed.Author = &DiscohookAuthor{
			Name:    base.AuthorName,
			IconURL: base.AuthorIcon,
		}
	}
	if base.FooterText != "" || base.FooterIcon != "" {
		embed.Footer = &DiscohookFooter{
			Text:    base.FooterText,
			IconURL: base.FooterIcon,
		}
	}
	if base.ImageURL != "" {
		embed.Image = &DiscohookImage{URL: base.ImageURL}
	}
	if base.ThumbURL != "" {
		embed.Thumbnail = &DiscohookImage{URL: base.ThumbURL}
	}

	return embed
}
