package partners

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

const (
	defaultMaxEmbedDescriptionChars = 4096
	defaultMaxEmbedsPerMessage      = 10

	defaultBoardTitle                 = "Partner Servers"
	defaultSectionHeaderTemplate      = "**{fandom}**"
	defaultLineTemplate               = "- [{name}]({link})"
	defaultEmptyStateText             = "No partner servers configured yet."
	defaultOtherFandomLabel           = "Other Servers"
	defaultSectionContinuationSuffix  = " (cont.)"
	defaultBoardContinuationTitleSufx = " (cont.)"
)

var (
	// ErrInvalidPartnerBoardEntry indicates one input item is invalid.
	ErrInvalidPartnerBoardEntry = errors.New("invalid partner board entry")
	// ErrInvalidPartnerBoardTemplate indicates template settings cannot be rendered safely.
	ErrInvalidPartnerBoardTemplate = errors.New("invalid partner board template")
	// ErrPartnerBoardExceedsEmbedLimit indicates rendered output exceeds Discord embed constraints.
	ErrPartnerBoardExceedsEmbedLimit = errors.New("partner board exceeds embed limit")
)

type normalizedTemplate struct {
	Title                      string
	ContinuationTitle          string
	Intro                      string
	SectionHeaderTemplate      string
	SectionContinuationSuffix  string
	SectionContinuationPattern string
	LineTemplate               string
	EmptyStateText             string
	FooterTemplate             string
	OtherFandomLabel           string
	Color                      int
	DisableFandomSorting       bool
	DisablePartnerSorting      bool
}

type normalizedPartner struct {
	Fandom string
	Name   string
	Link   string
}

type rendererLimits struct {
	maxDescriptionChars int
	maxEmbeds           int
}

// BoardRenderer renders partner records into final Discord embeds.
type BoardRenderer struct {
	maxDescriptionChars int
	maxEmbeds           int
}

// NewBoardRenderer creates a renderer with Discord-safe defaults.
func NewBoardRenderer() *BoardRenderer {
	return &BoardRenderer{
		maxDescriptionChars: defaultMaxEmbedDescriptionChars,
		maxEmbeds:           defaultMaxEmbedsPerMessage,
	}
}

func newBoardRendererWithLimits(maxDescriptionChars, maxEmbeds int) *BoardRenderer {
	return &BoardRenderer{
		maxDescriptionChars: maxDescriptionChars,
		maxEmbeds:           maxEmbeds,
	}
}

// Render transforms template + partner list into one or more embeds.
func (r *BoardRenderer) Render(template PartnerBoardTemplate, partners []PartnerRecord) ([]*discordgo.MessageEmbed, error) {
	limits := normalizeRendererLimits(r)
	tpl := normalizeTemplate(template)

	normalizedPartners, err := normalizePartners(partners, tpl.OtherFandomLabel)
	if err != nil {
		return nil, err
	}

	descriptions, totalFandoms, err := renderDescriptions(tpl, normalizedPartners, limits)
	if err != nil {
		return nil, err
	}

	if len(descriptions) > limits.maxEmbeds {
		return nil, fmt.Errorf(
			"%w: produced=%d limit=%d",
			ErrPartnerBoardExceedsEmbedLimit,
			len(descriptions),
			limits.maxEmbeds,
		)
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(descriptions))
	for i, description := range descriptions {
		title := tpl.Title
		if i > 0 {
			title = tpl.ContinuationTitle
		}

		embed := &discordgo.MessageEmbed{
			Title:       title,
			Description: description,
			Color:       tpl.Color,
		}
		if footer := buildFooter(tpl.FooterTemplate, len(normalizedPartners), totalFandoms, i+1, len(descriptions)); footer != "" {
			embed.Footer = &discordgo.MessageEmbedFooter{
				Text: footer,
			}
		}
		embeds = append(embeds, embed)
	}

	return embeds, nil
}

func normalizeRendererLimits(r *BoardRenderer) rendererLimits {
	limits := rendererLimits{
		maxDescriptionChars: defaultMaxEmbedDescriptionChars,
		maxEmbeds:           defaultMaxEmbedsPerMessage,
	}
	if r == nil {
		return limits
	}
	if r.maxDescriptionChars > 0 {
		limits.maxDescriptionChars = r.maxDescriptionChars
	}
	if r.maxEmbeds > 0 {
		limits.maxEmbeds = r.maxEmbeds
	}
	return limits
}

