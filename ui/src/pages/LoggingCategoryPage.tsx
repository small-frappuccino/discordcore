import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  buildMessageRouteChannelPickerOptions,
  formatGuildChannelValue,
} from "../features/features/discordEntities";
import {
  getFeatureAreaDefinition,
  getFeatureAreaRecords,
} from "../features/features/areas";
import {
  canEditLoggingChannel,
  describeLoggingDestination,
  getLoggingFeatureDetails,
  summarizeLoggingDestination,
  summarizeLoggingGuidance,
} from "../features/features/logging";
import {
  formatEffectiveSourceLabel,
  formatFeatureStatusLabel,
  formatOverrideLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";
import {
  AdvancedTextInput,
  AlertBanner,
  EntityPickerField,
  EmptyState,
  FeatureWorkspaceLayout,
  KeyValueList,
  LookupNotice,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";

export function LoggingCategoryPage() {
  const definition = getFeatureAreaDefinition("logging");
  const location = useLocation();
  const {
    authState,
    beginLogin,
    canEditSelectedGuild,
    currentOriginLabel,
    selectedGuild,
  } = useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const channelOptions = useGuildChannelOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");
  const [selectedFeatureId, setSelectedFeatureId] = useState("");
  const [channelDraft, setChannelDraft] = useState("");

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "logging");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const selectedFeature =
    areaFeatures.find((feature) => feature.id === selectedFeatureId) ?? null;
  const selectedFeatureDetails =
    selectedFeature === null ? null : getLoggingFeatureDetails(selectedFeature);
  const featuresRequiringChannel = areaFeatures.filter((feature) =>
    getLoggingFeatureDetails(feature).requiresChannel,
  );
  const configuredDestinations = featuresRequiringChannel.filter(
    (feature) => getLoggingFeatureDetails(feature).channelId !== "",
  ).length;
  const runtimeBlockedFeatures = areaFeatures.filter((feature) =>
    feature.blockers?.some((blocker) => blocker.code === "runtime_kill_switch"),
  );
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;

  useEffect(() => {
    if (canEditSelectedGuild) {
      return;
    }
    closeDrawer();
  }, [canEditSelectedGuild]);

  useEffect(() => {
    if (selectedFeature === null) {
      return;
    }
    if (!canEditLoggingChannel(selectedFeature)) {
      setSelectedFeatureId("");
      setChannelDraft("");
      return;
    }
    setChannelDraft(getLoggingFeatureDetails(selectedFeature).channelId);
  }, [selectedFeature]);

  const firstBlockedFeature = useMemo(
    () => areaFeatures.find((feature) => feature.readiness === "blocked") ?? null,
    [areaFeatures],
  );
  const messageRouteChannelOptions = useMemo(
    () => buildMessageRouteChannelPickerOptions(channelOptions.channels),
    [channelOptions.channels],
  );

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;

  async function handleRefreshLogging() {
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

  async function handleUseInherited(feature: FeatureRecord) {
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

  async function handleSaveDestination() {
    if (selectedFeature === null) {
      return;
    }

    setPendingFeatureId(selectedFeature.id);

    try {
      const updated = await mutation.patchFeature(selectedFeature.id, {
        channel_id: channelDraft.trim(),
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  function openDrawer(feature: FeatureRecord) {
    if (!canEditSelectedGuild || !canEditLoggingChannel(feature)) {
      return;
    }
    setSelectedFeatureId(feature.id);
    setChannelDraft(getLoggingFeatureDetails(feature).channelId);
  }

  function closeDrawer() {
    setSelectedFeatureId("");
    setChannelDraft("");
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

  function renderWorkspaceContent() {
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
                onClick={() => void handleRefreshLogging()}
              >
                Retry loading
              </button>
            ) : undefined
          }
        />
      );
    }

    if (areaFeatures.length === 0) {
      return (
        <div className="table-empty-state table-empty-state-compact">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>No logging routes yet</h2>
            <p className="section-description">
              This server does not expose any mapped logging features yet.
            </p>
          </div>
        </div>
      );
    }

    return (
      <div className="table-wrap">
        <table className="data-table feature-table logging-table">
          <thead>
            <tr>
              <th scope="col">Log route</th>
              <th scope="col">Destination</th>
              <th scope="col">Status</th>
              <th scope="col">Signal</th>
              <th scope="col">Actions</th>
            </tr>
          </thead>
          <tbody>
            {areaFeatures.map((feature) => {
              const details = getLoggingFeatureDetails(feature);
              const isPending =
                mutation.saving && pendingFeatureId === feature.id;

              return (
                <tr key={feature.id}>
                  <td>
                    <div className="feature-table-copy">
                      <strong>{feature.label}</strong>
                      <p>{feature.description}</p>
                    </div>
                  </td>
                  <td>
                    <div className="feature-table-copy">
                      <strong>
                        {formatGuildChannelValue(
                          getLoggingFeatureDetails(feature).channelId,
                          channelOptions.channels,
                          summarizeLoggingDestination(feature),
                        )}
                      </strong>
                      <p>{describeLoggingDestination(feature)}</p>
                    </div>
                  </td>
                  <td>
                    <div className="feature-status-cell">
                      <StatusBadge tone={getFeatureStatusTone(feature)}>
                        {formatFeatureStatusLabel(feature)}
                      </StatusBadge>
                      <span className="meta-note">
                        {formatOverrideLabel(feature.override_state)}
                      </span>
                    </div>
                  </td>
                  <td>
                    <div className="feature-table-copy">
                      <strong>
                        {feature.readiness === "blocked"
                          ? "Needs action"
                          : feature.effective_enabled
                            ? "Operational"
                            : "Disabled"}
                      </strong>
                      <p>{summarizeLoggingGuidance(feature)}</p>
                      {details.exclusiveModerationChannel ? (
                        <span className="meta-note">
                          Requires an exclusive moderation destination.
                        </span>
                      ) : null}
                    </div>
                  </td>
                  <td>
                    <div className="feature-row-actions">
                      {canEditSelectedGuild && canEditLoggingChannel(feature) ? (
                        <button
                          className="button-secondary"
                          type="button"
                          disabled={mutation.saving}
                          onClick={() => openDrawer(feature)}
                        >
                          Configure
                        </button>
                      ) : null}
                      <button
                        className="button-ghost"
                        type="button"
                        disabled={mutation.saving || !canEditSelectedGuild}
                        aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
                        onClick={() =>
                          void handleSetFeatureEnabled(
                            feature,
                            !feature.effective_enabled,
                          )
                        }
                      >
                        {isPending
                          ? "Saving..."
                          : feature.effective_enabled
                            ? "Disable"
                            : "Enable"}
                      </button>
                      {feature.override_state !== "inherit" ? (
                        <button
                          className="button-ghost"
                          type="button"
                          disabled={mutation.saving || !canEditSelectedGuild}
                          aria-label={`Use inherited setting for ${feature.label}`}
                          onClick={() => void handleUseInherited(feature)}
                        >
                          Use inherited
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    );
  }

  return (
    <>
      <section className="page-shell">
        <PageHeader
          eyebrow="Feature area"
          title={areaLabel}
          description="Configure event log routes for the selected server, keep destinations valid, and resolve blockers before relying on operational logging."
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
              <span className="meta-note">Server: {selectedServerLabel}</span>
              <span className="meta-note">Origin: {currentOriginLabel}</span>
            </>
          }
          actions={renderHeaderActions()}
        />

        <FeatureWorkspaceLayout
          notice={workspaceNotice}
          busyLabel={mutation.saving ? "Saving logging settings..." : undefined}
          summary={
            workspace.workspaceState === "ready" ? (
              <section
                className="overview-summary-strip"
                aria-label="Logging summary"
              >
                <MetricCard
                  label="Log routes"
                  value={String(areaSummary.total)}
                  description="Mapped logging features available for this server."
                />
                <MetricCard
                  label="Destinations set"
                  value={`${configuredDestinations}/${featuresRequiringChannel.length}`}
                  description="Routes that already have a configured destination channel."
                  tone={
                    configuredDestinations === featuresRequiringChannel.length &&
                    featuresRequiringChannel.length > 0
                      ? "success"
                      : "neutral"
                  }
                />
                <MetricCard
                  label="Ready"
                  value={String(areaSummary.ready)}
                  description="Enabled log routes that are not reporting blockers."
                  tone={areaSummary.ready > 0 ? "success" : "neutral"}
                />
                <MetricCard
                  label="Needs attention"
                  value={String(areaSummary.blocked)}
                  description="Routes blocked by missing destinations or runtime prerequisites."
                  tone={areaSummary.blocked > 0 ? "error" : "neutral"}
                />
              </section>
            ) : null
          }
          workspaceTitle="Manage logging routes"
          workspaceDescription="Keep the main workspace focused on destinations and current readiness. Open a route to configure its destination without leaving the table."
          workspaceMeta={
            workspace.workspaceState === "ready" ? (
              <>
                <span className="meta-note">{localOverrides} local overrides</span>
                <span className="meta-note">
                  {runtimeBlockedFeatures.length} runtime-blocked
                </span>
              </>
            ) : null
          }
          workspaceContent={renderWorkspaceContent()}
          aside={
            <aside className="page-aside">
              <SurfaceCard>
                <div className="card-copy">
                  <p className="section-label">Summary</p>
                  <h2>Category health</h2>
                  <p className="section-description">
                    Logging becomes reliable only after routes, runtime prerequisites,
                    and channel validation all line up.
                  </p>
                </div>

                <KeyValueList
                  items={[
                    {
                      label: "Server",
                      value: selectedServerLabel,
                    },
                    {
                      label: "Configured destinations",
                      value: `${configuredDestinations}/${featuresRequiringChannel.length}`,
                    },
                    {
                      label: "Blocked routes",
                      value: String(areaSummary.blocked),
                    },
                    {
                      label: "Current signal",
                      value: areaSummary.signal,
                    },
                  ]}
                />
              </SurfaceCard>

              <SurfaceCard>
                <div className="card-copy">
                  <p className="section-label">Guidance</p>
                  <h2>Operational notes</h2>
                  <p className="section-description">
                    Keep default logging routes visible in one list. Use the drawer
                    only when a route needs a destination or backend requirement review.
                  </p>
                </div>

                <ul className="feature-guidance-list">
                  <li>Configure destination channels before enabling new logging routes that require them.</li>
                  <li>Use inherited when a server should fall back to the configured default instead of pinning a local override.</li>
                  <li>Runtime kill switches and missing gateway intents appear through blockers and notices, not a separate settings page.</li>
                </ul>

                {firstBlockedFeature ? (
                  <div className="surface-subsection">
                    <p className="section-label">Current blocker</p>
                    <strong>{firstBlockedFeature.label}</strong>
                    <p className="meta-note">
                      {summarizeLoggingGuidance(firstBlockedFeature)}
                    </p>
                  </div>
                ) : null}

                {channelOptions.notice ? (
                  <LookupNotice
                    title="Channel references unavailable"
                    message={channelOptions.notice.message}
                    retryLabel="Retry channel lookup"
                    retryDisabled={channelOptions.loading}
                    onRetry={channelOptions.refresh}
                  />
                ) : null}
              </SurfaceCard>
            </aside>
          }
        />
      </section>

      {selectedFeature !== null &&
      canEditSelectedGuild &&
      canEditLoggingChannel(selectedFeature) ? (
        <div className="drawer-backdrop" onClick={closeDrawer} role="presentation">
          <aside
            aria-label={`Configure ${selectedFeature.label}`}
            aria-modal="true"
            className="drawer-panel logging-drawer"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="card-copy">
              <p className="section-label">Logging route</p>
              <div className="logging-drawer-title-row">
                <h2>{selectedFeature.label}</h2>
                <StatusBadge tone={getFeatureStatusTone(selectedFeature)}>
                  {formatFeatureStatusLabel(selectedFeature)}
                </StatusBadge>
              </div>
            </div>

            {mutation.notice ? <AlertBanner notice={mutation.notice} /> : null}

            <KeyValueList
              items={[
                {
                  label: "Applied from",
                  value: formatEffectiveSourceLabel(selectedFeature.effective_source),
                },
                {
                  label: "Override",
                  value: formatOverrideLabel(selectedFeature.override_state),
                },
                {
                  label: "Destination rule",
                  value: selectedFeatureDetails?.requiresChannel
                    ? "Needs destination channel"
                    : "No dedicated destination",
                },
                {
                  label: "Current signal",
                  value: summarizeLoggingGuidance(selectedFeature),
                },
              ]}
            />
            <EntityPickerField
              label="Destination channel"
              value={channelDraft}
              disabled={channelOptions.loading}
              onChange={setChannelDraft}
              options={messageRouteChannelOptions}
              placeholder={
                channelOptions.loading
                  ? "Loading channels..."
                  : messageRouteChannelOptions.length === 0
                    ? "No channels available"
                    : "No destination channel"
              }
              note={
                selectedFeatureDetails?.requiresChannel
                  ? undefined
                  : "Leave empty to clear the destination."
              }
            />

            {channelOptions.notice ? (
              <LookupNotice
                title="Channel references unavailable"
                message={channelOptions.notice.message}
                retryLabel="Retry channel lookup"
                retryDisabled={channelOptions.loading}
                onRetry={channelOptions.refresh}
              />
            ) : null}

            <AdvancedTextInput
              label="Channel ID fallback"
              inputLabel="Destination channel ID fallback"
              value={channelDraft}
              onChange={setChannelDraft}
              placeholder="Discord channel ID"
              note="Use only if channel lookup fails."
            />

            <div className="drawer-actions">
              <button
                className="button-primary"
                type="button"
                disabled={mutation.saving || !canEditSelectedGuild}
                onClick={() => void handleSaveDestination()}
              >
                {mutation.saving && pendingFeatureId === selectedFeature.id
                  ? "Saving..."
                  : "Save destination"}
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
