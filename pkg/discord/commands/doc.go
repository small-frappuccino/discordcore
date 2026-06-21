/*
Package commands provides the native, state-of-the-art Arikawa routing infrastructure for the application.

It securely manages the registration, synchronization, and atomic dispatch of Discord application commands
(slash-commands, user/message contexts, and component interactions). Built strictly upon
github.com/diamondburned/arikawa/v3, this package enforces a clean separation of concerns by completely
decoupling domain logic from the underlying gateway implementation.

The infrastructure leverages a thread-safe registry (`CommandRegistry`), an atomic router (`CommandRouter`),
and a bulk-overwrite syncer (`CommandSyncer`) to guarantee idempotency and concurrency-safe behavior across
all distributed command executions. It strictly eschews interface-based dynamic casting in favor of rigid
contract encapsulation via `ArikawaContext`.
*/
package commands
