/*
Package qotd bridges the Discord-agnostic QOTD domain logic to the actual
Discord runtime environment.

It contains:
1. The RuntimeService daemon, which orchestrates scheduling intervals, sleep
mechanisms, and graceful shutdowns to execute QOTD publish and reconcile
cycles across all guilds assigned to the active shard/instance.
2. The ArikawaPublisher adapter, which implements the pure qotd.Publisher
interface using the arikawa Discord API client.

# Graceful Shutdown & Concurrency
The daemon relies on context cancellation and waitgroups to guarantee that no
in-flight API calls to Discord or Postgres are brutally terminated during
deployment, preventing "abandoned" state corruption. The timers can also
be dynamically interrupted via channels if configuration changes radically.
*/
package qotd
