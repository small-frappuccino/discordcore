import type { ControlApiClient } from "../client";

// HealthLiveSnapshot mirrors LiveHealthSnapshot served by /v1/health/live.
export interface HealthLiveSnapshot {
  status: string;
  app: string;
  app_version?: string;
  core_version: string;
  bot_user?: string;
  bot_avatar_url?: string;
  started_at: string;
  uptime_seconds: number;
}

// HealthCacheSegmentSnapshot mirrors pkg/discord/cache.SegmentSnapshot.
export interface HealthCacheSegmentSnapshot {
  entries: number;
  hits: number;
  misses: number;
  evictions: number;
  hit_rate: number;
  ttl_seconds: number;
  limit: number;
}

// HealthCachePersistedStats mirrors pkg/storage.PersistentCacheStats.
export interface HealthCachePersistedStats {
  total: number;
  by_type?: Record<string, number>;
}

// HealthCacheSnapshot mirrors pkg/discord/cache.CacheMetricsSnapshot.
export interface HealthCacheSnapshot {
  members: HealthCacheSegmentSnapshot;
  guilds: HealthCacheSegmentSnapshot;
  roles: HealthCacheSegmentSnapshot;
  channels: HealthCacheSegmentSnapshot;
  persisted: HealthCachePersistedStats;
  last_warmup: string;
}

// HealthMonitoringAPISnapshot mirrors pkg/discord/logging.APISnapshot.
export interface HealthMonitoringAPISnapshot {
  audit_log_calls_total: number;
  guild_member_calls_total: number;
  messages_sent_total: number;
}

// HealthMonitoringCacheSnapshot mirrors pkg/discord/logging.MonitoringCacheSnapshot.
export interface HealthMonitoringCacheSnapshot {
  state_member_hits_total: number;
  roles_memory_hits_total: number;
  roles_store_hits_total: number;
  roles_audit_hits_total: number;
}

// HealthMonitoringSnapshot mirrors pkg/discord/logging.MetricsSnapshot.
export interface HealthMonitoringSnapshot {
  api: HealthMonitoringAPISnapshot;
  cache: HealthMonitoringCacheSnapshot;
}

// HealthQOTDSummarySnapshot mirrors pkg/observability.SummarySnapshot.
export interface HealthQOTDSummarySnapshot {
  count: number;
  sum_seconds: number;
  max_seconds: number;
}

// HealthQOTDPublishModeSnapshot mirrors pkg/qotd.PublishModeSnapshot.
export interface HealthQOTDPublishModeSnapshot {
  success_total: number;
  failure_total: number;
  failure_by_cause?: Record<string, number>;
  duration_seconds: HealthQOTDSummarySnapshot;
}

// HealthQOTDReconcileSnapshot mirrors pkg/qotd.ReconcileSnapshot.
export interface HealthQOTDReconcileSnapshot {
  cycles_total: number;
  failures_total: number;
  duration_seconds: HealthQOTDSummarySnapshot;
}

// HealthQOTDStateSnapshot mirrors pkg/qotd.StateSnapshot.
export interface HealthQOTDStateSnapshot {
  abandoned_total: number;
  divergence_total: number;
  unmanageable_thread_total: number;
  orphan_reservations_reclaimed_total: number;
  suppressions_cleared_total: number;
}

// HealthQOTDSnapshot mirrors pkg/qotd.MetricsSnapshot.
export interface HealthQOTDSnapshot {
  publishes: Record<string, HealthQOTDPublishModeSnapshot>;
  reconcile: HealthQOTDReconcileSnapshot;
  state: HealthQOTDStateSnapshot;
}

// HealthModerationSummarySnapshot mirrors pkg/observability.SummarySnapshot.
export interface HealthModerationSummarySnapshot {
  count: number;
  sum_seconds: number;
  max_seconds: number;
}

// HealthModerationCleanSnapshot mirrors pkg/discord/commands/moderation.CleanSnapshot.
export interface HealthModerationCleanSnapshot {
  attempts_total: number;
  success_total: number;
  failure_total: number;
  failure_by_cause?: Record<string, number>;
  delete_failure_by_class?: Record<string, number>;
  deleted_messages_total: number;
  audit_log_failure_total: number;
  duration_seconds: HealthModerationSummarySnapshot;
}

// HealthModerationSnapshot mirrors pkg/discord/commands/moderation.MetricsSnapshot.
export interface HealthModerationSnapshot {
  clean: HealthModerationCleanSnapshot;
}

export type HealthResult<T> =
  | { available: true; snapshot: T }
  | { available: false; message: string };

async function requestHealth<T>(
  client: ControlApiClient,
  path: string,
): Promise<HealthResult<T>> {
  const url = client.getBaseUrl() === "" ? path : `${client.getBaseUrl()}${path}`;
  const response = await fetch(url, {
    method: "GET",
    credentials: "include",
  });
  if (response.status === 503) {
    const text = await response.text();
    return {
      available: false,
      message: text.trim() === "" ? "telemetry unavailable" : text.trim(),
    };
  }
  if (!response.ok) {
    const text = await response.text();
    throw new Error(
      `Control API GET ${path} failed: ${response.status} ${response.statusText} - ${text}`.trim(),
    );
  }
  const snapshot = (await response.json()) as T;
  return { available: true, snapshot };
}

export async function getHealthLive(client: ControlApiClient): Promise<HealthResult<HealthLiveSnapshot>> {
  return requestHealth<HealthLiveSnapshot>(client, "/v1/health/live");
}

export async function getHealthCache(client: ControlApiClient): Promise<HealthResult<HealthCacheSnapshot>> {
  return requestHealth<HealthCacheSnapshot>(client, "/v1/health/cache");
}

export async function getHealthMonitoring(client: ControlApiClient): Promise<HealthResult<HealthMonitoringSnapshot>> {
  return requestHealth<HealthMonitoringSnapshot>(client, "/v1/health/monitoring");
}

export async function getHealthQOTD(client: ControlApiClient): Promise<HealthResult<HealthQOTDSnapshot>> {
  return requestHealth<HealthQOTDSnapshot>(client, "/v1/health/qotd");
}

export async function getHealthModeration(client: ControlApiClient): Promise<HealthResult<HealthModerationSnapshot>> {
  return requestHealth<HealthModerationSnapshot>(client, "/v1/health/moderation");
}
