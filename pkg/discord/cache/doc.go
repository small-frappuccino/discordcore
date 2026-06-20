/*
Package cache provides an in-memory, thread-safe, sharded weak reference cache for Discord entities.

It mitigates read-heavy loads against the Discord API and the local database by retaining
transient entities (e.g., Guilds, Members) while allowing deterministic garbage collection
when memory pressure dictates or TTL expires. This package relies heavily on weak pointers
to ensure it does not artificially extend the lifecycle of cached structs.
*/
package cache
