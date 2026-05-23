import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  HealthCacheSegmentSnapshot,
  HealthCacheSnapshot,
  HealthLiveSnapshot,
  HealthModerationSnapshot,
  HealthMonitoringSnapshot,
  HealthQOTDSnapshot,
  HealthResult,
} from "../api/control";
import {
  EmptyState,
  FlatPageLayout,
  KeyValueList,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";

const DEFAULT_POLL_INTERVAL_SECONDS = 30;
const MIN_POLL_INTERVAL_SECONDS = 5;

// HealthPage is the dashboard-side replacement for the deprecated
// /admin metrics_watch Discord command. It polls /v1/health/{live,cache,
// monitoring,qotd,moderation} on an interval the operator picks, and renders
// each subsystem snapshot. Each subsystem can independently report
// "unavailable" (the route returns 503 with a body) without breaking the
// other tiles — operators see exactly which surfaces are not yet wired.
export function HealthPage() {
  const { authState, beginLogin, client } = useDashboardSession();
  const [intervalSeconds, setIntervalSeconds] = useState(
    DEFAULT_POLL_INTERVAL_SECONDS,
  );
  const [paused, setPaused] = useState(false);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<Date | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [globalError, setGlobalError] = useState<string | null>(null);
  const [live, setLive] = useState<HealthResult<HealthLiveSnapshot> | null>(
    null,
  );
  const [cache, setCache] = useState<HealthResult<HealthCacheSnapshot> | null>(
    null,
  );
  const [monitoring, setMonitoring] = useState<
    HealthResult<HealthMonitoringSnapshot> | null
  >(null);
  const [qotd, setQotd] = useState<HealthResult<HealthQOTDSnapshot> | null>(
    null,
  );
  const [moderation, setModeration] = useState<
    HealthResult<HealthModerationSnapshot> | null
  >(null);

  const refresh = useCallback(async () => {
    if (authState !== "signed_in") {
      return;
    }
    setRefreshing(true);
    try {
      const [liveResult, cacheResult, monitoringResult, qotdResult, moderationResult] =
        await Promise.all([
          client.getHealthLive(),
          client.getHealthCache(),
          client.getHealthMonitoring(),
          client.getHealthQOTD(),
          client.getHealthModeration(),
        ]);
      setLive(liveResult);
      setCache(cacheResult);
      setMonitoring(monitoringResult);
      setQotd(qotdResult);
      setModeration(moderationResult);
      setGlobalError(null);
      setLastUpdatedAt(new Date());
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : "Failed to refresh subsystem snapshots.";
      setGlobalError(message);
    } finally {
      setRefreshing(false);
    }
  }, [authState, client]);

  useEffect(() => {
    if (authState !== "signed_in") {
      return;
    }
    void refresh();
  }, [authState, refresh]);

  useEffect(() => {
    if (authState !== "signed_in" || paused) {
      return;
    }
    const safeInterval = Math.max(
      intervalSeconds,
      MIN_POLL_INTERVAL_SECONDS,
    );
    const timer = window.setInterval(() => {
      void refresh();
    }, safeInterval * 1000);
    return () => window.clearInterval(timer);
  }, [authState, paused, intervalSeconds, refresh]);

  const headerStatus = useMemo(() => {
    if (authState !== "signed_in") {
      return <StatusBadge tone="info">Sign in required</StatusBadge>;
    }
    if (globalError !== null) {
      return <StatusBadge tone="error">Refresh failed</StatusBadge>;
    }
    if (lastUpdatedAt === null) {
      return <StatusBadge tone="info">Loading…</StatusBadge>;
    }
    return (
      <StatusBadge tone="success">
        {paused ? "Paused" : `Auto-refresh every ${intervalSeconds}s`}
      </StatusBadge>
    );
  }, [authState, globalError, lastUpdatedAt, paused, intervalSeconds]);

  const headerActions = useMemo(() => {
    if (authState !== "signed_in") {
      return (
        <button
          className="button-primary"
          type="button"
          onClick={() => void beginLogin("/manage")}
        >
          Sign in with Discord
        </button>
      );
    }
    return (
      <div className="inline-actions">
        <button
          className="button-secondary"
          type="button"
          disabled={refreshing}
          onClick={() => void refresh()}
        >
          {refreshing ? "Refreshing…" : "Refresh now"}
        </button>
        <button
          className="button-secondary"
          type="button"
          onClick={() => setPaused((prev) => !prev)}
        >
          {paused ? "Resume auto-refresh" : "Pause auto-refresh"}
        </button>
      </div>
    );
  }, [authState, beginLogin, refresh, refreshing, paused]);

  if (authState !== "signed_in") {
    return (
      <section className="page-shell">
        <PageHeader
          eyebrow="Operational"
          title="Health"
          description="Live snapshots from the embedded /v1/health/* endpoints."
          status={headerStatus}
          actions={headerActions}
        />
        <FlatPageLayout
          workspaceEyebrow={null}
          workspaceTitle={null}
          workspaceDescription={null}
        >
          <EmptyState
            title="Sign in to view health snapshots"
            description="Health endpoints require the same authentication as the rest of the control plane."
          />
        </FlatPageLayout>
      </section>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Operational"
        title="Health"
        description="Live snapshots from /v1/health/{live,cache,monitoring,qotd,moderation}. Replaces the deprecated /admin metrics_watch Discord command."
        status={headerStatus}
        actions={headerActions}
      />
      <FlatPageLayout
        workspaceEyebrow={null}
        workspaceTitle={null}
        workspaceDescription={null}
        summary={
          <section
            className="overview-summary-strip"
            aria-label="Refresh control"
          >
            <MetricCard
              label="Last updated"
              value={
                lastUpdatedAt === null
                  ? "Never"
                  : lastUpdatedAt.toLocaleTimeString()
              }
              description={
                globalError === null
                  ? "Snapshots refresh in the background."
                  : globalError
              }
              tone={globalError === null ? "info" : "error"}
            />
            <MetricCard
              label="Auto-refresh"
              value={paused ? "Paused" : `${intervalSeconds}s`}
              description="Use the buttons in the header to pause or refresh now."
              tone={paused ? "neutral" : "success"}
            />
            <SurfaceCard>
              <p className="section-label">Interval</p>
              <label className="field-stack">
                <span className="field-label">Seconds between polls</span>
                <input
                  aria-label="Seconds between polls"
                  inputMode="numeric"
                  min={MIN_POLL_INTERVAL_SECONDS}
                  step={5}
                  type="number"
                  value={intervalSeconds}
                  onChange={(event) => {
                    const parsed = Number.parseInt(event.target.value, 10);
                    if (Number.isFinite(parsed)) {
                      setIntervalSeconds(
                        Math.max(MIN_POLL_INTERVAL_SECONDS, parsed),
                      );
                    }
                  }}
                />
                <span className="meta-note">
                  Minimum {MIN_POLL_INTERVAL_SECONDS} seconds. Polling pauses
                  automatically when the auto-refresh button is toggled off.
                </span>
              </label>
            </SurfaceCard>
          </section>
        }
      >
        <LiveSection result={live} />
        <CacheSection result={cache} />
        <MonitoringSection result={monitoring} />
        <QOTDSection result={qotd} />
        <ModerationSection result={moderation} />
      </FlatPageLayout>
    </section>
  );
}

function LiveSection({
  result,
}: {
  result: HealthResult<HealthLiveSnapshot> | null;
}) {
  if (result === null) {
    return <PendingSection title="Liveness" />;
  }
  if (!result.available) {
    return <UnavailableSection title="Liveness" message={result.message} />;
  }
  const snapshot = result.snapshot;
  return (
    <section className="surface-subsection">
      <div className="card-copy">
        <p className="section-label">/v1/health/live</p>
        <h2>Liveness</h2>
        <p className="section-description">
          Returned by the embedded control HTTP server itself. Reaching this
          endpoint means the process is running and serving traffic.
        </p>
      </div>
      <KeyValueList
        items={[
          { label: "Status", value: snapshot.status },
          { label: "App", value: formatAppLabel(snapshot) },
          { label: "Core version", value: snapshot.core_version },
          { label: "Bot user", value: snapshot.bot_user ?? "—" },
          { label: "Started at", value: formatTimestamp(snapshot.started_at) },
          { label: "Uptime", value: formatDuration(snapshot.uptime_seconds) },
        ]}
      />
    </section>
  );
}

function CacheSection({
  result,
}: {
  result: HealthResult<HealthCacheSnapshot> | null;
}) {
  if (result === null) {
    return <PendingSection title="Cache" />;
  }
  if (!result.available) {
    return <UnavailableSection title="Cache" message={result.message} />;
  }
  const snapshot = result.snapshot;
  const persistedTotal = snapshot.persisted.total ?? 0;
  return (
    <section className="surface-subsection">
      <div className="card-copy">
        <p className="section-label">/v1/health/cache</p>
        <h2>Cache</h2>
        <p className="section-description">
          UnifiedCache segments and persisted cache totals from the default
          bot runtime.
        </p>
      </div>
      <div className="overview-summary-strip">
        <SegmentMetricCard
          label="Members"
          snapshot={snapshot.members}
        />
        <SegmentMetricCard label="Guilds" snapshot={snapshot.guilds} />
        <SegmentMetricCard label="Roles" snapshot={snapshot.roles} />
        <SegmentMetricCard
          label="Channels"
          snapshot={snapshot.channels}
        />
      </div>
      <KeyValueList
        items={[
          {
            label: "Persisted total",
            value: persistedTotal.toLocaleString(),
          },
          {
            label: "Persisted by type",
            value: formatPersistedByType(snapshot.persisted.by_type),
          },
          {
            label: "Last warmup",
            value: formatTimestamp(snapshot.last_warmup),
          },
        ]}
      />
    </section>
  );
}

function MonitoringSection({
  result,
}: {
  result: HealthResult<HealthMonitoringSnapshot> | null;
}) {
  if (result === null) {
    return <PendingSection title="Monitoring" />;
  }
  if (!result.available) {
    return <UnavailableSection title="Monitoring" message={result.message} />;
  }
  const snapshot = result.snapshot;
  return (
    <section className="surface-subsection">
      <div className="card-copy">
        <p className="section-label">/v1/health/monitoring</p>
        <h2>Monitoring</h2>
        <p className="section-description">
          Discord API call counts the monitoring service is responsible for,
          plus the in-process cache hit counters it uses to avoid those API
          calls.
        </p>
      </div>
      <div className="overview-summary-strip">
        <MetricCard
          label="Audit log calls"
          value={snapshot.api.audit_log_calls_total.toLocaleString()}
          description="Total GuildAuditLog fetches."
          tone="info"
        />
        <MetricCard
          label="Guild member calls"
          value={snapshot.api.guild_member_calls_total.toLocaleString()}
          description="Total GuildMember API resolutions."
          tone="info"
        />
        <MetricCard
          label="Messages sent"
          value={snapshot.api.messages_sent_total.toLocaleString()}
          description="Total log embeds posted to Discord."
          tone="info"
        />
      </div>
      <KeyValueList
        items={[
          {
            label: "State member cache hits",
            value: snapshot.cache.state_member_hits_total.toLocaleString(),
          },
          {
            label: "Roles cache memory hits",
            value: snapshot.cache.roles_memory_hits_total.toLocaleString(),
          },
          {
            label: "Roles cache store hits",
            value: snapshot.cache.roles_store_hits_total.toLocaleString(),
          },
          {
            label: "Roles audit cache hits",
            value: snapshot.cache.roles_audit_hits_total.toLocaleString(),
          },
        ]}
      />
    </section>
  );
}

function QOTDSection({
  result,
}: {
  result: HealthResult<HealthQOTDSnapshot> | null;
}) {
  if (result === null) {
    return <PendingSection title="QOTD" />;
  }
  if (!result.available) {
    return <UnavailableSection title="QOTD" message={result.message} />;
  }
  const snapshot = result.snapshot;
  const publishRows = Object.entries(snapshot.publishes ?? {}).map(
    ([mode, mode_snapshot]) => ({
      label: `Publishes (${mode})`,
      value: `${mode_snapshot.success_total.toLocaleString()} ok / ${mode_snapshot.failure_total.toLocaleString()} fail`,
    }),
  );
  return (
    <section className="surface-subsection">
      <div className="card-copy">
        <p className="section-label">/v1/health/qotd</p>
        <h2>QOTD</h2>
        <p className="section-description">
          Publish, reconcile, and side-event counters from the QOTD service.
        </p>
      </div>
      <KeyValueList
        items={[
          ...publishRows,
          {
            label: "Reconcile cycles",
            value: `${snapshot.reconcile.cycles_total.toLocaleString()} (${snapshot.reconcile.failures_total.toLocaleString()} failed)`,
          },
          {
            label: "Abandoned posts",
            value: snapshot.state.abandoned_total.toLocaleString(),
          },
          {
            label: "State divergences",
            value: snapshot.state.divergence_total.toLocaleString(),
          },
          {
            label: "Unmanageable threads",
            value: snapshot.state.unmanageable_thread_total.toLocaleString(),
          },
          {
            label: "Orphan reservations reclaimed",
            value: snapshot.state.orphan_reservations_reclaimed_total.toLocaleString(),
          },
          {
            label: "Suppressions cleared",
            value: snapshot.state.suppressions_cleared_total.toLocaleString(),
          },
        ]}
      />
    </section>
  );
}

function ModerationSection({
  result,
}: {
  result: HealthResult<HealthModerationSnapshot> | null;
}) {
  if (result === null) {
    return <PendingSection title="Moderation" />;
  }
  if (!result.available) {
    return <UnavailableSection title="Moderation" message={result.message} />;
  }
  const snapshot = result.snapshot.clean;
  return (
    <section className="surface-subsection">
      <div className="card-copy">
        <p className="section-label">/v1/health/moderation</p>
        <h2>Moderation</h2>
        <p className="section-description">
          /clean command counters: attempts, outcomes, deleted messages, and
          per-cause failure breakdown.
        </p>
      </div>
      <div className="overview-summary-strip">
        <MetricCard
          label="Attempts"
          value={snapshot.attempts_total.toLocaleString()}
          description="Total /clean invocations."
          tone="info"
        />
        <MetricCard
          label="Success"
          value={snapshot.success_total.toLocaleString()}
          description={`${snapshot.deleted_messages_total.toLocaleString()} messages removed.`}
          tone="success"
        />
        <MetricCard
          label="Failure"
          value={snapshot.failure_total.toLocaleString()}
          description="Includes refused and aborted runs."
          tone={snapshot.failure_total > 0 ? "error" : "neutral"}
        />
      </div>
      <KeyValueList
        items={[
          {
            label: "Audit log failures",
            value: snapshot.audit_log_failure_total.toLocaleString(),
          },
          {
            label: "Failure causes",
            value: formatCountMap(snapshot.failure_by_cause),
          },
          {
            label: "Delete failure classes",
            value: formatCountMap(snapshot.delete_failure_by_class),
          },
        ]}
      />
    </section>
  );
}

function SegmentMetricCard({
  label,
  snapshot,
}: {
  label: string;
  snapshot: HealthCacheSegmentSnapshot;
}) {
  const hitRatePct = (snapshot.hit_rate * 100).toFixed(1);
  return (
    <MetricCard
      label={label}
      value={snapshot.entries.toLocaleString()}
      description={`${snapshot.hits.toLocaleString()} hits / ${snapshot.misses.toLocaleString()} misses (${hitRatePct}%)`}
      tone="info"
    />
  );
}

function PendingSection({ title }: { title: string }) {
  return (
    <section className="surface-subsection">
      <div className="card-copy">
        <p className="section-label">Loading</p>
        <h2>{title}</h2>
        <p className="section-description">
          Waiting for the first snapshot to land.
        </p>
      </div>
    </section>
  );
}

function UnavailableSection({
  title,
  message,
}: {
  title: string;
  message: string;
}) {
  return (
    <section className="surface-subsection">
      <div className="card-copy">
        <p className="section-label">Unavailable</p>
        <h2>{title}</h2>
        <p className="section-description">{message}</p>
      </div>
    </section>
  );
}

function formatAppLabel(snapshot: HealthLiveSnapshot) {
  const version = snapshot.app_version?.trim() ?? "";
  if (version === "") {
    return snapshot.app;
  }
  return `${snapshot.app} ${version}`;
}

function formatTimestamp(value: string) {
  const trimmed = value.trim();
  if (trimmed === "" || trimmed === "0001-01-01T00:00:00Z") {
    return "—";
  }
  const parsed = new Date(trimmed);
  if (Number.isNaN(parsed.getTime())) {
    return trimmed;
  }
  return parsed.toLocaleString();
}

function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "0s";
  }
  const days = Math.floor(seconds / 86_400);
  const hours = Math.floor((seconds % 86_400) / 3_600);
  const minutes = Math.floor((seconds % 3_600) / 60);
  const secs = Math.floor(seconds % 60);
  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  if (parts.length === 0 || secs > 0) parts.push(`${secs}s`);
  return parts.join(" ");
}

function formatPersistedByType(byType: Record<string, number> | undefined) {
  if (byType === undefined) {
    return "—";
  }
  const keys = Object.keys(byType).sort();
  if (keys.length === 0) {
    return "—";
  }
  return keys
    .map((key) => `${key}=${byType[key]?.toLocaleString() ?? 0}`)
    .join(", ");
}

function formatCountMap(map: Record<string, number> | undefined) {
  if (map === undefined) {
    return "—";
  }
  const entries = Object.entries(map).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) {
    return "—";
  }
  return entries
    .map(([key, value]) => `${key}=${value.toLocaleString()}`)
    .join(", ");
}
