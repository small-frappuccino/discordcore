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
  buildLoggingRequirementNotes,
  canEditLoggingChannel,
  describeLoggingDestination,
  getLoggingFeatureDetails,
  summarizeLoggingDestination,
  summarizeLoggingGuidance,
} from "../features/features/logging";
import {
  canEditMuteRole,
  formatAutomodModeValue,
  formatModerationRouteCoverageValue,
  getModerationLogFeatures,
  getMuteRoleFeatureDetails,
  summarizeAutomodMode,
  summarizeAutomodSignal,
  summarizeMuteRoleSignal,
} from "../features/features/moderation";
import {
  formatRoleOptionLabel,
  formatRoleValue,
} from "../features/features/roles";
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
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";
import {
  AdvancedTextInput,
  AlertBanner,
  EntityPickerField,
  EmptyState,
  KeyValueList,
  LookupNotice,
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
  const roleOptions = useGuildRoleOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");
  const [selectedFeatureId, setSelectedFeatureId] = useState("");
  const [channelDraft, setChannelDraft] = useState("");
  const [roleDraft, setRoleDraft] = useState("");

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
  const muteRoleFeature =
    areaFeatures.find((feature) => feature.id === "moderation.mute_role") ?? null;
  const moderationLogFeatures = getModerationLogFeatures(areaFeatures);
  const selectedFeature =
    areaFeatures.find((feature) => feature.id === selectedFeatureId) ?? null;
  const firstBlockedFeature = useMemo(
    () => areaFeatures.find((feature) => feature.readiness === "blocked") ?? null,
    [areaFeatures],
  );
  const messageRouteChannelOptions = useMemo(
    () => buildMessageRouteChannelPickerOptions(channelOptions.channels),
    [channelOptions.channels],
  );
  const muteRoleOptions = useMemo(
    () =>
      roleOptions.roles.map((role) => ({
        value: role.id,
        label: formatRoleOptionLabel(role),
        disabled: role.is_default || role.managed,
      })),
    [roleOptions.roles],
  );
  const configuredModerationRoutes = moderationLogFeatures.filter(
    (feature) => getLoggingFeatureDetails(feature).channelId !== "",
  ).length;
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;
  const selectedIsMuteRole = selectedFeature?.id === "moderation.mute_role";
  const muteRoleId =
    muteRoleFeature === null
      ? ""
      : getMuteRoleFeatureDetails(muteRoleFeature).roleId;

  useEffect(() => {
    if (selectedFeature === null) {
      setChannelDraft("");
      setRoleDraft("");
      return;
    }

    if (selectedFeature.id === "moderation.mute_role") {
      setRoleDraft(getMuteRoleFeatureDetails(selectedFeature).roleId);
      setChannelDraft("");
      return;
    }

    if (!canEditLoggingChannel(selectedFeature)) {
      setSelectedFeatureId("");
      setChannelDraft("");
      setRoleDraft("");
      return;
    }

    setChannelDraft(getLoggingFeatureDetails(selectedFeature).channelId);
    setRoleDraft("");
  }, [selectedFeature]);

  async function handleRefreshModeration() {
    await Promise.all([
      workspace.refresh(),
      channelOptions.refresh(),
      roleOptions.refresh(),
    ]);
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

  async function handleSaveSelectedFeature() {
    if (selectedFeature === null) {
      return;
    }

    setPendingFeatureId(selectedFeature.id);

    try {
      const updated = await mutation.patchFeature(
        selectedFeature.id,
        selectedFeature.id === "moderation.mute_role"
          ? { role_id: roleDraft.trim() }
          : { channel_id: channelDraft.trim() },
      );
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  function openDrawer(feature: FeatureRecord) {
    if (!canEditLoggingChannel(feature) && !canEditMuteRole(feature)) {
      return;
    }

    setSelectedFeatureId(feature.id);
    if (feature.id === "moderation.mute_role") {
      setRoleDraft(getMuteRoleFeatureDetails(feature).roleId);
      setChannelDraft("");
      return;
    }

    setChannelDraft(getLoggingFeatureDetails(feature).channelId);
    setRoleDraft("");
  }

  function closeDrawer() {
    setSelectedFeatureId("");
    setChannelDraft("");
    setRoleDraft("");
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
          workspace.loading ||
          mutation.saving ||
          channelOptions.loading ||
          roleOptions.loading
        }
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

    if (
      automodFeature === null &&
      muteRoleFeature === null &&
      moderationLogFeatures.length === 0
    ) {
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

    return (
      <ModerationWorkspacePanels
        automodFeature={automodFeature}
        muteRoleFeature={muteRoleFeature}
        muteRoleId={muteRoleId}
        moderationLogFeatures={moderationLogFeatures}
        configuredModerationRoutes={configuredModerationRoutes}
        channelOptions={channelOptions}
        roleOptions={roleOptions}
        mutation={mutation}
        pendingFeatureId={pendingFeatureId}
        onOpenDrawer={openDrawer}
        onSetFeatureEnabled={handleSetFeatureEnabled}
        onUseDefault={handleUseDefault}
      />
    );
  }

  return (
    <>
      <section className="page-shell">
        <PageHeader
          eyebrow="Feature area"
          title={areaLabel}
          description="Review the logging-only AutoMod listener, the mute role, and the routes used by ban, massban, kick, mute, timeout, and warnings workflows."
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

        {workspace.workspaceState === "ready" ? (
          <section
            className="overview-summary-strip"
            aria-label="Moderation summary"
          >
            <MetricCard
              label="AutoMod"
              value={
                automodFeature === null
                  ? "Not available"
                  : formatFeatureStatusLabel(automodFeature)
              }
              description={
                automodFeature === null
                  ? "This server does not expose the AutoMod listener record."
                  : summarizeAutomodMode(automodFeature)
              }
              tone={
                automodFeature === null
                  ? "neutral"
                  : getFeatureStatusTone(automodFeature)
              }
            />
            <MetricCard
              label="Mute role"
              value={formatRoleValue(muteRoleId, roleOptions.roles)}
              description={
                muteRoleFeature === null
                  ? "This server does not expose a mute role control."
                  : summarizeMuteRoleSignal(muteRoleFeature)
              }
              tone={
                muteRoleFeature === null
                  ? "neutral"
                  : getFeatureStatusTone(muteRoleFeature)
              }
            />
            <MetricCard
              label="Moderation logs"
              value={formatModerationRouteCoverageValue(moderationLogFeatures)}
              description="AutoMod action and moderation case routes configured in this workspace."
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
                      Keep the moderation surface focused on the supported staff
                      actions: log readiness, mute-role setup, and the listener
                      state used alongside Discord native AutoMod.
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
                      : workspace.loading ||
                          channelOptions.loading ||
                          roleOptions.loading
                        ? "Refreshing moderation workspace..."
                        : undefined
                  }
                />

                {renderWorkspaceContent()}
              </div>
            </SurfaceCard>
          </div>

          <ModerationAside
            selectedServerLabel={selectedServerLabel}
            automodFeature={automodFeature}
            muteRoleId={muteRoleId}
            moderationLogFeatures={moderationLogFeatures}
            firstBlockedFeature={firstBlockedFeature}
            areaSummarySignal={areaSummary.signal}
            channelOptions={channelOptions}
            roleOptions={roleOptions}
          />
        </section>
      </section>

      {selectedFeature !== null ? (
        <ModerationFeatureDrawer
          selectedFeature={selectedFeature}
          selectedIsMuteRole={selectedIsMuteRole}
          mutationNotice={mutation.notice}
          mutationSaving={mutation.saving}
          pendingFeatureId={pendingFeatureId}
          roleDraft={roleDraft}
          setRoleDraft={setRoleDraft}
          muteRoleOptions={muteRoleOptions}
          roleOptions={roleOptions}
          channelDraft={channelDraft}
          setChannelDraft={setChannelDraft}
          channelOptions={channelOptions}
          messageRouteChannelOptions={messageRouteChannelOptions}
          onSave={handleSaveSelectedFeature}
          onClose={closeDrawer}
        />
      ) : null}
    </>
  );
}

interface ModerationWorkspacePanelsProps {
  automodFeature: FeatureRecord | null;
  muteRoleFeature: FeatureRecord | null;
  muteRoleId: string;
  moderationLogFeatures: FeatureRecord[];
  configuredModerationRoutes: number;
  channelOptions: ReturnType<typeof useGuildChannelOptions>;
  roleOptions: ReturnType<typeof useGuildRoleOptions>;
  mutation: ReturnType<typeof useFeatureMutation>;
  pendingFeatureId: string;
  onOpenDrawer: (feature: FeatureRecord) => void;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
  onUseDefault: (feature: FeatureRecord) => Promise<void>;
}

function ModerationWorkspacePanels({
  automodFeature,
  muteRoleFeature,
  muteRoleId,
  moderationLogFeatures,
  configuredModerationRoutes,
  channelOptions,
  roleOptions,
  mutation,
  pendingFeatureId,
  onOpenDrawer,
  onSetFeatureEnabled,
  onUseDefault,
}: ModerationWorkspacePanelsProps) {
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
                Discord still owns the actual AutoMod rules. This workspace only
                keeps the listener state and related log readiness visible for
                staff.
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
                label: "Mode",
                value: formatAutomodModeValue(automodFeature),
              },
              {
                label: "Current signal",
                value: summarizeAutomodSignal(automodFeature),
              },
              {
                label: "Supported actions",
                value: "ban, massban, kick, mute, timeout, warnings",
              },
            ]}
          />

          <div className="inline-actions moderation-service-actions">
            <button
              className="button-primary"
              type="button"
              disabled={mutation.saving}
              onClick={() =>
                void onSetFeatureEnabled(
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
                onClick={() => void onUseDefault(automodFeature)}
              >
                Use default
              </button>
            ) : null}
          </div>
        </section>
      ) : null}

      {muteRoleFeature !== null ? (
        <section className="surface-subsection moderation-service-panel">
          <div className="moderation-service-head">
            <div className="card-copy moderation-service-copy">
              <p className="section-label">Role-based mute</p>
              <div className="moderation-title-row">
                <h3>{muteRoleFeature.label}</h3>
                <StatusBadge tone={getFeatureStatusTone(muteRoleFeature)}>
                  {formatFeatureStatusLabel(muteRoleFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">
                Configure the role applied by the mute command. Keep it
                assignable by the bot and below both bot and moderator top
                roles.
              </p>
            </div>

            <span className="meta-pill subtle-pill">
              {muteRoleFeature.override_state === "inherit"
                ? "Using default"
                : "Configured here"}
            </span>
          </div>

          <KeyValueList
            items={[
              {
                label: "Module state",
                value: muteRoleFeature.effective_enabled ? "On" : "Off",
              },
              {
                label: "Configured role",
                value: formatRoleValue(muteRoleId, roleOptions.roles),
              },
              {
                label: "Current signal",
                value: summarizeMuteRoleSignal(muteRoleFeature),
              },
              {
                label: "Used by",
                value: "/moderation mute",
              },
            ]}
          />

          {roleOptions.notice ? (
            <LookupNotice
              title="Role references unavailable"
              message={roleOptions.notice.message}
              retryLabel="Retry role lookup"
              retryDisabled={roleOptions.loading}
              onRetry={roleOptions.refresh}
            />
          ) : null}

          <div className="inline-actions moderation-service-actions">
            {canEditMuteRole(muteRoleFeature) ? (
              <button
                className="button-secondary"
                type="button"
                onClick={() => onOpenDrawer(muteRoleFeature)}
              >
                Configure mute role
              </button>
            ) : null}
            <button
              className="button-ghost"
              type="button"
              disabled={mutation.saving}
              onClick={() =>
                void onSetFeatureEnabled(
                  muteRoleFeature,
                  !muteRoleFeature.effective_enabled,
                )
              }
            >
              {mutation.saving && pendingFeatureId === muteRoleFeature.id
                ? "Saving..."
                : muteRoleFeature.effective_enabled
                  ? "Disable"
                  : "Enable"}
            </button>
            {muteRoleFeature.override_state !== "inherit" ? (
              <button
                className="button-ghost"
                type="button"
                disabled={mutation.saving}
                onClick={() => void onUseDefault(muteRoleFeature)}
              >
                Use inherited
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
              Keep AutoMod executions and moderation case logs routed to the
              right staff destinations without leaving this workspace.
            </p>
          </div>
        </div>

        {moderationLogFeatures.length === 0 ? (
          <div className="table-empty-state table-empty-state-compact">
            <div className="card-copy">
              <p className="section-label">Routes</p>
              <h2>No moderation log routes yet</h2>
              <p className="section-description">
                This server is not exposing moderation log routes in the
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
                        </div>
                      </td>
                      <td>
                        <div className="feature-row-actions">
                          {canEditLoggingChannel(feature) ? (
                            <button
                              aria-label={`Configure ${feature.label}`}
                              className="button-secondary"
                              type="button"
                              onClick={() => onOpenDrawer(feature)}
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
                              void onSetFeatureEnabled(
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
                              onClick={() => void onUseDefault(feature)}
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

interface ModerationAsideProps {
  selectedServerLabel: string;
  automodFeature: FeatureRecord | null;
  muteRoleId: string;
  moderationLogFeatures: FeatureRecord[];
  firstBlockedFeature: FeatureRecord | null;
  areaSummarySignal: string;
  channelOptions: ReturnType<typeof useGuildChannelOptions>;
  roleOptions: ReturnType<typeof useGuildRoleOptions>;
}

function ModerationAside({
  selectedServerLabel,
  automodFeature,
  muteRoleId,
  moderationLogFeatures,
  firstBlockedFeature,
  areaSummarySignal,
  channelOptions,
  roleOptions,
}: ModerationAsideProps) {
  return (
    <aside className="page-aside">
      <SurfaceCard>
        <div className="card-copy">
          <p className="section-label">Summary</p>
          <h2>Current moderation state</h2>
          <p className="section-description">
            Use this panel to confirm whether the supported moderation
            workflows have their mute role and staff log routes ready.
          </p>
        </div>

        <KeyValueList
          items={[
            {
              label: "Server",
              value: selectedServerLabel,
            },
            {
              label: "Supported actions",
              value: "ban, massban, kick, mute, timeout, warnings",
            },
            {
              label: "AutoMod mode",
              value:
                automodFeature === null
                  ? "Not available"
                  : formatAutomodModeValue(automodFeature),
            },
            {
              label: "Mute role",
              value: formatRoleValue(muteRoleId, roleOptions.roles),
            },
            {
              label: "Moderation logs",
              value: formatModerationRouteCoverageValue(moderationLogFeatures),
            },
            {
              label: "Current blocker",
              value:
                firstBlockedFeature?.blockers?.[0]?.message ?? areaSummarySignal,
            },
          ]}
        />
      </SurfaceCard>

      <SurfaceCard>
        <div className="card-copy">
          <p className="section-label">Guidance</p>
          <h2>How this page works</h2>
          <p className="section-description">
            Keep the workspace centered on supported moderation outcomes. Rule
            engines and low-level diagnostics stay out of the primary path
            here.
          </p>
        </div>

        <ul className="feature-guidance-list">
          <li>
            Supported moderation commands are ban, massban, kick, mute,
            timeout, and warnings.
          </li>
          <li>
            AutoMod is logging-only here. Discord owns the actual rule
            execution and enforcement settings.
          </li>
          <li>
            Configure the mute role and moderation log routes before relying on
            staff workflows.
          </li>
        </ul>

        {firstBlockedFeature ? (
          <div className="surface-subsection">
            <p className="section-label">Current blocker</p>
            <strong>{firstBlockedFeature.label}</strong>
            <p className="meta-note">
              {firstBlockedFeature.blockers?.[0]?.message ?? areaSummarySignal}
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

        {roleOptions.notice ? (
          <LookupNotice
            title="Role references unavailable"
            message={roleOptions.notice.message}
            retryLabel="Retry role lookup"
            retryDisabled={roleOptions.loading}
            onRetry={roleOptions.refresh}
          />
        ) : null}
      </SurfaceCard>
    </aside>
  );
}

interface ModerationFeatureDrawerProps {
  selectedFeature: FeatureRecord;
  selectedIsMuteRole: boolean;
  mutationNotice: ReturnType<typeof useFeatureMutation>["notice"];
  mutationSaving: boolean;
  pendingFeatureId: string;
  roleDraft: string;
  setRoleDraft: (value: string) => void;
  muteRoleOptions: Array<{
    value: string;
    label: string;
    disabled?: boolean;
  }>;
  roleOptions: ReturnType<typeof useGuildRoleOptions>;
  channelDraft: string;
  setChannelDraft: (value: string) => void;
  channelOptions: ReturnType<typeof useGuildChannelOptions>;
  messageRouteChannelOptions: Array<{
    value: string;
    label: string;
    description?: string;
  }>;
  onSave: () => Promise<void>;
  onClose: () => void;
}

function ModerationFeatureDrawer({
  selectedFeature,
  selectedIsMuteRole,
  mutationNotice,
  mutationSaving,
  pendingFeatureId,
  roleDraft,
  setRoleDraft,
  muteRoleOptions,
  roleOptions,
  channelDraft,
  setChannelDraft,
  channelOptions,
  messageRouteChannelOptions,
  onSave,
  onClose,
}: ModerationFeatureDrawerProps) {
  return (
    <div className="drawer-backdrop" onClick={onClose} role="presentation">
      <aside
        aria-label={`Configure ${selectedFeature.label}`}
        aria-modal="true"
        className="drawer-panel moderation-drawer"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
      >
        <div className="card-copy">
          <p className="section-label">
            {selectedIsMuteRole ? "Role-based mute" : "Moderation logging"}
          </p>
          <div className="logging-drawer-title-row">
            <h2>{selectedFeature.label}</h2>
            <StatusBadge tone={getFeatureStatusTone(selectedFeature)}>
              {formatFeatureStatusLabel(selectedFeature)}
            </StatusBadge>
          </div>
          <p className="section-description">{selectedFeature.description}</p>
        </div>

        {mutationNotice ? <AlertBanner notice={mutationNotice} /> : null}

        {selectedIsMuteRole ? (
          <MuteRoleDrawerBody
            selectedFeature={selectedFeature}
            roleDraft={roleDraft}
            setRoleDraft={setRoleDraft}
            muteRoleOptions={muteRoleOptions}
            roleOptions={roleOptions}
          />
        ) : (
          <ModerationDestinationDrawerBody
            selectedFeature={selectedFeature}
            channelDraft={channelDraft}
            setChannelDraft={setChannelDraft}
            channelOptions={channelOptions}
            messageRouteChannelOptions={messageRouteChannelOptions}
          />
        )}

        <div className="drawer-actions">
          <button
            className="button-primary"
            type="button"
            disabled={mutationSaving}
            onClick={() => void onSave()}
          >
            {mutationSaving && pendingFeatureId === selectedFeature.id
              ? "Saving..."
              : selectedIsMuteRole
                ? "Save mute role"
                : "Save destination"}
          </button>
          <button className="button-secondary" type="button" onClick={onClose}>
            Cancel
          </button>
        </div>
      </aside>
    </div>
  );
}

interface MuteRoleDrawerBodyProps {
  selectedFeature: FeatureRecord;
  roleDraft: string;
  setRoleDraft: (value: string) => void;
  muteRoleOptions: Array<{
    value: string;
    label: string;
    disabled?: boolean;
  }>;
  roleOptions: ReturnType<typeof useGuildRoleOptions>;
}

function MuteRoleDrawerBody({
  selectedFeature,
  roleDraft,
  setRoleDraft,
  muteRoleOptions,
  roleOptions,
}: MuteRoleDrawerBodyProps) {
  return (
    <>
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
            label: "Current role",
            value: formatRoleValue(roleDraft, roleOptions.roles),
          },
          {
            label: "Current signal",
            value: summarizeMuteRoleSignal(selectedFeature),
          },
        ]}
      />

      <EntityPickerField
        label="Mute role"
        value={roleDraft}
        disabled={roleOptions.loading}
        onChange={setRoleDraft}
        options={muteRoleOptions}
        placeholder={
          roleOptions.loading
            ? "Loading roles..."
            : muteRoleOptions.length === 0
              ? "No roles available"
              : "No mute role"
        }
        note="Choose a non-managed role that the bot can assign and remove."
      />

      {roleOptions.notice ? (
        <LookupNotice
          title="Role references unavailable"
          message={roleOptions.notice.message}
          retryLabel="Retry role lookup"
          retryDisabled={roleOptions.loading}
          onRetry={roleOptions.refresh}
        />
      ) : null}

      <AdvancedTextInput
        label="Mute role ID fallback"
        inputLabel="Mute role ID fallback"
        value={roleDraft}
        onChange={setRoleDraft}
        placeholder="Discord role ID"
        note="Use this only when the role picker is unavailable or you need to paste a role ID directly."
      />

      <div className="surface-subsection">
        <p className="section-label">Requirements</p>
        <ul className="feature-guidance-list">
          <li>Use a dedicated mute role instead of @everyone.</li>
          <li>
            The mute role must stay below both the moderator and bot highest
            roles.
          </li>
          <li>
            Avoid managed integration roles because Discord will reject manual
            assignment.
          </li>
        </ul>
      </div>
    </>
  );
}

interface ModerationDestinationDrawerBodyProps {
  selectedFeature: FeatureRecord;
  channelDraft: string;
  setChannelDraft: (value: string) => void;
  channelOptions: ReturnType<typeof useGuildChannelOptions>;
  messageRouteChannelOptions: Array<{
    value: string;
    label: string;
    description?: string;
  }>;
}

function ModerationDestinationDrawerBody({
  selectedFeature,
  channelDraft,
  setChannelDraft,
  channelOptions,
  messageRouteChannelOptions,
}: ModerationDestinationDrawerBodyProps) {
  return (
    <>
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
            value: getLoggingFeatureDetails(selectedFeature).requiresChannel
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
        note="Use this only when the channel picker is unavailable or when you need to paste a channel ID directly."
      />

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
          <p className="section-label">Runtime dependency</p>
          <p className="meta-note">
            This route depends on runtime conditions reported by the control
            server. Review the blocker message before saving.
          </p>
        </div>
      ) : null}
    </>
  );
}
