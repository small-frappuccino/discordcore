/*
Package embeds implements the slash-command routing and interaction handlers
for the custom embeds feature.

This package orchestrates the ingestion of Discord interaction events, parsing
Arikawa-native command options into domain configurations, and delegating execution
to the core embeds service. It enforces strictly typed ephemeral responses and
provides operational fault tolerance for edge cases like dangling command identifiers.
*/
package embeds
