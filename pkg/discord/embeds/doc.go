/*
Package embeds implements the rendering and synchronization engine for custom Discord embeds.

This package isolates the core domain logic required to translate repository-native
embed configurations (such as files.CustomEmbedConfig) into strictly typed payloads
consumable by the Discord API (via the arikawa client). It guarantees that active
Discord messages remain structurally coherent with the local configuration files.

The synchronization pipeline employs a best-effort, fault-tolerant batch processing
strategy. It enforces operational guardrails against transient network failures
and unknown resource identifiers, mitigating thundering herd phenomena while
reconciling state drift.
*/
package embeds
