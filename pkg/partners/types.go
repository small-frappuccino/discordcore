package partners

// PartnerRecord is one partner entry to be rendered in a board.
type PartnerRecord struct {
	Fandom string `json:"fandom,omitempty"`
	Name   string `json:"name,omitempty"`
	Link   string `json:"link,omitempty"`
}

// PartnerBoardTemplate controls how partner lists are rendered into embeds.
// Templates support token replacement with {token} placeholders.
type PartnerBoardTemplate struct {
	Title                      string `json:"title,omitempty"`
	ContinuationTitle          string `json:"continuation_title,omitempty"`
	Intro                      string `json:"intro,omitempty"`
	SectionHeaderTemplate      string `json:"section_header_template,omitempty"`       // {fandom}, {count}
	SectionContinuationSuffix  string `json:"section_continuation_suffix,omitempty"`   // suffix appended when a section spans multiple chunks
	SectionContinuationPattern string `json:"section_continuation_template,omitempty"` // {fandom}, {count}, {header}
	LineTemplate               string `json:"line_template,omitempty"`                 // {fandom}, {name}, {link}, {index}, {global_index}
	EmptyStateText             string `json:"empty_state_text,omitempty"`
	FooterTemplate             string `json:"footer_template,omitempty"` // {total_partners}, {total_fandoms}, {embed_index}, {embed_count}
	OtherFandomLabel           string `json:"other_fandom_label,omitempty"`
	Color                      int    `json:"color,omitempty"`
	DisableFandomSorting       bool   `json:"disable_fandom_sorting,omitempty"`
	DisablePartnerSorting      bool   `json:"disable_partner_sorting,omitempty"`
}
