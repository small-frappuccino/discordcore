/*
Package core provides the foundational orchestration and execution environment
for Discord slash commands within the application.

It defines the canonical lifecycle boundaries, context encapsulation, and request
routing mechanisms necessary to translate raw gateway events from Arikawa into
strongly typed, handler-driven interaction flows.

This package manages the CommandRegistry which enforces deterministic registration
and syncing, and the Dispatcher which guarantees isolated execution of command
handlers. All dependencies in this layer are intentionally kept agnostic of specific
feature domains, providing a centralized integration seam for extending the bot's
functional capabilities.
*/
package core
