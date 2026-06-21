/*
Package qotd implements the Question of the Day domain logic and state machine.

This package provides the core business logic, scheduling computations, and
state management for the QOTD feature. It operates completely independently of
any Discord API specifics, delegating side-effects via the Publisher interface.

# Actor Model
To prevent race conditions, particularly around publishing and state mutations
across concurrent requests or background scheduled triggers, all state-changing
operations for a given guild are serialized using an actor model.

# State Machine
Questions transition from Draft -> Ready -> Reserved -> Used.
Official Posts transition from Provisioning -> Current -> Previous -> Archiving -> Archived.
Answers transition from Provisioning -> Active -> Archiving -> Archived.
*/
package qotd
