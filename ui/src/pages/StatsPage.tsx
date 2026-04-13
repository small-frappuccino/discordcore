import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  EmptyState,
  FlatPageLayout,
  KeyValueList,
  LookupNotice,
  MetricCard,
  PageHeader,
  StatusBadge,
  UnsavedChangesBar,
} from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  getFeatureAreaDefinition,
  getFeatureAreaRecords,
} from "../features/features/areas";
import {
  featureTags,
  findFeatureByTag,
} from "../features/features/featureContract";
import {
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import {
  formatStatsChannelAudience,
  formatStatsChannelLabel,
  formatStatsChannelTemplate,
  formatStatsChannelValue,
  formatStatsConfigValue,
  formatStatsIntervalValue,
  getStatsFeatureDetails,
  summarizeStatsSignal,
} from "../features/features/stats";
import { featureSupportsAnyField } from "../features/features/model";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";

export function StatsPage() {
  const definition = getFeatureAreaDefinition("stats");
  const location = useLocation();
  const { authState, beginLogin, canEditSelectedGuild } = useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const channelOptions = useGuildChannelOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "stats");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const statsFeature = findFeatureByTag(
    areaFeatures,
    featureTags.statsPrimary,
  );
  const statsDetails =
    statsFeature === null ? null : getStatsFeatureDetails(statsFeature);

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
        workspace.updateFeature(updated);
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
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveStatsSettings(
    configEnabled: boolean,
    updateIntervalMins: number,
  ) {
    if (statsFeature === null) {
      return;
    }

    setPendingFeatureId(statsFeature.id);

    try {
      const updated = await mutation.patchFeature(statsFeature.id, {
        config_enabled: configEnabled,
        update_interval_mins: updateIntervalMins,
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
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

    return null;
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
        <StatsScheduleSection
          feature={statsFeature}
          details={statsDetails}
          canEditSelectedGuild={canEditSelectedGuild}
          mutationSaving={mutation.saving}
          pendingFeatureId={pendingFeatureId}
          onClearNotice={mutation.clearNotice}
          onSave={handleSaveStatsSettings}
          onSetFeatureEnabled={handleSetFeatureEnabled}
          onUseDefault={handleUseDefault}
        />

        {channelOptions.notice ? (
          <LookupNotice
            as="section"
            title="Channel references unavailable"
            message={channelOptions.notice.message}
            retryLabel="Retry channel lookup"
            retryDisabled={channelOptions.loading}
            onRetry={channelOptions.refresh}
          />
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
              : formatWorkspaceStateTitle(areaLabel, workspace.workspaceState)}
          </StatusBadge>
        }
        actions={renderHeaderActions()}
      />

      <FlatPageLayout
        notice={workspaceNotice}
        summary={
          workspace.workspaceState === "ready" &&
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
            </section>
          ) : null
        }
        workspaceTitle="Stats configuration"
        workspaceDescription="Keep the schedule controls and current inventory visible in the main workspace instead of editing the stats schedule in a separate drawer."
      >
        {renderPageState()}
      </FlatPageLayout>
    </section>
  );
}

interface StatsScheduleSectionProps {
  feature: FeatureRecord;
  details: ReturnType<typeof getStatsFeatureDetails>;
  canEditSelectedGuild: boolean;
  mutationSaving: boolean;
  pendingFeatureId: string;
  onClearNotice: () => void;
  onSave: (configEnabled: boolean, updateIntervalMins: number) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
  onUseDefault: (feature: FeatureRecord) => Promise<void>;
}

function StatsScheduleSection({
  feature,
  details,
  canEditSelectedGuild,
  mutationSaving,
  pendingFeatureId,
  onClearNotice,
  onSave,
  onSetFeatureEnabled,
  onUseDefault,
}: StatsScheduleSectionProps) {
  const [configEnabledDraft, setConfigEnabledDraft] = useState(
    details.configEnabled ? "enabled" : "disabled",
  );
  const [updateIntervalDraft, setUpdateIntervalDraft] = useState(
    String(details.updateIntervalMins),
  );
  const canEditSettings =
    canEditSelectedGuild &&
    featureSupportsAnyField(feature, [
      "config_enabled",
      "update_interval_mins",
    ]);
  const parsedUpdateIntervalMins = Number.parseInt(
    updateIntervalDraft.trim(),
    10,
  );
  const canSaveStatsSettings =
    Number.isFinite(parsedUpdateIntervalMins) && parsedUpdateIntervalMins > 0;
  const hasUnsavedChanges =
    details.configEnabled !== (configEnabledDraft === "enabled") ||
    details.updateIntervalMins !== parsedUpdateIntervalMins;

  useEffect(() => {
    setConfigEnabledDraft(details.configEnabled ? "enabled" : "disabled");
  }, [details.configEnabled]);

  useEffect(() => {
    setUpdateIntervalDraft(String(details.updateIntervalMins));
  }, [details.updateIntervalMins]);

  function handleReset() {
    onClearNotice();
    setConfigEnabledDraft(details.configEnabled ? "enabled" : "disabled");
    setUpdateIntervalDraft(String(details.updateIntervalMins));
  }

  return (
    <section className="flat-config-section">
      <div className="flat-config-header">
        <div className="card-copy flat-config-copy">
          <p className="section-label">Primary workflow</p>
          <div className="flat-config-title-row">
            <h2>Stats updates</h2>
            <StatusBadge tone={getFeatureStatusTone(feature)}>
              {formatFeatureStatusLabel(feature)}
            </StatusBadge>
          </div>
          <p className="section-description">
            Keep the stats module enabled, choose whether updates should run,
            and adjust the interval without leaving the main page.
          </p>
        </div>
      </div>

      <KeyValueList
        items={[
          {
            label: "Update rule",
            value: formatStatsConfigValue(details.configEnabled),
          },
          {
            label: "Update interval",
            value: formatStatsIntervalValue(details.updateIntervalMins),
          },
          {
            label: "Current signal",
            value: summarizeStatsSignal(feature),
          },
        ]}
      />

      <div className="flat-config-fields">
        <label className="field-stack">
          <span className="field-label">Update rule</span>
          <select
            aria-label="Update rule"
            disabled={!canEditSettings}
            value={configEnabledDraft}
            onChange={(event) => setConfigEnabledDraft(event.target.value)}
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
            disabled={!canEditSettings}
            inputMode="numeric"
            min={1}
            step={1}
            type="number"
            value={updateIntervalDraft}
            onChange={(event) => setUpdateIntervalDraft(event.target.value)}
          />
          <span className="meta-note">
            Enter how often the selected server should refresh the configured
            stats channel names.
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

      <div className="inline-actions flat-config-actions">
        <button
          className="button-secondary"
          type="button"
          disabled={mutationSaving || !canEditSelectedGuild}
          onClick={() =>
            void onSetFeatureEnabled(feature, !feature.effective_enabled)
          }
        >
          {mutationSaving && pendingFeatureId === feature.id
            ? "Saving..."
            : feature.effective_enabled
              ? "Disable stats module"
              : "Enable stats module"}
        </button>
        {feature.override_state !== "inherit" ? (
          <button
            className="button-ghost"
            type="button"
            disabled={mutationSaving || !canEditSelectedGuild}
            onClick={() => void onUseDefault(feature)}
          >
            Use default
          </button>
        ) : null}
      </div>

      <UnsavedChangesBar
        hasUnsavedChanges={hasUnsavedChanges}
        saveLabel={
          mutationSaving && pendingFeatureId === feature.id
            ? "Saving..."
            : "Save changes"
        }
        saving={mutationSaving && pendingFeatureId === feature.id}
        disabled={!canEditSettings || !canSaveStatsSettings}
        onReset={handleReset}
        onSave={() =>
          onSave(configEnabledDraft === "enabled", parsedUpdateIntervalMins)
        }
      />
    </section>
  );
}
