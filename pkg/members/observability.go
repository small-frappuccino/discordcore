package members

import (
	"sync/atomic"
)

// Metrics is the observability seam the members service writes through.
type Metrics interface {
	RecordGuildMemberCall()
	RecordStateMemberCacheHit()
	RecordRolesCacheMemoryHit()
	RecordRolesCacheStoreHit()
	RecordRolesAuditCacheHit()
	RecordAuditLogCall()
}

// SnapshotProvider is the optional capability the /v1/health/members handler looks for.
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is the JSON payload /v1/health/members returns.
type MetricsSnapshot struct {
	GuildMemberCallsTotal int64 `json:"guild_member_calls_total"`
	StateMemberHitsTotal  int64 `json:"state_member_hits_total"`
	RolesMemoryHitsTotal  int64 `json:"roles_memory_hits_total"`
	RolesStoreHitsTotal   int64 `json:"roles_store_hits_total"`
	RolesAuditHitsTotal   int64 `json:"roles_audit_hits_total"`
	AuditLogCallsTotal    int64 `json:"audit_log_calls_total"`
}

// NopMetrics is the default implementation when the service is constructed without explicit metrics wiring.
type NopMetrics struct{}

func (NopMetrics) RecordGuildMemberCall()     {}
func (NopMetrics) RecordStateMemberCacheHit() {}
func (NopMetrics) RecordRolesCacheMemoryHit() {}
func (NopMetrics) RecordRolesCacheStoreHit()  {}
func (NopMetrics) RecordRolesAuditCacheHit()  {}
func (NopMetrics) RecordAuditLogCall()        {}

// InMemoryMetrics is the lightweight implementation backing /v1/health/members.
type InMemoryMetrics struct {
	guildMemberCalls atomic.Int64
	stateMemberHits  atomic.Int64
	rolesMemoryHits  atomic.Int64
	rolesStoreHits   atomic.Int64
	rolesAuditHits   atomic.Int64
	auditLogCalls    atomic.Int64
}

// NewInMemoryMetrics constructs the production metrics implementation.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{}
}

func (m *InMemoryMetrics) RecordGuildMemberCall()     { m.guildMemberCalls.Add(1) }
func (m *InMemoryMetrics) RecordStateMemberCacheHit() { m.stateMemberHits.Add(1) }
func (m *InMemoryMetrics) RecordRolesCacheMemoryHit() { m.rolesMemoryHits.Add(1) }
func (m *InMemoryMetrics) RecordRolesCacheStoreHit()  { m.rolesStoreHits.Add(1) }
func (m *InMemoryMetrics) RecordRolesAuditCacheHit()  { m.rolesAuditHits.Add(1) }
func (m *InMemoryMetrics) RecordAuditLogCall()        { m.auditLogCalls.Add(1) }

// Snapshot returns a JSON-friendly view of the current counter state.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		GuildMemberCallsTotal: m.guildMemberCalls.Load(),
		StateMemberHitsTotal:  m.stateMemberHits.Load(),
		RolesMemoryHitsTotal:  m.rolesMemoryHits.Load(),
		RolesStoreHitsTotal:   m.rolesStoreHits.Load(),
		RolesAuditHitsTotal:   m.rolesAuditHits.Load(),
		AuditLogCallsTotal:    m.auditLogCalls.Load(),
	}
}
