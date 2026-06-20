/*
Package runtime implements the administrative configuration panel for discordcore.

It provides an interactive, ephemeral dashboard surfaced via slash commands that
allows authorized administrators to modify the bot's runtime behavior without
requiring a full restart. This package orchestrates component states, configuration
mutations, and UI rendering entirely through the arikawa Discord API client.

The architecture is divided into strictly separated layers:
  - state.go: Transport layer handling payload serialization and cryptographic authorization.
  - config.go: Data layer managing schema validation and ConfigManager concurrency.
  - view.go: Presentation layer rendering arikawa-compliant component structures.
  - commands.go: Controller layer handling dispatch, routing, and HTTP API interaction.
*/
package runtime
