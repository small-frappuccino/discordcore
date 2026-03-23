import { useEffect, useMemo, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import { appRoutes } from "../app/routes";
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
  buildLoggingRequirementNotes,
  canEditLoggingChannel,
  describeLoggingDestination,
  getLoggingFeatureDetails,
  summarizeLoggingDestination,
  summarizeLoggingGuidance,
} from "../features/features/logging";
import {
  formatAutomodRuleCoverageValue,
  formatModerationRouteCoverageValue,
  getAutomodFeatureDetails,
  getModerationLogFeatures,
  summarizeAutomodRuleInventory,
  summarizeAutomodSignal,
} from "../features/features/moderation";
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
  AlertBanner,
  EntityPickerField,
  EmptyState,
  KeyValueList,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";

export function ModerationPage() {
  const definition = getFeatureAreaDefinition("moderation");
  const location = useLocation();
  const {
    authState,
    beginLogin,
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

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "moderation");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const automodFeature =
    areaFeatures.find((feature) => feature.id === "services.automod") ?? null;
  const moderationLogFeatures = getModerationLogFeatures(areaFeatures);
  const selectedFeature =
    moderationLogFeatures.find((feature) => feature.id === selectedFeatureId) ?? null;
  const selectedFeatureDetails =
    selectedFeature === null ? null : getLoggingFeatureDetails(selectedFeature);
  const firstBlockedFeature = useMemo(
    () => areaFeatures.find((feature) => feature.readiness === "blocked") ?? null,
    [areaFeatures],
  );
  const messageRouteChannelOptions = useMemo(
    () => buildMessageRouteChannelPickerOptions(channelOptions.channels),
    [channelOptions.channels],
  );
  const configuredModerationRoutes = moderationLogFeatures.filter(
    (feature) => getLoggingFeatureDetails(feature).channelId !== "",
  ).length;
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;

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

  async function handleRefreshModeration() {
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

  async function handleSaveModerationRoute() {
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
    if (!canEditLoggingChannel(feature)) {
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

    if (selectedGuild === null) {
      return null;
    }

    return (
      <button
        className="button-secondary"
        type="button"
        disabled={workspace.loading || mutation.saving || channelOptions.loading}
        onClick={() => void handleRefreshModeration()}
      >
        Refresh moderation
      </button>
    );
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
                onClick={() => void handleRefreshModeration()}
              >
                Retry loading
              </button>
            ) : undefined
          }
        />
      );
    }

    if (automodFeature === null && moderationLogFeatures.length === 0) {
      return (
        <div className="table-empty-state table-empty-state-compact">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>No moderation controls yet</h2>
            <p className="section-description">
              The selected server does not expose moderation feature records for
              this workspace yet.
            </p>
          </div>
        </div>
      );
    }

    const automodDetails =
      automodFeature === null ? null : getAutomodFeatureDetails(automodFeature);

    return (
      <div className="moderation-workspace-grid">
        {automodFeature !== null ? (
          <section className="surface-subsection moderation-service-panel">
            <div className="moderation-service-head">
              <div className="card-copy moderation-service-copy">
                <p className="section-label">Automatic moderation</p>
                <div className="moderation-title-row">
                  <h3>{automodFeature.label}</h3>
                  <StatusBadge tone={getFeatureStatusTone(automodFeature)}>
                    {formatFeatureStatusLabel(automodFeature)}
                  </StatusBadge>
                </div>
                <p className="section-description">
                  Keep the AutoMod service state, rule coverage, and current
                  blockers visible in one place before a dedicated rule editor
                  lands in this module.
                </p>
              </div>

              <span className="meta-pill subtle-pill">
                {automodFeature.override_state === "inherit"
                  ? "Using default"
                  : "Configured here"}
              </span>
            </div>

            <KeyValueList
              items={[
                {
                  label: "Module state",
                  value: automodFeature.effective_enabled ? "On" : "Off",
                },
                {
                  label: "Rulesets",
                  value: String(automodDetails?.rulesetCount ?? 0),
                },
                {
                  label: "Loose rules",
                  value: String(automodDetails?.looseRuleCount ?? 0),
                },
                {
                  label: "Blocklists",
                  value: String(automodDetails?.blocklistCount ?? 0),
                },
                {
                  label: "Current signal",
                  value: summarizeAutomodSignal(automodFeature),
                },
              ]}
            />

            <div className="surface-subsection moderation-rules-preview">
              <div className="card-copy moderation-rules-copy">
                <p className="section-label">Rules workspace</p>
                <h4>Prepared for future rule management</h4>
                <p className="meta-note">
                  This module already tracks service readiness and current rule
                  counts, so a dedicated editor can plug into the same
                  navigation later without changing where admins manage
                  moderation.
                </p>
              </div>

              <KeyValueList
                items={[
                  {
                    label: "Rule coverage",
                    value: formatAutomodRuleCoverageValue(automodFeature),
                  },
                  {
                    label: "Inventory",
                    value: summarizeAutomodRuleInventory(automodFeature),
                  },
                ]}
              />
            </div>

            <div className="inline-actions moderation-service-actions">
              <button
                className="button-primary"
                type="button"
                disabled={mutation.saving}
                onClick={() =>
                  void handleSetFeatureEnabled(
                    automodFeature,
                    !automodFeature.effective_enabled,
                  )
                }
              >
                {mutation.saving && pendingFeatureId === automodFeature.id
                  ? "Saving..."
                  : automodFeature.effective_enabled
                    ? "Disable Automod service"
                    : "Enable Automod service"}
              </button>
              {automodFeature.override_state !== "inherit" ? (
                <button
                  className="button-ghost"
                  type="button"
                  disabled={mutation.saving}
                  onClick={() => void handleUseDefault(automodFeature)}
                >
                  Use default
                </button>
              ) : null}
            </div>
          </section>
        ) : null}

        <section className="surface-subsection moderation-log-panel">
          <div className="moderation-log-panel-header">
            <div className="card-copy">
              <p className="section-label">Moderation logging</p>
              <div className="moderation-title-row">
                <h3>Enforcement event routes</h3>
                <StatusBadge
                  tone={
                    moderationLogFeatures.some(
                      (feature) => feature.readiness === "blocked",
                    )
                      ? "error"
                      : moderationLogFeatures.some(
                            (feature) => feature.effective_enabled,
                          )
                        ? "success"
                        : "neutral"
                  }
                >
                  {moderationLogFeatures.length === 0
                    ? "Not mapped"
                    : `${configuredModerationRoutes}/${moderationLogFeatures.length} configured`}
                </StatusBadge>
              </div>
              <p className="section-description">
                Keep AutoMod actions, moderation cases, and cleanup events
                routed to the right moderation destinations without leaving this
                workspace.
              </p>
            </div>
          </div>

          {moderationLogFeatures.length === 0 ? (
            <div className="table-empty-state table-empty-state-compact">
              <div className="card-copy">
                <p className="section-label">Routes</p>
                <h2>No moderation log routes yet</h2>
                <p className="section-description">
                  This server is not exposing the moderation log routes in the
                  current workspace snapshot.
                </p>
              </div>
            </div>
          ) : (
            <div className="table-wrap">
              <table className="data-table feature-table moderation-log-table">
                <thead>
                  <tr>
                    <th scope="col">Route</th>
                    <th scope="col">Destination</th>
                    <th scope="col">Status</th>
                    <th scope="col">Signal</th>
                    <th scope="col">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {moderationLogFeatures.map((feature) => {
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
                                details.channelId,
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
                            {canEditLoggingChannel(feature) ? (
                              <button
                                className="button-secondary"
                                type="button"
                                onClick={() => openDrawer(feature)}
                              >
                                Configure
                              </button>
                            ) : null}
                            <button
                              className="button-ghost"
                              type="button"
                              disabled={mutation.saving}
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
                                disabled={mutation.saving}
                                aria-label={`Use inherited setting for ${feature.label}`}
                                onClick={() => void handleUseDefault(feature)}
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
          )}
        </section>
      </div>
    );
  }

  return (
    <>
      <section className="page-shell">
        <PageHeader
          eyebrow="Feature area"
          title={areaLabel}
          description="Review AutoMod readiness, moderation event logging, and the blockers that still need attention for the selected server."
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
              <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
              <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
            </>
          }
          actions={renderHeaderActions()}
        />

        {workspace.workspaceState === "ready" && automodFeature !== null ? (
          <section
            className="overview-summary-strip"
            aria-label="Moderation summary"
          >
            <MetricCard
              label="AutoMod"
              value={formatFeatureStatusLabel(automodFeature)}
              description={summarizeAutomodSignal(automodFeature)}
              tone={getFeatureStatusTone(automodFeature)}
            />
            <MetricCard
              label="Rule coverage"
              value={formatAutomodRuleCoverageValue(automodFeature)}
              description={summarizeAutomodRuleInventory(automodFeature)}
              tone={
                getAutomodFeatureDetails(automodFeature).rulesetCount > 0
                  ? "info"
                  : "neutral"
              }
            />
            <MetricCard
              label="Moderation logs"
              value={formatModerationRouteCoverageValue(moderationLogFeatures)}
              description="AutoMod, moderation case, and cleanup routes currently mapped in this workspace."
              tone={configuredModerationRoutes > 0 ? "info" : "neutral"}
            />
            <MetricCard
              label="Needs attention"
              value={String(areaSummary.blocked)}
              description="Enabled moderation controls still reporting blockers."
              tone={areaSummary.blocked > 0 ? "error" : "neutral"}
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
                    <h2>Moderation controls</h2>
                    <p className="section-description">
                      Keep the service switch, moderation event routes, and rule
                      readiness in one operational workspace. This keeps the
                      day-to-day moderation surface clear even before the future
                      rule editor lands.
                    </p>
                  </div>
                  <div className="workspace-view-meta">
                    {workspace.workspaceState === "ready" ? (
                      <>
                        <span className="meta-pill subtle-pill">
                          {localOverrides} local overrides
                        </span>
                        <span className="meta-pill subtle-pill">
                          {configuredModerationRoutes}/{moderationLogFeatures.length} routes configured
                        </span>
                      </>
                    ) : null}
                  </div>
                </div>

                <AlertBanner
                  notice={workspaceNotice}
                  busyLabel={
                    mutation.saving
                      ? "Saving moderation settings..."
                      : workspace.loading || channelOptions.loading
                        ? "Refreshing moderation workspace..."
                        : undefined
                  }
                />

                {renderWorkspaceContent()}
              </div>
            </SurfaceCard>
          </div>

          <aside className="page-aside">
            <SurfaceCard>
              <div className="card-copy">
                <p className="section-label">Summary</p>
                <h2>Current moderation state</h2>
                <p className="section-description">
                  Use this panel to confirm the AutoMod service state, current
                  rule coverage, and whether moderation events are already routed
                  to the expected destinations.
                </p>
              </div>

              <KeyValueList
                items={[
                  {
                    label: "Server",
                    value: selectedServerLabel,
                  },
                  {
                    label: "AutoMod",
                    value:
                      automodFeature === null
                        ? "Not available"
                        : formatFeatureStatusLabel(automodFeature),
                  },
                  {
                    label: "Rule coverage",
                    value:
                      automodFeature === null
                        ? "Not available"
                        : formatAutomodRuleCoverageValue(automodFeature),
                  },
                  {
                    label: "Moderation logs",
                    value: formatModerationRouteCoverageValue(moderationLogFeatures),
                  },
                  {
                    label: "Current blocker",
                    value:
                      firstBlockedFeature?.blockers?.[0]?.message ??
                      areaSummary.signal,
                  },
                ]}
              />
            </SurfaceCard>

            <SurfaceCard>
              <div className="card-copy">
                <p className="section-label">Guidance</p>
                <h2>How this page works</h2>
                <p className="section-description">
                  Keep the primary workspace centered on moderation outcomes:
                  service state, logging routes, and blockers. Diagnostics and
                  runtime details stay secondary.
                </p>
              </div>

              <ul className="feature-guidance-list">
                <li>Enable AutoMod only after the current rule coverage is ready for the selected server.</li>
                <li>Keep moderation event routes configured before relying on enforcement logs in staff workflows.</li>
                <li>Runtime gating and low-level logging toggles still belong in Settings diagnostics.</li>
              </ul>

              {firstBlockedFeature ? (
                <div className="surface-subsection">
                  <p className="section-label">Current blocker</p>
                  <strong>{firstBlockedFeature.label}</strong>
                  <p className="meta-note">
                    {firstBlockedFeature.blockers?.[0]?.message ?? areaSummary.signal}
                  </p>
                </div>
              ) : null}

              {channelOptions.notice ? (
                <div className="surface-subsection">
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
                </div>
              ) : null}
            </SurfaceCard>
          </aside>
        </section>
      </section>

      {selectedFeature !== null && canEditLoggingChannel(selectedFeature) ? (
        <div className="drawer-backdrop" onClick={closeDrawer} role="presentation">
          <aside
            aria-label={`Configure ${selectedFeature.label}`}
            aria-modal="true"
            className="drawer-panel moderation-drawer"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="card-copy">
              <p className="section-label">Moderation logging</p>
              <div className="logging-drawer-title-row">
                <h2>{selectedFeature.label}</h2>
                <StatusBadge tone={getFeatureStatusTone(selectedFeature)}>
                  {formatFeatureStatusLabel(selectedFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">{selectedFeature.description}</p>
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
              note="Leave this empty to clear the destination or keep the route without a dedicated channel."
            />

            {channelOptions.notice ? (
              <div className="surface-subsection">
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
              </div>
            ) : null}

            <details className="details-panel">
              <summary>Advanced</summary>
              <div className="details-content">
                <label className="field-stack">
                  <span className="field-label">Channel ID fallback</span>
                  <input
                    aria-label="Destination channel ID fallback"
                    value={channelDraft}
                    onChange={(event) => setChannelDraft(event.target.value)}
                    placeholder="Discord channel ID"
                  />
                  <span className="meta-note">
                    Use this only when the channel picker is unavailable or
                    when you need to paste a channel ID directly.
                  </span>
                </label>
              </div>
            </details>

            <div className="surface-subsection">
              <p className="section-label">Requirements</p>
              <ul className="feature-guidance-list">
                {buildLoggingRequirementNotes(selectedFeature).map((note) => (
                  <li key={note}>{note}</li>
                ))}
              </ul>
            </div>

            {selectedFeature.blockers?.some(
              (blocker) =>
                blocker.code === "runtime_kill_switch" ||
                blocker.code === "missing_intent",
            ) ? (
              <div className="surface-subsection">
                <p className="section-label">Needs diagnostics</p>
                <p className="meta-note">
                  This route depends on runtime conditions that are reviewed in
                  Settings diagnostics.
                </p>
                <div className="sidebar-actions">
                  <Link
                    className="button-secondary"
                    to={`${appRoutes.settings}#diagnostics`}
                  >
                    Open Settings diagnostics
                  </Link>
                </div>
              </div>
            ) : null}

            <div className="drawer-actions">
              <button
                className="button-primary"
                type="button"
                disabled={mutation.saving}
                onClick={() => void handleSaveModerationRoute()}
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
