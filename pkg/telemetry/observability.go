package telemetry

import (
	"sync/atomic"
)

// Metrics is the narrow observability seam the monitoring service writes
// through. The interface intentionally hides whether the counters are
// in-memory, shipped to Prometheus, or thrown away — recording code stays
// the same in all three worlds. NopMetrics is the default so call-sites can
// invoke m.RecordX(...) without nil checks; the in-memory implementation
// (NewInMemoryMetrics) is what /v1/health/monitoring reads from.
//
// Method shape is "Record<event>()" not "GetCounter('name').Inc()" because a
// typed surface catches event-naming drift at compile time. Adding a new
// counter is one method on this interface plus the corresponding field on
// the snapshot; ad-hoc string keys are not supported on purpose.
// APIMetrics tracks Discord-side API call counts the monitoring service
// is responsible for.
type APIMetrics interface {
	// RecordAuditLogCall is called each time monitoring fetches the
	// guild audit log via Discord API. Operators read this as the rate
	// of audit-log polls; sudden spikes correlate with elevated 429
	// pressure on the Discord-side audit endpoint.
	RecordAuditLogCall()

	// RecordGuildMemberCall is called each time monitoring fetches a
	// member via Discord API (Guild Member endpoint). Distinguishes
	// cache-driven member resolution from API-driven member resolution
	// in the API budget breakdown.
	RecordGuildMemberCall()

	// RecordMessageSent is called each time monitoring posts a message
	// to a Discord channel (log embed, notification). Headline number
	// for "bot is talking to Discord" rate.
	RecordMessageSent()
}

// MonitoringCacheMetrics tracks the in-process caches monitoring uses to
// avoid Discord API calls.
type MonitoringCacheMetrics interface {
	// RecordStateMemberCacheHit is called when monitoring resolves a
	// member via the discordgo.State cache instead of going to the API.
	// Inverse of RecordGuildMemberCall over the same code path; the
	// ratio between the two is the member resolution cache efficiency.
	RecordStateMemberCacheHit()

	// RecordRolesCacheMemoryHit is called when monitoring resolves
	// previous member roles via the in-memory rolesCache before
	// falling through to the persistent store.
	RecordRolesCacheMemoryHit()

	// RecordRolesCacheStoreHit is called when the in-memory rolesCache
	// missed and monitoring resolved the prior roles from the
	// Postgres-backed member-roles snapshot table instead.
	RecordRolesCacheStoreHit()

	// RecordRolesAuditCacheHit is called when a second guild-scoped
	// member update lands within the role-audit cache TTL and reuses
	// the audit-log fetch instead of re-hitting the Discord API. The
	// rate is a proxy for how effective the per-guild audit dedup is.
	RecordRolesAuditCacheHit()
}

// Metrics is the union of all observability seams the monitoring service writes
// through.
type Metrics interface {
	APIMetrics
	MonitoringCacheMetrics
}

// SnapshotProvider is the optional capability the /v1/health/monitoring
// handler looks for. The in-memory implementation satisfies it; the noop
// does not (it has nothing to snapshot). Routes use a type assertion so the
// metrics dependency stays write-only on the hot path.
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is the JSON payload /v1/health/monitoring returns. The
// outer struct exists so future monitoring counters can add their own
// snapshot sub-types without breaking the top-level shape.
type MetricsSnapshot struct {
	API   APISnapshot             `json:"api"`
	Cache MonitoringCacheSnapshot `json:"cache"`
}

// APISnapshot tracks Discord-side API call counts the monitoring service
// is responsible for. Operators correlate these against Discord rate-limit
// telemetry to detect runaway loops or under-cached lookup paths.
type APISnapshot struct {
	AuditLogCallsTotal    int64 `json:"audit_log_calls_total"`
	GuildMemberCallsTotal int64 `json:"guild_member_calls_total"`
	MessagesSentTotal     int64 `json:"messages_sent_total"`
}

