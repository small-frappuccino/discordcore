/*
Package partners implements the slash-command routing for partner board management.

It integrates directly with the Arikawa router to ingest administrative execution
flows, converting Discord interaction payloads into explicit domain synchronization
triggers. The commands encapsulate localized error handling, shielding the
primary event loop from state mutations originating from malformed user inputs.
*/
package partners
