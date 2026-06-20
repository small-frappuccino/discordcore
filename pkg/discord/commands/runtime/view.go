package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// The presentation layer translates memory structures strictly into arikawa payloads.
const (
	cidSelectKey    = customIDPrefix + "select:key"
	cidSelectGroup  = customIDPrefix + "select:group"
	cidButtonMain   = customIDPrefix + "nav:main"
	cidButtonHelp   = customIDPrefix + "nav:help"
	cidButtonBack   = customIDPrefix + "nav:back"
	cidButtonDetail = customIDPrefix + "action:details"
	cidButtonToggle = customIDPrefix + "action:toggle"
	cidButtonEdit   = customIDPrefix + "action:edit"
	cidButtonReset  = customIDPrefix + "action:reset"
	cidButtonReload = customIDPrefix + "action:reload"
)

// fieldsForLines rigorously chunks grouped text configurations to ensure strict compliance
// with Discord's REST API limitations of exactly 1024 bytes per EmbedField value.
func fieldsForLines(name string, lines []string) []discord.EmbedField {
	if len(lines) == 0 {
		return []discord.EmbedField{{Name: name, Value: "(no keys)"}}
	}

	const maxValueLen = 1024
	var out []discord.EmbedField
	curName := name
	curVal := ""

	flush := func() {
		if curVal == "" {
			return
		}
		out = append(out, discord.EmbedField{
			Name:   curName,
			Value:  curVal,
			Inline: false,
		})
		curName = name + " (cont.)"
		curVal = ""
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for len(line) > 0 {
			if curVal != "" {
				if len(curVal)+1+len(line) <= maxValueLen {
					curVal += "\n" + line
					break
				}
				flush()
			}

			if len(line) <= maxValueLen {
				curVal = line
				break
			}

			chunkBytes := 0
			for i, r := range line {
				runeBytes := len(string(r))
				if chunkBytes+runeBytes > maxValueLen {
					curVal = line[:i]
					line = line[i:]
					flush()
					break
				}
				chunkBytes += runeBytes
			}
		}
	}
	flush()

	if len(out) == 0 {
		out = append(out, discord.EmbedField{Name: name, Value: "(no keys)"})
	}
	return out
}

// formatForEmbed provides a visually condensed representation of a state field.
func formatForEmbed(raw string, sp spec) string {
	if raw == "" {
		return "*(default)*"
	}
	if sp.RedactInMain {
		return "*(redacted)*"
	}
	if len(raw) > 50 {
		return raw[:47] + "..."
	}
	return raw
}

// formatForDetails provides a complete, unrestricted view of a state field.
func formatForDetails(raw string, sp spec) string {
	if raw == "" {
		return "*(default)*"
	}
	return raw
}

// renderMainEmbed constructs the primary visualization layer utilizing arikawa primitives natively.
func renderMainEmbed(rc files.RuntimeConfig, st panelState) discord.Embed {
	sp, _ := specByKey(st.Key)

	scopeDesc := "Global"
	if st.Scope != "global" {
		scopeDesc = fmt.Sprintf("Guild (`%s`)", st.Scope)
	}

	desc := strings.Join([]string{
		"This panel lets you edit the persisted runtime configuration that replaced the old operational environment variables.",
		"",
		fmt.Sprintf("Scope: **%s**", scopeDesc),
		fmt.Sprintf("Selected: `%s` | Type: **%s** | Default: **%s** | %s", sp.Key, sp.Type, sp.DefaultHint, sp.RestartHint),
		"Use the menus to filter and navigate, then use the buttons to edit the selected setting.",
	}, "\n")

	fields := []discord.EmbedField{}
	fields = append(fields, groupFieldsForMain(rc, st)...)

	return discord.Embed{
		Title:       "Runtime Configuration",
		Description: desc,
		Color:       0x3498db, // Theme Info
		Fields:      fields,
		Footer: &discord.EmbedFooter{
			Text: "Some changes can be applied immediately, especially THEME and selected ALICE_DISABLE_* settings.",
		},
		Timestamp: discord.NewTimestamp(time.Now()),
	}
}