func normalizeTemplate(in PartnerBoardTemplate) normalizedTemplate {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = defaultBoardTitle
	}

	continuationTitle := strings.TrimSpace(in.ContinuationTitle)
	if continuationTitle == "" {
		continuationTitle = title + defaultBoardContinuationTitleSufx
	}

	sectionHeader := strings.TrimSpace(in.SectionHeaderTemplate)
	if sectionHeader == "" {
		sectionHeader = defaultSectionHeaderTemplate
	}

	lineTemplate := strings.TrimSpace(in.LineTemplate)
	if lineTemplate == "" {
		lineTemplate = defaultLineTemplate
	}

	emptyState := strings.TrimSpace(in.EmptyStateText)
	if emptyState == "" {
		emptyState = defaultEmptyStateText
	}

	otherFandomLabel := strings.TrimSpace(in.OtherFandomLabel)
	if otherFandomLabel == "" {
		otherFandomLabel = defaultOtherFandomLabel
	}

	continuationSuffix := strings.TrimSpace(in.SectionContinuationSuffix)
	if continuationSuffix == "" {
		continuationSuffix = defaultSectionContinuationSuffix
	}

	color := in.Color
	if color == 0 {
		color = theme.Info()
	}

	return normalizedTemplate{
		Title:                      title,
		ContinuationTitle:          continuationTitle,
		Intro:                      strings.TrimSpace(in.Intro),
		SectionHeaderTemplate:      sectionHeader,
		SectionContinuationSuffix:  continuationSuffix,
		SectionContinuationPattern: strings.TrimSpace(in.SectionContinuationPattern),
		LineTemplate:               lineTemplate,
		EmptyStateText:             emptyState,
		FooterTemplate:             strings.TrimSpace(in.FooterTemplate),
		OtherFandomLabel:           otherFandomLabel,
		Color:                      color,
		DisableFandomSorting:       in.DisableFandomSorting,
		DisablePartnerSorting:      in.DisablePartnerSorting,
	}
}

func normalizePartners(partners []PartnerRecord, otherFandomLabel string) ([]normalizedPartner, error) {
	out := make([]normalizedPartner, 0, len(partners))
	for i, p := range partners {
		fandom := sanitizeSingleLine(p.Fandom)
		if fandom == "" {
			fandom = otherFandomLabel
		}

		name := sanitizeSingleLine(p.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: partner[%d] name is required", ErrInvalidPartnerBoardEntry, i)
		}

		link, err := normalizeLink(p.Link)
		if err != nil {
			return nil, fmt.Errorf("%w: partner[%d] link: %v", ErrInvalidPartnerBoardEntry, i, err)
		}

		out = append(out, normalizedPartner{
			Fandom: fandom,
			Name:   name,
			Link:   link,
		})
	}
	return out, nil
}

func renderDescriptions(
	tpl normalizedTemplate,
	partners []normalizedPartner,
	limits rendererLimits,
) ([]string, int, error) {
	if runeLen(tpl.Intro) > limits.maxDescriptionChars {
		return nil, 0, fmt.Errorf(
			"%w: intro exceeds description limit (%d)",
			ErrInvalidPartnerBoardTemplate,
			limits.maxDescriptionChars,
		)
	}

	var (
		descriptions []string
		current      = strings.TrimSpace(tpl.Intro)
	)

	if len(partners) == 0 {
		next, carry, err := appendDescriptionChunk(descriptions, current, tpl.EmptyStateText, limits.maxDescriptionChars)
		if err != nil {
			return nil, 0, err
		}
		descriptions, current = next, carry
		return finalizeDescriptions(descriptions, current, tpl.EmptyStateText), 0, nil
	}

	grouped, fandomOrder := groupByFandom(partners)
	if !tpl.DisableFandomSorting {
		sortByFoldedText(fandomOrder)
	}

	globalIndex := 1
	for _, fandom := range fandomOrder {
		entries := append([]normalizedPartner(nil), grouped[fandom]...)
		if !tpl.DisablePartnerSorting {
			sort.SliceStable(entries, func(i, j int) bool {
				left := strings.ToLower(entries[i].Name)
				right := strings.ToLower(entries[j].Name)
				if left == right {
					return entries[i].Link < entries[j].Link
				}
				return left < right
			})
		}

		sectionFragments, nextGlobalIndex, err := renderSectionFragments(tpl, fandom, entries, globalIndex, limits.maxDescriptionChars)
		if err != nil {
			return nil, len(fandomOrder), err
		}
		globalIndex = nextGlobalIndex

		for _, fragment := range sectionFragments {
			next, carry, err := appendDescriptionChunk(descriptions, current, fragment, limits.maxDescriptionChars)
			if err != nil {
				return nil, len(fandomOrder), err
			}
			descriptions, current = next, carry
		}
	}

	return finalizeDescriptions(descriptions, current, tpl.EmptyStateText), len(fandomOrder), nil
}

