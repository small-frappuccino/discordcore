

1) Service Architecture & Dependencies
- Status: Done (with a couple of small loose ends)
- What’s implemented:
  - Central service manager with lifecycle, dependency ordering, health checks, restart policy, and unified error handling hooks:
    - `internal/service/manager.go` implements `ServiceManager` with StartAll/StopAll, dependency graph/topological sort, periodic health checks, restart, info, etc.
    - `internal/service/base.go` provides `BaseService`, `ServiceWrapper` for wrapping legacy services, and `ManagedService` for auto-restart usage.
  - Main uses the ServiceManager to register and start services:
    - `cmd/discordcore/main.go` constructs `ServiceManager`, wraps Monitoring and Automod as services, registers them, and invokes `StartAll()` and `StopAll()`.
  - Admin commands for service management (status/list/restart/health) are wired:
    - `internal/discord/commands/admin/service_commands.go` provides `/admin service status`, `/admin service list`, `/admin service restart`, `/admin health`, etc.
  - Unified error handling/logging exists and is integrated into `ServiceManager` retries:
    - `internal/errors/handler.go` contains a centralized error handler with retry/circuit-breaker-ish logic. `ServiceManager` calls `errorHandler.HandleWithRetry` on startup paths.
- What remains:
  - Unused dependencies in `CommandHandler`: it still accepts `monitoringService` and `automodService` but doesn’t use them:
    - `internal/discord/commands/handler.go`: the fields exist and getters exist, but command setup doesn’t actually use them. These parameters can be removed or actually used.
  - There’s no “container” DI framework; but a solid ServiceManager is in place, which was the core of the TODO’s recommendation.

2) Event Handler Conflicts
- Status: Missing
- What’s implemented:
  - Multiple services independently register handlers:
    - `internal/discord/logging/monitoring.go`: `setupEventHandlers()` registers presence/member/user/guild handlers.
    - `internal/discord/logging/member_events.go`: `Start()` registers member add/remove.
    - `internal/discord/logging/message_events.go`: `Start()` registers message create/update/delete.
    - `internal/discord/logging/automod.go`: `Start()` registers AutoModerationAction handler.
- What remains:
  - No centralized event router exists (no `EventRouter` abstraction).
  - Potential for conflicting or duplicated handlers remains; nothing prevents double-registration or introduces routing and ordering semantics.

3) Configuration Channel Management
- Status: Missing
- What’s implemented:
  - Channel IDs are stored in `files.GuildConfig`:
    - `internal/files/types.go` includes `UserLogChannelID`, `MessageLogChannelID`, `AutomodLogChannelID`, `CommandChannelID`.
  - Fallback/usage patterns:
    - Automod logs fall back to `CommandChannelID` when `AutomodLogChannelID` is empty:
      - `internal/discord/logging/automod.go`.
    - Avatar/membership/message logs rely on dedicated channel fields with no fallback to command channel:
      - `internal/discord/logging/monitoring.go` (avatar/user logs),
      - `internal/discord/logging/member_events.go` (user join/leave),
      - `internal/discord/logging/message_events.go` (edit/delete).
- What remains:
  - There’s no pre-validation of channels at configuration time.
  - No permission checks for target channels before attempting to send.
  - Fallback behavior is inconsistent by service, as noted in the TODO.
  - No `ConfigValidator` exists.

4) Inconsistent Cache Management
- Status: Partial
- What’s implemented:
  - A unified cache interface exists:
    - `internal/cache/interface.go` defines `CacheManager`, `TTLCache`, `PersistentCache`, `GuildCache`, `CacheStats`, etc.
  - Avatar cache persists to disk:
    - `internal/files/cache.go` provides `AvatarCacheManager` persisting per guild to a unified cache file.
  - Message event service uses an in-memory map with its own cleanup ticker (1h) and 24h TTL behavior:
    - `internal/discord/logging/message_events.go`.
  - Admin command for showing cache stats exists but isn’t wired:
    - `internal/discord/commands/admin/service_commands.go` has `/admin cache`, but in main the `cacheManager` passed is nil.