func groupFieldsForMain(rc files.RuntimeConfig, st panelState) []discord.EmbedField {
	specs := specsForGroup(st.Group)

	grouped := map[string][]string{}
	for _, sp := range specs {
		if sp.GuildOnly && st.Scope == "global" {
			continue
		}
		raw, _ := getValue(rc, sp.Key)
		display := formatForEmbed(raw, sp)
		line := fmt.Sprintf("`%s`: **%s**", sp.Key, display)
		grouped[sp.Group] = append(grouped[sp.Group], line)
	}

	groupOrder := []string{"THEME", "SERVICES (LOGGING)", "MODERATION", "MESSAGE CACHE", "BACKFILL", "SAFETY", "VERIFICATION"}
	fields := []discord.EmbedField{}

	if st.Group != "" && st.Group != "ALL" {
		lines := grouped[st.Group]
		fields = append(fields, fieldsForLines(st.Group, lines)...)
		return fields
	}

	for _, g := range groupOrder {
		lines := grouped[g]
		if len(lines) == 0 {
			continue
		}
		fields = append(fields, fieldsForLines(g, lines)...)
		if len(fields) >= 25 {
			break
		}
	}

	return fields
}

// renderDetailsEmbed renders an expanded state diagnostic for isolated value inspection.
func renderDetailsEmbed(rc files.RuntimeConfig, st panelState) discord.Embed {
	sp, ok := specByKey(st.Key)
	if !ok {
		return errorEmbed("Unknown key")
	}
	raw, _ := getValue(rc, sp.Key)
	cur := formatForDetails(raw, sp)

	scopeDesc := "Global"
	if st.Scope != "global" {
		scopeDesc = fmt.Sprintf("Guild (`%s`)", st.Scope)
	}

	lines := []string{
		fmt.Sprintf("`%s`", sp.Key),
		"",
		fmt.Sprintf("**Scope:** %s", scopeDesc),
		fmt.Sprintf("**Group:** %s", sp.Group),
		fmt.Sprintf("**Type:** %s", sp.Type),
		fmt.Sprintf("**Default:** %s", sp.DefaultHint),
		fmt.Sprintf("**Current:** %s", cur),
		"",
		fmt.Sprintf("**Description:** %s", sp.ShortHelp),
		fmt.Sprintf("**Effect:** %s", sp.RestartHint),
	}

	if sp.GuildOnly {
		lines = append(lines, "", "**Note:** This setting can only be configured per guild.")
	}

	return discord.Embed{
		Title:       "Runtime Configuration - Details",
		Description: strings.Join(lines, "\n"),
		Color:       0x95a5a6, // Theme Muted
		Footer: &discord.EmbedFooter{
			Text: "Use BACK to return to the panel.",
		},
		Timestamp: discord.NewTimestamp(time.Now()),
	}
}

func renderHelpEmbed() discord.Embed {
	desc := strings.Join([]string{
		"This panel edits the persisted `runtime_config`.",
		"",
		"**Notes:**",
		"- Names stay in ALL CAPS so they still map cleanly to the old env var mental model.",
		"- The bot no longer reads these options from the environment, except for the token.",
		"- Some changes can be hot-applied, especially THEME and selected ALICE_DISABLE_* settings.",
		"",
		"**How to edit:**",
		"1) Filter by group if needed and select a key.",
		"2) For boolean values, use TOGGLE.",
		"3) For other values, use EDIT and fill in the modal.",
		"4) RESET clears the saved value and restores the code default.",
	}, "\n")

	return discord.Embed{
		Title:       "Runtime Configuration - Help",
		Description: desc,
		Color:       0x3498db, // Theme Info
		Timestamp:   discord.NewTimestamp(time.Now()),
	}
}

// errorEmbed standardizes catastrophic boundary failures for UI visualization.
func errorEmbed(msg string) discord.Embed {
	return discord.Embed{
		Title:       "Runtime Error",
		Description: msg,
		Color:       0xe74c3c, // Theme Error
		Timestamp:   discord.NewTimestamp(time.Now()),
	}
}

// withHotApplyWarning conditionally mutates an embed organically to inject failure warnings post-mutation.
func withHotApplyWarning(embed discord.Embed, applyErr error) discord.Embed {
	if applyErr == nil {
		return embed
	}

	clone := embed
	msg := fmt.Sprintf(
		"The runtime configuration was saved, but the change couldn't be applied immediately. A restart may be required.\nError: %v",
		applyErr,
	)
	if strings.TrimSpace(clone.Description) == "" {
		clone.Description = msg
	} else {
		clone.Description = strings.TrimSpace(clone.Description) + "\n\n" + msg
	}
	return clone
}

// renderMainComponents translates structural dependencies into an arikawa interactable component array.
func renderMainComponents(rc files.RuntimeConfig, st panelState) discord.ContainerComponents {
	return discord.ContainerComponents{
		renderGroupSelectRow(st),
		renderKeySelectRow(st),
		renderActionRow(st),
		renderNavRow(st),
	}
}