func finalizeDescriptions(descriptions []string, current, fallback string) []string {
	if strings.TrimSpace(current) != "" {
		descriptions = append(descriptions, current)
	}
	if len(descriptions) == 0 {
		descriptions = append(descriptions, fallback)
	}
	return descriptions
}

func renderSectionFragments(
	tpl normalizedTemplate,
	fandom string,
	entries []normalizedPartner,
	globalStart int,
	maxDescriptionChars int,
) ([]string, int, error) {
	header := strings.TrimSpace(applyTemplate(tpl.SectionHeaderTemplate, map[string]string{
		"fandom": fandom,
		"count":  strconv.Itoa(len(entries)),
	}))
	if header == "" {
		return nil, globalStart, fmt.Errorf("%w: section header rendered empty for fandom=%q", ErrInvalidPartnerBoardTemplate, fandom)
	}

	continuationHeader := buildSectionContinuationHeader(tpl, header, fandom, len(entries))
	if continuationHeader == "" {
		return nil, globalStart, fmt.Errorf("%w: section continuation header rendered empty for fandom=%q", ErrInvalidPartnerBoardTemplate, fandom)
	}

	lines := make([]string, 0, len(entries))
	globalIndex := globalStart
	for i, entry := range entries {
		line := strings.TrimSpace(applyTemplate(tpl.LineTemplate, map[string]string{
			"fandom":       fandom,
			"name":         escapeMarkdownLinkText(entry.Name),
			"link":         entry.Link,
			"index":        strconv.Itoa(i + 1),
			"global_index": strconv.Itoa(globalIndex),
		}))
		if line == "" {
			return nil, globalStart, fmt.Errorf("%w: line template rendered empty for fandom=%q index=%d", ErrInvalidPartnerBoardTemplate, fandom, i+1)
		}
		lines = append(lines, line)
		globalIndex++
	}

	fragments, err := splitSectionIntoChunks(header, continuationHeader, lines, maxDescriptionChars)
	if err != nil {
		return nil, globalStart, err
	}
	return fragments, globalIndex, nil
}

func buildSectionContinuationHeader(tpl normalizedTemplate, header, fandom string, count int) string {
	tokens := map[string]string{
		"fandom": fandom,
		"count":  strconv.Itoa(count),
		"header": header,
	}
	if strings.TrimSpace(tpl.SectionContinuationPattern) != "" {
		return strings.TrimSpace(applyTemplate(tpl.SectionContinuationPattern, tokens))
	}
	return header + tpl.SectionContinuationSuffix
}