- What remains:
  - `AvatarCacheManager` does not implement `internal/cache.CacheManager` (different package and shape).
  - The message cache is not unified with the avatar cache nor exposed via the `CacheManager` interface.
  - Admin cache stats currently do nothing because `main.go` calls `admin.NewAdminCommands(serviceManager, nil)`.
  - No unified cache stats across caches and no centralized cleanup/metrics.

5) Unimplemented TODOs with Service Impact
- Status: Missing
- Confirmed still TODO:
  - Member join time persistence for accurate “time in server”:
    - `internal/discord/logging/member_events.go`: `calculateServerTime` returns 0 with a TODO to persist join data.
  - Audit log integration to detect who deleted messages:
    - `internal/discord/logging/message_events.go`: TODO to check audit logs; currently always labels as deleted by “Usuário”.

6) Notification System Fragmentation
- Status: Partial
- What’s implemented:
  - `NotificationSender` is used for avatar/member/message notifications:
    - `internal/discord/logging/notifications.go`.
  - A single `NotificationSender` instance is created in `MonitoringService` and passed to `MemberEventService` and `MessageEventService`:
    - `internal/discord/logging/monitoring.go`.
- What remains:
  - `AutomodService` sends embeds directly via the session, not through `NotificationSender`:
    - `internal/discord/logging/automod.go` builds and sends embeds itself.
  - There’s no centralized notification templating/routing beyond the `NotificationSender` methods, and it’s not instantiated/managed by a central service manager. If you add more services later, you could accidentally create additional instances elsewhere.

Recommended Integration Improvements vs Current State
- Service Manager: Already implemented and in use. You can mark this as done.
- Centralized Event Router: Not implemented; still recommended.
- Unified Cache Interface: Interface exists (done), but you should:
  - Provide a concrete implementation(s) used by all caches,
  - Migrate `AvatarCacheManager` and `MessageEventService` cache to the interface, and
  - Wire a cache manager into `AdminCommands` to make `/admin cache` useful.
- Configuration Validator: Not implemented; still recommended (validate channel existence and permissions up front).
- Service Integration Commands: Already implemented (status/list/restart/health/cache/system info). Only missing piece is wiring a real cache manager instance for `/admin cache`.

Concrete next steps I’d recommend
- Quick wins:
  - Remove unused constructor parameters from `commands.NewCommandHandler` (or actually use them). This reduces coupling immediately.
  - Pass a real `cache.CacheManager` into `admin.NewAdminCommands(...)` instead of `nil`, and expose stats for at least avatar cache to make `/admin cache` functional.
  - Refactor `AutomodService` to use `NotificationSender` for consistency.
- Medium:
  - Introduce an `EventRouter` in a central place. Services register typed handlers with the router which binds exactly one Discord handler per event type and fans out internally. This removes registration duplication and allows ordering, filtering, and metrics.
  - Add a `ConfigValidator` pass at startup and on config changes to verify channel IDs exist, bot permissions are adequate, and apply consistent fallback policy.
- Longer-term:
  - Implement message deletion audit-log correlation (Discord audit logs require specific permissions and rate limiting considerations).
  - Persist member join timestamps (or fetchable approximation) to compute server time on leave events.
  - Consolidate caching: provide a `CacheManager` implementation(s) that both avatar and message caches can use; surface `Stats()` and `Cleanup()` via `/admin cache`.

Summary: What in the TODO is already solved
- Implement the Service Manager pattern to coordinate service lifecycle: Done.
- Create unified error handling and logging strategy: Done (core pieces exist and are integrated).
- Add service management admin commands: Done.
- Unified cache interface: Interface exists (partial; not adopted).
- Centralized event routing: Missing.
- Configuration validation: Missing.
- Audit log integration for moderation features: Missing.
- Member join time persistence: Missing.
- Notification handling centralized and consistent: Partial (Automod bypasses `NotificationSender`).
