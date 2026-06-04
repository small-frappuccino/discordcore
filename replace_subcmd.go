package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	err := filepath.WalkDir("d:\\Users\\alice\\git\\discordcore\\pkg\\discord", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// We only want to replace the TYPE core.SubCommand and core.SubCommandMeta.
		// So we replace "core.SubCommand " -> "core.Command "
		// "core.SubCommand," -> "core.Command,"
		// "core.SubCommand)" -> "core.Command)"
		// "core.SubCommand\n" -> "core.Command\n"
		// And map[string]SubCommand -> map[string]Command
		
		newContent := string(content)
		
		// In the core package, it's just SubCommand
		if strings.Contains(path, "pkg\\discord\\commands\\core") {
			newContent = strings.ReplaceAll(newContent, "map[string]SubCommand", "map[string]Command")
			newContent = strings.ReplaceAll(newContent, "map[string]map[string]SubCommand", "map[string]map[string]Command")
			newContent = strings.ReplaceAll(newContent, "(subcmd SubCommand)", "(subcmd Command)")
			newContent = strings.ReplaceAll(newContent, "[]SubCommand", "[]Command")
			newContent = strings.ReplaceAll(newContent, "SubCommandMeta", "CommandMeta")
			newContent = strings.ReplaceAll(newContent, "func (r *CommandRegistry) RegisterSubCommand(parentName string, subcmd SubCommand)", "func (r *CommandRegistry) RegisterSubCommand(parentName string, subcmd Command)")
			newContent = strings.ReplaceAll(newContent, "func (r *CommandRegistry) GetSubCommand(parentName, subName string) (SubCommand, bool)", "func (r *CommandRegistry) GetSubCommand(parentName, subName string) (Command, bool)")
			newContent = strings.ReplaceAll(newContent, "func (cr *CommandRouter) RegisterSlashSubCommand(parentName string, subcmd SubCommand)", "func (cr *CommandRouter) RegisterSlashSubCommand(parentName string, subcmd Command)")
			newContent = strings.ReplaceAll(newContent, "func (cr *CommandRouter) RegisterSlashSubCommandForDomain(domain, parentName string, subcmd SubCommand)", "func (cr *CommandRouter) RegisterSlashSubCommandForDomain(domain, parentName string, subcmd Command)")
			newContent = strings.ReplaceAll(newContent, "func (cr *CommandRouter) RegisterSubCommand(parentName string, subcmd SubCommand)", "func (cr *CommandRouter) RegisterSubCommand(parentName string, subcmd Command)")
			newContent = strings.ReplaceAll(newContent, "func (cm *CommandManager) buildGuildSubCommandOption(guildID, routePath string, subcmd SubCommand)", "func (cm *CommandManager) buildGuildSubCommandOption(guildID, routePath string, subcmd Command)")
			newContent = strings.ReplaceAll(newContent, "func sortedSubCommandNames(subcommands map[string]SubCommand)", "func sortedSubCommandNames(subcommands map[string]Command)")
			newContent = strings.ReplaceAll(newContent, "func (r *CommandRegistry) GetAllSubCommands(parentName string) map[string]SubCommand", "func (r *CommandRegistry) GetAllSubCommands(parentName string) map[string]Command")
			newContent = strings.ReplaceAll(newContent, "func newMockSubCommand(name string) SubCommand", "func newMockSubCommand(name string) Command")
		} else {
			// Outside core package
			newContent = strings.ReplaceAll(newContent, "core.SubCommand ", "core.Command ")
			newContent = strings.ReplaceAll(newContent, "core.SubCommand)", "core.Command)")
			newContent = strings.ReplaceAll(newContent, "core.SubCommand,", "core.Command,")
			newContent = strings.ReplaceAll(newContent, "core.SubCommand\n", "core.Command\n")
			newContent = strings.ReplaceAll(newContent, "core.SubCommandMeta", "core.CommandMeta")
			newContent = strings.ReplaceAll(newContent, "map[string]core.SubCommand", "map[string]core.Command")
		}

		if string(content) != newContent {
			fmt.Println("Updating:", path)
			return os.WriteFile(path, []byte(newContent), 0644)
		}

		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
