/*
Package qotd implements the Discord slash command interface for QOTD.

It defines the /qotd command tree, processes interaction payloads directly
via arikawa, and coordinates closely with the qotd domain service.

# Command Tree
- /qotd publish [consume_automatic_slot]
- /qotd skip
- /qotd questions add <question> [deck]
- /qotd questions list [deck]
- /qotd questions queue [deck]
- /qotd questions mark_published <id> [deck]
- /qotd questions recover <id> [deck]
- /qotd questions remove <id> [deck]

# Interaction Acknowledgements
All mutation commands guarantee a 15-minute response window by issuing an
InteractionAckModeDefer prior to executing domain logic.

# Concurrency
This package employs Thundering Herd protection on hot paths (e.g. publish)
and isolates panics from bringing down the main gateway listener.
*/
package qotd