func renderDetailComponents(st panelState) discord.ContainerComponents {
	return discord.ContainerComponents{
		&discord.ActionRowComponent{
			&discord.ButtonComponent{
				CustomID: discord.ComponentID(cidButtonBack + stateSep + st.withMode(pageMain).encode()),
				Label:    "BACK",
				Style:    discord.SecondaryButtonStyle(),
			},
			&discord.ButtonComponent{
				CustomID: discord.ComponentID(cidButtonReload + stateSep + st.withMode(pageDetail).encode()),
				Label:    "RELOAD",
				Style:    discord.SecondaryButtonStyle(),
			},
		},
	}
}

func renderHelpComponents(st panelState) discord.ContainerComponents {
	return discord.ContainerComponents{
		&discord.ActionRowComponent{
			&discord.ButtonComponent{
				CustomID: discord.ComponentID(cidButtonBack + stateSep + st.withMode(pageMain).encode()),
				Label:    "BACK",
				Style:    discord.SecondaryButtonStyle(),
			},
		},
	}
}

func renderGroupSelectRow(st panelState) *discord.ActionRowComponent {
	groups := allGroups()
	opts := make([]discord.SelectOption, 0, len(groups))
	for _, g := range groups {
		opts = append(opts, discord.SelectOption{
			Label:       g,
			Value:       st.withGroup(g).withMode(pageMain).encode(),
			Description: "Filter keys by group",
			Default:     g == st.Group,
		})
	}

	return &discord.ActionRowComponent{
		&discord.StringSelectComponent{
			CustomID:    discord.ComponentID(cidSelectGroup),
			Options:     opts,
			Placeholder: "Filter by group",
		},
	}
}

func renderKeySelectRow(st panelState) *discord.ActionRowComponent {
	specs := specsForGroup(st.Group)
	opts := make([]discord.SelectOption, 0, len(specs))

	// Max 25 components in a Select Menu in Discord
	for i, sp := range specs {
		if i >= 25 {
			break
		}
		opts = append(opts, discord.SelectOption{
			Label:       string(sp.Key),
			Value:       st.withKey(sp.Key).withMode(pageMain).encode(),
			Description: sp.ShortHelp,
			Default:     sp.Key == st.Key,
		})
	}

	if len(opts) == 0 {
		opts = append(opts, discord.SelectOption{
			Label:       "No keys",
			Value:       st.encode(),
			Description: "No keys available in this group",
		})
	}

	return &discord.ActionRowComponent{
		&discord.StringSelectComponent{
			CustomID:    discord.ComponentID(cidSelectKey),
			Options:     opts,
			Placeholder: "Select a configuration key",
		},
	}
}

func renderActionRow(st panelState) *discord.ActionRowComponent {
	st = st.withMode(pageMain)

	// Operational annotation: Button arrays map dynamically to the defined spec layer logic.
	sp, ok := specByKey(st.Key)
	if !ok {
		return &discord.ActionRowComponent{}
	}

	components := []discord.InteractiveComponent{
		&discord.ButtonComponent{
			CustomID: discord.ComponentID(cidButtonDetail + stateSep + st.encode()),
			Label:    "DETAILS",
			Style:    discord.SecondaryButtonStyle(),
		},
	}

	if sp.Type == vtBool {
		components = append(components, &discord.ButtonComponent{
			CustomID: discord.ComponentID(cidButtonToggle + stateSep + st.encode()),
			Label:    "TOGGLE",
			Style:    discord.SuccessButtonStyle(),
		})
	} else {
		components = append(components, &discord.ButtonComponent{
			CustomID: discord.ComponentID(cidButtonEdit + stateSep + st.encode()),
			Label:    "EDIT",
			Style:    discord.PrimaryButtonStyle(),
		})
	}

	components = append(components, &discord.ButtonComponent{
		CustomID: discord.ComponentID(cidButtonReset + stateSep + st.encode()),
		Label:    "RESET",
		Style:    discord.DangerButtonStyle(),
	})

	row := discord.ActionRowComponent(components)
	return &row
}

func renderNavRow(st panelState) *discord.ActionRowComponent {
	return &discord.ActionRowComponent{
		&discord.ButtonComponent{
			CustomID: discord.ComponentID(cidButtonHelp + stateSep + st.withMode(pageHelp).encode()),
			Label:    "HELP",
			Style:    discord.SecondaryButtonStyle(),
		},
		&discord.ButtonComponent{
			CustomID: discord.ComponentID(cidButtonReload + stateSep + st.withMode(pageMain).encode()),
			Label:    "RELOAD",
			Style:    discord.SecondaryButtonStyle(),
		},
	}
}