// MonitoringCacheSnapshot tracks the in-process caches monitoring uses to
// avoid Discord API calls. Distinct from cache.CacheMetricsSnapshot (which
// covers the UnifiedCache segments); these counters belong to monitoring's
// own bookkeeping caches (rolesCache, roleUpdateAuditCache, the State hit
// path) and would not naturally fit on the UnifiedCache snapshot.
type MonitoringCacheSnapshot struct {
	StateMemberHitsTotal int64 `json:"state_member_hits_total"`
	RolesMemoryHitsTotal int64 `json:"roles_memory_hits_total"`
	RolesStoreHitsTotal  int64 `json:"roles_store_hits_total"`
	RolesAuditHitsTotal  int64 `json:"roles_audit_hits_total"`
}

// NopMetrics is the default implementation when the monitoring service is
// constructed without explicit metrics wiring. Every method is a no-op;
// this lets library code call ms.observability().RecordX(...) without nil
// checks.
type NopMetrics struct{}

// RecordAuditLogCall records audit log call.
func (NopMetrics) RecordAuditLogCall() {}

// RecordGuildMemberCall records guild member call.
func (NopMetrics) RecordGuildMemberCall() {}

// RecordMessageSent records message sent.
func (NopMetrics) RecordMessageSent() {}

// RecordStateMemberCacheHit records state member cache hit.
func (NopMetrics) RecordStateMemberCacheHit() {}

// RecordRolesCacheMemoryHit records roles cache memory hit.
func (NopMetrics) RecordRolesCacheMemoryHit() {}

// RecordRolesCacheStoreHit records roles cache store hit.
func (NopMetrics) RecordRolesCacheStoreHit() {}

// RecordRolesAuditCacheHit records roles audit cache hit.
func (NopMetrics) RecordRolesAuditCacheHit() {}

// InMemoryMetrics is the lightweight implementation backing
// /v1/health/monitoring. All counters are atomic int64; Snapshot performs
// one atomic load per field and returns a JSON-friendly copy.
//
// Goroutine safety: every method is safe to call concurrently.
type InMemoryMetrics struct {
	auditLogCalls    atomic.Int64
	guildMemberCalls atomic.Int64
	messagesSent     atomic.Int64

	stateMemberHits atomic.Int64
	rolesMemoryHits atomic.Int64
	rolesStoreHits  atomic.Int64
	rolesAuditHits  atomic.Int64
}

// RecordAuditLogCall records audit log call.
func (m *InMemoryMetrics) RecordAuditLogCall() { m.auditLogCalls.Add(1) }

// RecordGuildMemberCall records guild member call.
func (m *InMemoryMetrics) RecordGuildMemberCall() { m.guildMemberCalls.Add(1) }

// RecordMessageSent records message sent.
func (m *InMemoryMetrics) RecordMessageSent() { m.messagesSent.Add(1) }

// RecordStateMemberCacheHit records state member cache hit.
func (m *InMemoryMetrics) RecordStateMemberCacheHit() { m.stateMemberHits.Add(1) }

// RecordRolesCacheMemoryHit records roles cache memory hit.
func (m *InMemoryMetrics) RecordRolesCacheMemoryHit() { m.rolesMemoryHits.Add(1) }

// RecordRolesCacheStoreHit records roles cache store hit.
func (m *InMemoryMetrics) RecordRolesCacheStoreHit() { m.rolesStoreHits.Add(1) }

// RecordRolesAuditCacheHit records roles audit cache hit.
func (m *InMemoryMetrics) RecordRolesAuditCacheHit() { m.rolesAuditHits.Add(1) }

// Snapshot returns a JSON-friendly view of the current counter state. The
// returned MetricsSnapshot is a value copy; callers can mutate it without
// affecting the live counters.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		API: APISnapshot{
			AuditLogCallsTotal:    m.auditLogCalls.Load(),
			GuildMemberCallsTotal: m.guildMemberCalls.Load(),
			MessagesSentTotal:     m.messagesSent.Load(),
		},
		Cache: MonitoringCacheSnapshot{
			StateMemberHitsTotal: m.stateMemberHits.Load(),
			RolesMemoryHitsTotal: m.rolesMemoryHits.Load(),
			RolesStoreHitsTotal:  m.rolesStoreHits.Load(),
			RolesAuditHitsTotal:  m.rolesAuditHits.Load(),
		},
	}
}
