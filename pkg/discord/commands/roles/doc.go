/*
Package roles implements the slash-command routing and interaction handling
for role panel workflows.

It integrates directly with the Arikawa router to execute configuration mutations
and process component interactions (e.g., button clicks). The command structure
encapsulates payload validation and localizes structural errors to prevent
malformed inputs from compromising the primary event loop.
*/
package roles
