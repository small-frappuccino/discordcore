import { useState } from "react";
import { useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  AlertBanner,
  EmptyState,
  KeyValueList,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  getFeatureAreaDefinition,
  getFeatureAreaRecords,
} from "../features/features/areas";
import {
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import {
  canEditStatsSettings,
  formatStatsChannelAudience,
  formatStatsChannelCountValue,
  formatStatsChannelLabel,
  formatStatsChannelTemplate,
  formatStatsChannelValue,
  formatStatsConfigValue,
  formatStatsIntervalValue,
  getStatsFeatureDetails,
  summarizeStatsSignal,
} from "../features/features/stats";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";

export function StatsPage() {
  const definition = getFeatureAreaDefinition("stats");
  const location = useLocation();
  const { authState, beginLogin, currentOriginLabel, selectedGuild } =
    useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const channelOptions = useGuildChannelOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [configEnabledDraft, setConfigEnabledDraft] = useState("enabled");
  const [updateIntervalDraft, setUpdateIntervalDraft] = useState("30");

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "stats");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const workspaceNotice =
    mutation.notice ?? channelOptions.notice ?? workspace.notice;
  const statsFeature =
    areaFeatures.find((feature) => feature.id === "stats_channels") ?? null;
  const statsDetails =
    statsFeature === null ? null : getStatsFeatureDetails(statsFeature);
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;
  const enabledModules = areaFeatures.filter(
    (feature) => feature.effective_enabled,
  ).length;
  const parsedUpdateIntervalMins = Number.parseInt(
    updateIntervalDraft.trim(),
    10,
  );
  const canSaveStatsSettings =
    Number.isFinite(parsedUpdateIntervalMins) && parsedUpdateIntervalMins > 0;

  async function handleRefreshStats() {
    await Promise.all([workspace.refresh(), channelOptions.refresh()]);
  }

  async function handleSetFeatureEnabled(
    feature: FeatureRecord,
    enabled: boolean,
  ) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        enabled,
      });
      if (updated !== null) {
        await workspace.refresh();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleUseDefault(feature: FeatureRecord) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        enabled: null,
      });
      if (updated !== null) {
        await workspace.refresh();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  function openDrawer(feature: FeatureRecord) {
    const details = getStatsFeatureDetails(feature);
    setConfigEnabledDraft(details.configEnabled ? "enabled" : "disabled");
    setUpdateIntervalDraft(String(details.updateIntervalMins));
    setDrawerOpen(true);
    mutation.clearNotice();
  }

  function closeDrawer() {
    setDrawerOpen(false);
    setConfigEnabledDraft("enabled");
    setUpdateIntervalDraft("30");
    mutation.clearNotice();
  }

  async function handleSaveStatsSettings() {
    if (statsFeature === null || !canSaveStatsSettings) {
      return;
    }

    setPendingFeatureId(statsFeature.id);

    try {
      const updated = await mutation.patchFeature(statsFeature.id, {
        config_enabled: configEnabledDraft === "enabled",
        update_interval_mins: parsedUpdateIntervalMins,
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  function renderHeaderActions() {
    if (authState !== "signed_in") {
      return (
        <button
          className="button-primary"
          type="button"
          onClick={() => void beginLogin(nextPath)}
        >
          Sign in with Discord
        </button>
      );
    }

    if (selectedGuild === null) {
      return null;
    }

    return (
      <button
        className="button-secondary"
        type="button"
        disabled={
          workspace.loading || mutation.saving || channelOptions.loading
        }
        onClick={() => void handleRefreshStats()}
      >
        Refresh stats
      </button>
    );
  }

  function renderPageState() {
    if (workspace.workspaceState !== "ready") {
      return (
        <EmptyState
          title={formatWorkspaceStateTitle(areaLabel, workspace.workspaceState)}
          description={formatWorkspaceStateDescription(
            areaLabel,
            workspace.workspaceState,
          )}
          action={
            authState !== "signed_in" ? (
              <button
                className="button-primary"
                type="button"
                onClick={() => void beginLogin(nextPath)}
              >
                Sign in with Discord
              </button>
            ) : workspace.workspaceState === "unavailable" ? (
              <button
                className="button-secondary"
                type="button"
                onClick={() => void handleRefreshStats()}
              >
                Retry loading
              </button>
            ) : undefined
          }
        />
      );
    }

    if (statsFeature === null || statsDetails === null) {
      return (
        <div className="table-empty-state table-empty-state-compact">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>No stats controls yet</h2>
            <p className="section-description">
              The selected server does not expose the stats channel feature
              record required by this workspace yet.
            </p>
          </div>
        </div>
      );
    }

    return (
      <>
        <section className="surface-subsection">
          <div className="card-copy">
            <p className="section-label">Primary workflow</p>
            <h2>Stats updates</h2>
            <p className="section-description">
              Keep the stats module enabled, choose whether updates should run,
              and confirm the current schedule before reviewing the configured
              channel list.
            </p>
          </div>

          <KeyValueList
            items={[
              {
                label: "Module state",
                value: statsFeature.effective_enabled ? "On" : "Off",
              },
              {
                label: "Update rule",
                value: formatStatsConfigValue(statsDetails.configEnabled),
              },
              {
                label: "Update interval",
                value: formatStatsIntervalValue(
                  statsDetails.updateIntervalMins,
                ),
              },
              {
                label: "Configured channels",
                value: formatStatsChannelCountValue(
                  statsDetails.configuredChannelCount,
                ),
              },
              {
                label: "Current signal",
                value: summarizeStatsSignal(statsFeature),
              },
            ]}
          />

          <div className="feature-row-actions">
            <button
              className="button-primary"
              type="button"
              disabled={!canEditStatsSettings(statsFeature)}
              onClick={() => openDrawer(statsFeature)}
            >
              Configure stats schedule
            </button>
            <button
              className="button-secondary"
              type="button"
              disabled={mutation.saving}
              onClick={() =>
                void handleSetFeatureEnabled(
                  statsFeature,
                  !statsFeature.effective_enabled,
                )
              }
            >
              {mutation.saving && pendingFeatureId === statsFeature.id
                ? "Saving..."
                : statsFeature.effective_enabled
                  ? "Disable stats module"
                  : "Enable stats module"}
            </button>
            {statsFeature.override_state !== "inherit" ? (
              <button
                className="button-ghost"
                type="button"
                disabled={mutation.saving}
                onClick={() => void handleUseDefault(statsFeature)}
              >
                Use default
              </button>
            ) : null}
          </div>
        </section>

        {statsFeature.readiness === "blocked" ? (
          <section className="surface-subsection">
            <p className="section-label">Needs setup</p>
            <strong>{summarizeStatsSignal(statsFeature)}</strong>
            <p className="meta-note">
              Use the schedule editor to enable updates or review whether the
              selected server still has at least one configured stats channel.
            </p>
          </section>
        ) : null}

        {channelOptions.notice ? (
          <section className="surface-subsection">
            <p className="section-label">Channel references unavailable</p>
            <p className="meta-note">{channelOptions.notice.message}</p>
            <div className="sidebar-actions">
              <button
                className="button-secondary"
                type="button"
                disabled={channelOptions.loading}
                onClick={() => void channelOptions.refresh()}
              >
                Retry channel lookup
              </button>
            </div>
          </section>
        ) : null}

        <section className="surface-subsection">
          <div className="card-copy">
            <p className="section-label">Configured channels</p>
            <h2>Stats channel inventory</h2>
            <p className="section-description">
              Review the current server channel list that receives periodic
              member-count renames.
            </p>
          </div>

          {statsDetails.channels.length === 0 ? (
            <div className="table-empty-state table-empty-state-compact">
              <div className="card-copy">
                <p className="section-label">Inventory</p>
                <h3>No stats channels configured</h3>
                <p className="section-description">
                  This server does not currently expose any stats channel
                  targets in its configuration.
                </p>
              </div>
            </div>
          ) : (
            <div className="table-wrap">
              <table className="data-table feature-table">
                <thead>
                  <tr>
                    <th scope="col">Channel</th>
                    <th scope="col">Label</th>
                    <th scope="col">Audience</th>
                    <th scope="col">Name format</th>
                  </tr>
                </thead>
                <tbody>
                  {statsDetails.channels.map((channel) => (
                    <tr key={`${channel.channelId}:${channel.label}`}>
                      <td>
                        <div className="feature-table-copy">
                          <strong>
                            {formatStatsChannelValue(
                              channel,
                              channelOptions.channels,
                            )}
                          </strong>
                        </div>
                      </td>
                      <td>{formatStatsChannelLabel(channel)}</td>
                      <td>{formatStatsChannelAudience(channel)}</td>
                      <td>{formatStatsChannelTemplate(channel)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </>
    );
  }

  return (
    <>
      <section className="page-shell">
        <PageHeader
          eyebrow="Feature area"
          title={areaLabel}
          description="Manage the stats module schedule and review the configured stats channels for the selected server without falling back to the generic feature list."
          status={
            <StatusBadge
              tone={
                workspace.workspaceState === "ready" ? areaSummary.tone : "info"
              }
            >
              {workspace.workspaceState === "ready"
                ? areaSummary.label
                : formatWorkspaceStateTitle(
                    areaLabel,
                    workspace.workspaceState,
                  )}
            </StatusBadge>
          }
          meta={
            <>
              <span className="meta-pill subtle-pill">
                {selectedServerLabel}
              </span>
              <span className="meta-pill subtle-pill">
                {currentOriginLabel}
              </span>
            </>
          }
          actions={renderHeaderActions()}
        />

        {workspace.workspaceState === "ready" &&
        statsFeature !== null &&
        statsDetails !== null ? (
          <section
            className="overview-summary-strip"
            aria-label="Stats summary"
          >
            <MetricCard
              label="Stats module"
              value={formatFeatureStatusLabel(statsFeature)}
              description={summarizeStatsSignal(statsFeature)}
              tone={getFeatureStatusTone(statsFeature)}
            />
            <MetricCard
              label="Update rule"
              value={formatStatsConfigValue(statsDetails.configEnabled)}
              description={
                statsDetails.configEnabled
                  ? `Runs every ${formatStatsIntervalValue(statsDetails.updateIntervalMins)}.`
                  : "Updates are paused until the server rule is enabled."
              }
              tone={statsDetails.configEnabled ? "info" : "neutral"}
            />
            <MetricCard
              label="Stats channels"
              value={formatStatsChannelCountValue(
                statsDetails.configuredChannelCount,
              )}
              description="Configured channels currently reviewed in this workspace."
              tone={
                statsDetails.configuredChannelCount > 0 ? "info" : "neutral"
              }
            />
            <MetricCard
              label="Overrides"
              value={String(localOverrides)}
              description={`${enabledModules}/${areaFeatures.length} stats modules enabled for this server.`}
            />
          </section>
        ) : null}

        <section className="content-grid content-grid-with-aside">
          <div className="page-main">
            <SurfaceCard className="feature-category-panel">
              <div className="workspace-view">
                <div className="workspace-view-header">
                  <div className="card-copy">
                    <p className="section-label">Workspace</p>
                    <h2>Stats configuration</h2>
                    <p className="section-description">
                      Keep the default workspace focused on the schedule admins
                      actually need to verify here: whether stats updates should
                      run, how often they run, and which channels the server
                      will rename.
                    </p>
                  </div>
                  <div className="workspace-view-meta">
                    {workspace.workspaceState === "ready" ? (
                      <>
                        <span className="meta-pill subtle-pill">
                          {localOverrides} local overrides
                        </span>
                        <span className="meta-pill subtle-pill">
                          {enabledModules}/{areaFeatures.length} enabled
                        </span>
                      </>
                    ) : null}
                  </div>
                </div>

                <AlertBanner
                  notice={workspaceNotice}
                  busyLabel={
                    mutation.saving
                      ? "Saving stats settings..."
                      : workspace.loading || channelOptions.loading
                        ? "Refreshing stats workspace..."
                        : undefined
                  }
                />

                {renderPageState()}
              </div>
            </SurfaceCard>
          </div>

          <aside className="page-aside">
            <SurfaceCard>
              <div className="card-copy">
                <p className="section-label">Summary</p>
                <h2>Current stats setup</h2>
                <p className="section-description">
                  Use this panel to confirm the selected server, the active
                  schedule, and the current signal reported by the control
                  server.
                </p>
              </div>

              <KeyValueList
                items={[
                  {
                    label: "Server",
                    value: selectedServerLabel,
                  },
                  {
                    label: "Module state",
                    value:
                      statsFeature === null
                        ? "Not available"
                        : statsFeature.effective_enabled
                          ? "On"
                          : "Off",
                  },
                  {
                    label: "Update interval",
                    value:
                      statsDetails === null
                        ? "Not available"
                        : formatStatsIntervalValue(
                            statsDetails.updateIntervalMins,
                          ),
                  },
                  {
                    label: "Configured channels",
                    value:
                      statsDetails === null
                        ? "Not available"
                        : formatStatsChannelCountValue(
                            statsDetails.configuredChannelCount,
                          ),
                  },
                  {
                    label: "Current signal",
                    value:
                      statsFeature === null
                        ? areaSummary.signal
                        : summarizeStatsSignal(statsFeature),
                  },
                ]}
              />
            </SurfaceCard>

            <SurfaceCard>
              <div className="card-copy">
                <p className="section-label">Guidance</p>
                <h2>How this page works</h2>
                <p className="section-description">
                  Keep the main workspace centered on the stats schedule and the
                  current channel inventory instead of a generic feature row.
                </p>
              </div>

              <ul className="feature-guidance-list">
                <li>
                  Use the module toggle when stats renames should stop or resume
                  for the selected server.
                </li>
                <li>
                  Use the schedule editor to pause updates or change how often
                  the configured channels refresh.
                </li>
                <li>
                  Review the channel inventory here before changing the
                  underlying stats channel definitions elsewhere.
                </li>
              </ul>
            </SurfaceCard>
          </aside>
        </section>
      </section>

      {drawerOpen && statsFeature !== null ? (
        <div
          className="drawer-backdrop"
          onClick={closeDrawer}
          role="presentation"
        >
          <aside
            aria-label={getDrawerLabel(statsFeature)}
            aria-modal="true"
            className="drawer-panel commands-drawer"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="card-copy">
              <p className="section-label">Stats</p>
              <div className="logging-drawer-title-row">
                <h2>{statsFeature.label}</h2>
                <StatusBadge tone={getFeatureStatusTone(statsFeature)}>
                  {formatFeatureStatusLabel(statsFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">{statsFeature.description}</p>
            </div>

            {mutation.notice ? <AlertBanner notice={mutation.notice} /> : null}

            <KeyValueList
              items={[
                {
                  label: "Module state",
                  value: statsFeature.effective_enabled ? "On" : "Off",
                },
                {
                  label: "Current signal",
                  value: summarizeStatsSignal(statsFeature),
                },
                {
                  label: "Configured channels",
                  value:
                    statsDetails === null
                      ? "Not available"
                      : formatStatsChannelCountValue(
                          statsDetails.configuredChannelCount,
                        ),
                },
              ]}
            />

            <div className="field-grid roles-form-grid">
              <label className="field-stack">
                <span className="field-label">Update rule</span>
                <select
                  aria-label="Update rule"
                  value={configEnabledDraft}
                  onChange={(event) =>
                    setConfigEnabledDraft(event.target.value)
                  }
                >
                  <option value="enabled">Enabled</option>
                  <option value="disabled">Disabled</option>
                </select>
                <span className="meta-note">
                  Pause channel renames here without disabling the module-level
                  override for this server.
                </span>
              </label>

              <label className="field-stack">
                <span className="field-label">Update interval (minutes)</span>
                <input
                  aria-label="Update interval (minutes)"
                  inputMode="numeric"
                  min={1}
                  step={1}
                  type="number"
                  value={updateIntervalDraft}
                  onChange={(event) =>
                    setUpdateIntervalDraft(event.target.value)
                  }
                />
                <span className="meta-note">
                  Enter how often the selected server should refresh the
                  configured stats channel names.
                </span>
              </label>
            </div>

            {!canSaveStatsSettings ? (
              <div className="surface-subsection">
                <p className="section-label">Interval required</p>
                <p className="meta-note">
                  Enter a whole number greater than zero before saving the stats
                  schedule.
                </p>
              </div>
            ) : null}

            <div className="drawer-actions">
              <button
                className="button-primary"
                type="button"
                disabled={mutation.saving || !canSaveStatsSettings}
                onClick={() => void handleSaveStatsSettings()}
              >
                {mutation.saving && pendingFeatureId === statsFeature.id
                  ? "Saving..."
                  : "Save stats settings"}
              </button>
              <button
                className="button-secondary"
                type="button"
                onClick={closeDrawer}
              >
                Cancel
              </button>
            </div>
          </aside>
        </div>
      ) : null}
    </>
  );
}

function getDrawerLabel(feature: FeatureRecord) {
  return `Configure ${feature.label}`;
}