func splitSectionIntoChunks(header, continuationHeader string, lines []string, maxDescriptionChars int) ([]string, error) {
	if runeLen(header) > maxDescriptionChars {
		return nil, fmt.Errorf(
			"%w: section header length exceeds description limit (%d)",
			ErrInvalidPartnerBoardTemplate,
			maxDescriptionChars,
		)
	}
	if runeLen(continuationHeader) > maxDescriptionChars {
		return nil, fmt.Errorf(
			"%w: section continuation header length exceeds description limit (%d)",
			ErrInvalidPartnerBoardTemplate,
			maxDescriptionChars,
		)
	}

	if len(lines) == 0 {
		return []string{header}, nil
	}

	activeHeader := header
	current := activeHeader
	out := make([]string, 0, 1)

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		lineLimit := maxDescriptionChars - runeLen(activeHeader) - 1
		if lineLimit <= 0 {
			return nil, fmt.Errorf(
				"%w: header leaves no room for section lines",
				ErrInvalidPartnerBoardTemplate,
			)
		}
		line = truncateToRuneLimit(line, lineLimit)

		candidate := current + "\n" + line
		if runeLen(candidate) <= maxDescriptionChars {
			current = candidate
			continue
		}

		if current != activeHeader {
			out = append(out, current)
		}

		activeHeader = continuationHeader
		lineLimit = maxDescriptionChars - runeLen(activeHeader) - 1
		if lineLimit <= 0 {
			return nil, fmt.Errorf(
				"%w: continuation header leaves no room for section lines",
				ErrInvalidPartnerBoardTemplate,
			)
		}
		line = truncateToRuneLimit(line, lineLimit)

		current = activeHeader + "\n" + line
		if runeLen(current) > maxDescriptionChars {
			return nil, fmt.Errorf(
				"%w: section line cannot fit even after truncation",
				ErrInvalidPartnerBoardTemplate,
			)
		}
	}

	if strings.TrimSpace(current) != "" {
		out = append(out, current)
	}
	if len(out) == 0 {
		out = append(out, header)
	}
	return out, nil
}

func appendDescriptionChunk(
	descriptions []string,
	current string,
	chunk string,
	maxDescriptionChars int,
) ([]string, string, error) {
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return descriptions, current, nil
	}
	if runeLen(chunk) > maxDescriptionChars {
		return nil, "", fmt.Errorf(
			"%w: chunk length exceeds description limit (%d)",
			ErrInvalidPartnerBoardTemplate,
			maxDescriptionChars,
		)
	}

	if strings.TrimSpace(current) == "" {
		return descriptions, chunk, nil
	}

	candidate := current + "\n\n" + chunk
	if runeLen(candidate) <= maxDescriptionChars {
		return descriptions, candidate, nil
	}

	if runeLen(current) > maxDescriptionChars {
		return nil, "", fmt.Errorf(
			"%w: current description length exceeds description limit (%d)",
			ErrInvalidPartnerBoardTemplate,
			maxDescriptionChars,
		)
	}

	descriptions = append(descriptions, current)
	return descriptions, chunk, nil
}

func buildFooter(template string, totalPartners, totalFandoms, page, pageCount int) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	return strings.TrimSpace(applyTemplate(template, map[string]string{
		"total_partners": strconv.Itoa(totalPartners),
		"total_fandoms":  strconv.Itoa(totalFandoms),
		"embed_index":    strconv.Itoa(page),
		"embed_count":    strconv.Itoa(pageCount),
	}))
}

func groupByFandom(partners []normalizedPartner) (map[string][]normalizedPartner, []string) {
	grouped := make(map[string][]normalizedPartner, len(partners))
	order := make([]string, 0, len(partners))

	for _, p := range partners {
		if _, exists := grouped[p.Fandom]; !exists {
			order = append(order, p.Fandom)
		}
		grouped[p.Fandom] = append(grouped[p.Fandom], p)
	}
	return grouped, order
}

func normalizeLink(raw string) (string, error) {
	raw = sanitizeSingleLine(raw)
	if raw == "" {
		return "", fmt.Errorf("link is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL")
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("missing URL host")
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q", scheme)
	}

	return u.String(), nil
}

func sanitizeSingleLine(in string) string {
	out := strings.TrimSpace(in)
	out = strings.ReplaceAll(out, "\r\n", " ")
	out = strings.ReplaceAll(out, "\n", " ")
	out = strings.ReplaceAll(out, "\r", " ")
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func escapeMarkdownLinkText(in string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
	)
	return replacer.Replace(in)
}

func applyTemplate(template string, values map[string]string) string {
	if template == "" || len(values) == 0 {
		return template
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := template
	for _, key := range keys {
		out = strings.ReplaceAll(out, "{"+key+"}", values[key])
	}
	return out
}

func sortByFoldedText(items []string) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(items[i])
		right := strings.ToLower(items[j])
		if left == right {
			return items[i] < items[j]
		}
		return left < right
	})
}

func truncateToRuneLimit(in string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if runeLen(in) <= limit {
		return in
	}
	if limit <= 3 {
		return strings.Repeat(".", limit)
	}

	runes := []rune(in)
	return string(runes[:limit-3]) + "..."
}

func runeLen(in string) int {
	return utf8.RuneCountInString(in)
}
