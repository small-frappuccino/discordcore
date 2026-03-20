import { useMemo, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import type { FeatureRecord, GuildRoleOption } from "../api/control";
import { appRoutes } from "../app/routes";
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
import { getFeatureAreaDefinition, getFeatureAreaRecords } from "../features/features/areas";
import {
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import {
  canEditAutoRole,
  canEditPermissionMirror,
  canEditPresenceWatchBot,
  canEditPresenceWatchUser,
  countEnabledFeatures,
  formatRequirementRolesValue,
  formatRoleOptionLabel,
  formatRoleValue,
  getAutoRoleFeatureDetails,
  getPermissionMirrorDetails,
  getPresenceWatchBotDetails,
  getPresenceWatchUserDetails,
  summarizeAdvancedRoleSignal,
  summarizeAutoRoleSignal,
} from "../features/features/roles";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";

type RolesDrawerKind =
  | "auto_role_assignment"
  | "presence_watch.bot"
  | "presence_watch.user"
  | "safety.bot_role_perm_mirror";

interface RolesDrawerState {
  featureId: RolesDrawerKind;
}

export function RolesPage() {
  const definition = getFeatureAreaDefinition("roles");
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
  const roleOptions = useGuildRoleOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");
  const [drawerState, setDrawerState] = useState<RolesDrawerState | null>(null);
  const [configEnabledDraft, setConfigEnabledDraft] = useState("enabled");
  const [targetRoleDraft, setTargetRoleDraft] = useState("");
  const [levelRoleDraft, setLevelRoleDraft] = useState("");
  const [boosterRoleDraft, setBoosterRoleDraft] = useState("");
  const [watchBotDraft, setWatchBotDraft] = useState("enabled");
  const [userIdDraft, setUserIdDraft] = useState("");
  const [actorRoleDraft, setActorRoleDraft] = useState("");

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "roles");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const workspaceNotice = mutation.notice ?? roleOptions.notice ?? workspace.notice;
  const autoRoleFeature =
    areaFeatures.find((feature) => feature.id === "auto_role_assignment") ?? null;
  const autoRoleDetails =
    autoRoleFeature === null ? null : getAutoRoleFeatureDetails(autoRoleFeature);
  const advancedFeatures = areaFeatures.filter(
    (feature) => feature.id !== "auto_role_assignment",
  );
  const advancedEnabledCount = countEnabledFeatures(advancedFeatures);
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;
  const firstBlockedFeature = useMemo(
    () => areaFeatures.find((feature) => feature.readiness === "blocked") ?? null,
    [areaFeatures],
  );
  const selectedFeature =
    drawerState === null
      ? null
      : areaFeatures.find((feature) => feature.id === drawerState.featureId) ?? null;
  const rolePickerUnavailable = roleOptions.notice !== null || roleOptions.roles.length === 0;

  async function handleRefreshRoles() {
    await Promise.all([workspace.refresh(), roleOptions.refresh()]);
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

  async function handleUseDefaultState(feature: FeatureRecord) {
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
    mutation.clearNotice();

    switch (feature.id) {
      case "auto_role_assignment": {
        const details = getAutoRoleFeatureDetails(feature);
        setConfigEnabledDraft(details.configEnabled ? "enabled" : "disabled");
        setTargetRoleDraft(details.targetRoleId);
        setLevelRoleDraft(details.levelRoleId);
        setBoosterRoleDraft(details.boosterRoleId);
        setDrawerState({
          featureId: feature.id,
        });
        return;
      }
      case "presence_watch.bot": {
        const details = getPresenceWatchBotDetails(feature);
        setWatchBotDraft(details.watchBot ? "enabled" : "disabled");
        setDrawerState({
          featureId: feature.id,
        });
        return;
      }
      case "presence_watch.user": {
        const details = getPresenceWatchUserDetails(feature);
        setUserIdDraft(details.userId);
        setDrawerState({
          featureId: feature.id,
        });
        return;
      }
      case "safety.bot_role_perm_mirror": {
        const details = getPermissionMirrorDetails(feature);
        setActorRoleDraft(details.actorRoleId);
        setDrawerState({
          featureId: feature.id,
        });
        return;
      }
      default:
        return;
    }
  }

  function closeDrawer() {
    setDrawerState(null);
    setConfigEnabledDraft("enabled");
    setTargetRoleDraft("");
    setLevelRoleDraft("");
    setBoosterRoleDraft("");
    setWatchBotDraft("enabled");
    setUserIdDraft("");
    setActorRoleDraft("");
    mutation.clearNotice();
  }

  async function handleSaveAutoRole() {
    if (selectedFeature?.id !== "auto_role_assignment") {
      return;
    }

    setPendingFeatureId(selectedFeature.id);

    try {
      const updated = await mutation.patchFeature(selectedFeature.id, {
        config_enabled: configEnabledDraft === "enabled",
        target_role_id: targetRoleDraft,
        required_role_ids: [levelRoleDraft, boosterRoleDraft].filter(
          (value) => value.trim() !== "",
        ),
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSavePresenceWatchBot() {
    if (selectedFeature?.id !== "presence_watch.bot") {
      return;
    }

    setPendingFeatureId(selectedFeature.id);

    try {
      const updated = await mutation.patchFeature(selectedFeature.id, {
        watch_bot: watchBotDraft === "enabled",
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSavePresenceWatchUser() {
    if (selectedFeature?.id !== "presence_watch.user") {
      return;
    }

    setPendingFeatureId(selectedFeature.id);

    try {
      const updated = await mutation.patchFeature(selectedFeature.id, {
        user_id: userIdDraft.trim(),
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSavePermissionMirror() {
    if (selectedFeature?.id !== "safety.bot_role_perm_mirror") {
      return;
    }

    setPendingFeatureId(selectedFeature.id);

    try {
      const updated = await mutation.patchFeature(selectedFeature.id, {
        actor_role_id: actorRoleDraft,
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
        disabled={workspace.loading || roleOptions.loading || mutation.saving}
        onClick={() => void handleRefreshRoles()}
      >
        Refresh roles
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
                onClick={() => void handleRefreshRoles()}
              >
                Retry loading
              </button>
            ) : undefined
          }
        />
      );
    }

    if (autoRoleFeature === null || autoRoleDetails === null) {
      return (
        <div className="table-empty-state table-empty-state-compact">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>No role controls yet</h2>
            <p className="section-description">
              The selected server does not expose role controls in this workspace
              yet.
            </p>
          </div>
        </div>
      );
    }

    return (
      <>
        <div className="workspace-callout roles-primary-callout">
          <div className="workspace-callout-copy">
            <div className="card-copy">
              <p className="section-label">Primary workflow</p>
              <h2>Automatic role assignment</h2>
              <p className="section-description">
                Configure the role that should be assigned automatically and the
                two role requirements that gate the assignment flow.
              </p>
            </div>

            <div className="roles-primary-status">
              <StatusBadge tone={getFeatureStatusTone(autoRoleFeature)}>
                {formatFeatureStatusLabel(autoRoleFeature)}
              </StatusBadge>
              <span className="meta-note">
                {summarizeAutoRoleSignal(autoRoleFeature)}
              </span>
            </div>

            <div className="feature-row-actions roles-primary-actions">
              <button
                className="button-secondary"
                type="button"
                disabled={roleOptions.loading || rolePickerUnavailable || !canEditAutoRole(autoRoleFeature)}
                onClick={() => openDrawer(autoRoleFeature)}
              >
                Configure auto role
              </button>
              <button
                className="button-ghost"
                type="button"
                disabled={mutation.saving}
                aria-label={`${autoRoleFeature.effective_enabled ? "Disable" : "Enable"} ${autoRoleFeature.label}`}
                onClick={() =>
                  void handleSetFeatureEnabled(
                    autoRoleFeature,
                    !autoRoleFeature.effective_enabled,
                  )
                }
              >
                {mutation.saving && pendingFeatureId === autoRoleFeature.id
                  ? "Saving..."
                  : autoRoleFeature.effective_enabled
                    ? "Disable"
                    : "Enable"}
              </button>
              {autoRoleFeature.override_state !== "inherit" ? (
                <button
                  className="button-ghost"
                  type="button"
                  disabled={mutation.saving}
                  onClick={() => void handleUseDefaultState(autoRoleFeature)}
                >
                  Use default
                </button>
              ) : null}
            </div>
          </div>

          <KeyValueList
            className="workspace-status-list"
            items={[
              {
                label: "Module state",
                value: autoRoleFeature.effective_enabled ? "On" : "Off",
              },
              {
                label: "Assignment rule",
                value: autoRoleDetails.configEnabled ? "Enabled" : "Disabled",
              },
              {
                label: "Target role",
                value: formatRoleValue(
                  autoRoleDetails.targetRoleId,
                  roleOptions.roles,
                ),
              },
              {
                label: "Level role",
                value: formatRoleValue(
                  autoRoleDetails.levelRoleId,
                  roleOptions.roles,
                ),
              },
              {
                label: "Booster role",
                value: formatRoleValue(
                  autoRoleDetails.boosterRoleId,
                  roleOptions.roles,
                ),
              },
            ]}
          />
        </div>

        {roleOptions.notice ? (
          <div className="surface-subsection">
            <p className="section-label">Role lookup unavailable</p>
            <p className="meta-note">
              The dashboard could not load server roles right now. Refresh roles
              before opening role-based editors.
            </p>
          </div>
        ) : null}

        {autoRoleFeature.readiness === "blocked" ? (
          <div className="surface-subsection">
            <p className="section-label">Needs setup</p>
            <strong>{summarizeAutoRoleSignal(autoRoleFeature)}</strong>
            <p className="meta-note">
              Start by choosing the target role, then confirm the level and
              booster requirements.
            </p>
          </div>
        ) : null}

        <details className="details-panel roles-advanced-details">
          <summary>
            <span>Advanced controls</span>
            <span className="meta-pill subtle-pill">
              {advancedEnabledCount}/{advancedFeatures.length} active
            </span>
          </summary>

          <div className="details-content roles-advanced-content">
            <p>
              Keep the default workspace focused on automatic role assignment.
              Open these controls only when you need presence watching or the
              permission mirror guard.
            </p>

            <div className="roles-advanced-list">
              {advancedFeatures.map((feature) => {
                const isPending = mutation.saving && pendingFeatureId === feature.id;
                const canOpenDrawer =
                  feature.id === "presence_watch.user" ||
                  feature.id === "presence_watch.bot" ||
                  (feature.id === "safety.bot_role_perm_mirror" &&
                    !roleOptions.loading &&
                    !rolePickerUnavailable);

                return (
                  <div className="roles-advanced-row" key={feature.id}>
                    <div className="roles-advanced-copy">
                      <div className="roles-advanced-title-row">
                        <strong>{feature.label}</strong>
                        <StatusBadge tone={getFeatureStatusTone(feature)}>
                          {formatFeatureStatusLabel(feature)}
                        </StatusBadge>
                      </div>
                      <p>{feature.description}</p>
                      <span className="meta-note">
                        {summarizeAdvancedRoleSignal(feature)}
                      </span>
                    </div>

                    <div className="feature-row-actions roles-advanced-actions">
                      {canOpenDrawer ? (
                        <button
                          className="button-secondary"
                          type="button"
                          disabled={
                            (feature.id === "presence_watch.bot" &&
                              !canEditPresenceWatchBot(feature)) ||
                            (feature.id === "presence_watch.user" &&
                              !canEditPresenceWatchUser(feature)) ||
                            (feature.id === "safety.bot_role_perm_mirror" &&
                              !canEditPermissionMirror(feature))
                          }
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
                          onClick={() => void handleUseDefaultState(feature)}
                        >
                          Use default
                        </button>
                      ) : null}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </details>
      </>
    );
  }

  return (
    <>
      <section className="page-shell">
        <PageHeader
          eyebrow="Feature area"
          title={areaLabel}
          description="Configure automatic role assignment for the selected server and keep advanced role-aware controls available without turning this page into a diagnostics workspace."
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
          meta={
            <>
              <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
              <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
            </>
          }
          actions={renderHeaderActions()}
        />

        {workspace.workspaceState === "ready" && autoRoleFeature !== null && autoRoleDetails !== null ? (
          <section className="overview-summary-strip" aria-label="Roles summary">
            <MetricCard
              label="Auto role"
              value={formatFeatureStatusLabel(autoRoleFeature)}
              description={summarizeAutoRoleSignal(autoRoleFeature)}
              tone={getFeatureStatusTone(autoRoleFeature)}
            />
            <MetricCard
              label="Target role"
              value={formatRoleValue(autoRoleDetails.targetRoleId, roleOptions.roles)}
              description="The role that gets assigned automatically."
            />
            <MetricCard
              label="Requirement roles"
              value={formatRequirementRolesValue(autoRoleFeature, roleOptions.roles)}
              description={
                autoRoleDetails.requiredRoleCount === 2
                  ? "Both requirement roles are configured."
                  : "Choose the level and booster roles that gate the assignment."
              }
              tone={
                autoRoleDetails.requiredRoleCount === 2 ? "success" : "neutral"
              }
            />
            <MetricCard
              label="Advanced controls"
              value={`${advancedEnabledCount}/${advancedFeatures.length}`}
              description="Presence watching and permission mirror controls kept behind disclosure."
              tone={advancedEnabledCount > 0 ? "info" : "neutral"}
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
                    <h2>Manage roles</h2>
                    <p className="section-description">
                      Keep the main workspace centered on auto role assignment.
                      Advanced controls stay available, but they do not compete
                      with the primary setup flow.
                    </p>
                  </div>
                  <div className="workspace-view-meta">
                    {workspace.workspaceState === "ready" ? (
                      <>
                        <span className="meta-pill subtle-pill">
                          {localOverrides} local overrides
                        </span>
                        <span className="meta-pill subtle-pill">
                          {advancedEnabledCount} advanced controls active
                        </span>
                      </>
                    ) : null}
                  </div>
                </div>

                <AlertBanner
                  notice={workspaceNotice}
                  busyLabel={
                    mutation.saving
                      ? "Saving role settings..."
                      : workspace.loading || roleOptions.loading
                        ? "Refreshing roles workspace..."
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
                <h2>Current role setup</h2>
                <p className="section-description">
                  Use this panel to confirm the selected server, the current auto
                  role target, and whether advanced controls are active.
                </p>
              </div>

              <KeyValueList
                items={[
                  {
                    label: "Server",
                    value: selectedServerLabel,
                  },
                  {
                    label: "Target role",
                    value:
                      autoRoleDetails === null
                        ? "Not available"
                        : formatRoleValue(autoRoleDetails.targetRoleId, roleOptions.roles),
                  },
                  {
                    label: "Requirement roles",
                    value:
                      autoRoleFeature === null
                        ? "Not available"
                        : formatRequirementRolesValue(autoRoleFeature, roleOptions.roles),
                  },
                  {
                    label: "Current signal",
                    value:
                      firstBlockedFeature === null
                        ? areaSummary.signal
                        : firstBlockedFeature.id === "auto_role_assignment"
                          ? summarizeAutoRoleSignal(firstBlockedFeature)
                          : summarizeAdvancedRoleSignal(firstBlockedFeature),
                  },
                ]}
              />
            </SurfaceCard>

            <SurfaceCard>
              <div className="card-copy">
                <p className="section-label">Guidance</p>
                <h2>How this page works</h2>
                <p className="section-description">
                  The default view focuses on the task most admins come here to
                  complete: automatic role assignment.
                </p>
              </div>

              <ul className="feature-guidance-list">
                <li>Choose the target role first, then set the level and booster requirements.</li>
                <li>Use advanced controls only when you need presence watching or the permission mirror guard.</li>
                <li>Runtime issues stay in Settings diagnostics instead of taking over the main roles workspace.</li>
              </ul>

              {firstBlockedFeature?.id === "safety.bot_role_perm_mirror" ? (
                <div className="surface-subsection">
                  <p className="section-label">Needs diagnostics</p>
                  <p className="meta-note">
                    Permission mirror blockers that come from runtime state are
                    reviewed in Settings diagnostics.
                  </p>
                  <div className="sidebar-actions">
                    <Link className="button-secondary" to={`${appRoutes.settings}#diagnostics`}>
                      Open Settings diagnostics
                    </Link>
                  </div>
                </div>
              ) : null}
            </SurfaceCard>
          </aside>
        </section>
      </section>

      {selectedFeature !== null ? (
        <div className="drawer-backdrop" onClick={closeDrawer} role="presentation">
          <aside
            aria-label={getDrawerLabel(selectedFeature)}
            aria-modal="true"
            className="drawer-panel roles-drawer"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="card-copy">
              <p className="section-label">Roles</p>
              <div className="logging-drawer-title-row">
                <h2>{selectedFeature.label}</h2>
                <StatusBadge tone={getFeatureStatusTone(selectedFeature)}>
                  {formatFeatureStatusLabel(selectedFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">{selectedFeature.description}</p>
            </div>

            {mutation.notice ? <AlertBanner notice={mutation.notice} /> : null}

            {renderDrawerBody({
              selectedFeature,
              pendingFeatureId,
              mutationSaving: mutation.saving,
              roleOptions: roleOptions.roles,
              configEnabledDraft,
              targetRoleDraft,
              levelRoleDraft,
              boosterRoleDraft,
              watchBotDraft,
              userIdDraft,
              actorRoleDraft,
              setConfigEnabledDraft,
              setTargetRoleDraft,
              setLevelRoleDraft,
              setBoosterRoleDraft,
              setWatchBotDraft,
              setUserIdDraft,
              setActorRoleDraft,
              closeDrawer,
              handleSaveAutoRole,
              handleSavePresenceWatchBot,
              handleSavePresenceWatchUser,
              handleSavePermissionMirror,
            })}
          </aside>
        </div>
      ) : null}
    </>
  );
}

interface RenderDrawerBodyProps {
  selectedFeature: FeatureRecord;
  pendingFeatureId: string;
  mutationSaving: boolean;
  roleOptions: GuildRoleOption[];
  configEnabledDraft: string;
  targetRoleDraft: string;
  levelRoleDraft: string;
  boosterRoleDraft: string;
  watchBotDraft: string;
  userIdDraft: string;
  actorRoleDraft: string;
  setConfigEnabledDraft: (value: string) => void;
  setTargetRoleDraft: (value: string) => void;
  setLevelRoleDraft: (value: string) => void;
  setBoosterRoleDraft: (value: string) => void;
  setWatchBotDraft: (value: string) => void;
  setUserIdDraft: (value: string) => void;
  setActorRoleDraft: (value: string) => void;
  closeDrawer: () => void;
  handleSaveAutoRole: () => Promise<void>;
  handleSavePresenceWatchBot: () => Promise<void>;
  handleSavePresenceWatchUser: () => Promise<void>;
  handleSavePermissionMirror: () => Promise<void>;
}

function renderDrawerBody({
  selectedFeature,
  pendingFeatureId,
  mutationSaving,
  roleOptions,
  configEnabledDraft,
  targetRoleDraft,
  levelRoleDraft,
  boosterRoleDraft,
  watchBotDraft,
  userIdDraft,
  actorRoleDraft,
  setConfigEnabledDraft,
  setTargetRoleDraft,
  setLevelRoleDraft,
  setBoosterRoleDraft,
  setWatchBotDraft,
  setUserIdDraft,
  setActorRoleDraft,
  closeDrawer,
  handleSaveAutoRole,
  handleSavePresenceWatchBot,
  handleSavePresenceWatchUser,
  handleSavePermissionMirror,
}: RenderDrawerBodyProps) {
  if (selectedFeature.id === "auto_role_assignment") {
    return (
      <>
        <KeyValueList
          items={[
            {
              label: "Module state",
              value: selectedFeature.effective_enabled ? "On" : "Off",
            },
            {
              label: "Current signal",
              value: summarizeAutoRoleSignal(selectedFeature),
            },
            {
              label: "Server setting",
              value:
                selectedFeature.override_state === "inherit"
                  ? "Using default"
                  : "Configured here",
            },
          ]}
        />

        <div className="field-grid roles-form-grid">
          <label className="field-stack">
            <span className="field-label">Assignment rule</span>
            <select
              aria-label="Assignment rule"
              value={configEnabledDraft}
              onChange={(event) => setConfigEnabledDraft(event.target.value)}
            >
              <option value="enabled">Enabled</option>
              <option value="disabled">Disabled</option>
            </select>
            <span className="meta-note">
              Keep the module enabled but pause the assignment rule when you need
              to preserve the rest of the setup.
            </span>
          </label>

          <label className="field-stack">
            <span className="field-label">Target role</span>
            <select
              aria-label="Target role"
              value={targetRoleDraft}
              onChange={(event) => setTargetRoleDraft(event.target.value)}
            >
              <option value="">Select a target role</option>
              {roleOptions.map((role) => (
                <option key={role.id} value={role.id}>
                  {formatRoleOptionLabel(role)}
                </option>
              ))}
            </select>
          </label>

          <label className="field-stack">
            <span className="field-label">Level role</span>
            <select
              aria-label="Level role"
              value={levelRoleDraft}
              onChange={(event) => setLevelRoleDraft(event.target.value)}
            >
              <option value="">Select the level role</option>
              {roleOptions.map((role) => (
                <option key={role.id} value={role.id}>
                  {formatRoleOptionLabel(role)}
                </option>
              ))}
            </select>
          </label>

          <label className="field-stack">
            <span className="field-label">Booster role</span>
            <select
              aria-label="Booster role"
              value={boosterRoleDraft}
              onChange={(event) => setBoosterRoleDraft(event.target.value)}
            >
              <option value="">Select the booster role</option>
              {roleOptions.map((role) => (
                <option key={role.id} value={role.id}>
                  {formatRoleOptionLabel(role)}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="surface-subsection">
          <p className="section-label">What to configure</p>
          <ul className="feature-guidance-list">
            <li>Target role: the role granted automatically.</li>
            <li>Level role: the first requirement checked before assignment.</li>
            <li>Booster role: the second requirement that keeps the setup valid.</li>
          </ul>
        </div>

        <div className="drawer-actions">
          <button
            className="button-primary"
            type="button"
            disabled={mutationSaving}
            onClick={() => void handleSaveAutoRole()}
          >
            {mutationSaving && pendingFeatureId === selectedFeature.id
              ? "Saving..."
              : "Save auto role"}
          </button>
          <button className="button-secondary" type="button" onClick={closeDrawer}>
            Cancel
          </button>
        </div>
      </>
    );
  }

  if (selectedFeature.id === "presence_watch.bot") {
    return (
      <>
        <KeyValueList
          items={[
            {
              label: "Module state",
              value: selectedFeature.effective_enabled ? "On" : "Off",
            },
            {
              label: "Current signal",
              value: summarizeAdvancedRoleSignal(selectedFeature),
            },
          ]}
        />

        <label className="field-stack">
          <span className="field-label">Watch bot presence</span>
          <select
            aria-label="Watch bot presence"
            value={watchBotDraft}
            onChange={(event) => setWatchBotDraft(event.target.value)}
          >
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>
          <span className="meta-note">
            This advanced flag controls whether the runtime watches the bot
            identity itself.
          </span>
        </label>

        <div className="drawer-actions">
          <button
            className="button-primary"
            type="button"
            disabled={mutationSaving}
            onClick={() => void handleSavePresenceWatchBot()}
          >
            {mutationSaving && pendingFeatureId === selectedFeature.id
              ? "Saving..."
              : "Save presence watch"}
          </button>
          <button className="button-secondary" type="button" onClick={closeDrawer}>
            Cancel
          </button>
        </div>
      </>
    );
  }

  if (selectedFeature.id === "presence_watch.user") {
    return (
      <>
        <KeyValueList
          items={[
            {
              label: "Module state",
              value: selectedFeature.effective_enabled ? "On" : "Off",
            },
            {
              label: "Current signal",
              value: summarizeAdvancedRoleSignal(selectedFeature),
            },
          ]}
        />

        <label className="field-stack">
          <span className="field-label">User ID</span>
          <input
            aria-label="User ID"
            value={userIdDraft}
            onChange={(event) => setUserIdDraft(event.target.value)}
            placeholder="Discord user ID"
          />
          <span className="meta-note">
            This stays advanced for now. The future version can replace this
            field with a member picker.
          </span>
        </label>

        <div className="drawer-actions">
          <button
            className="button-primary"
            type="button"
            disabled={mutationSaving}
            onClick={() => void handleSavePresenceWatchUser()}
          >
            {mutationSaving && pendingFeatureId === selectedFeature.id
              ? "Saving..."
              : "Save user watch"}
          </button>
          <button className="button-secondary" type="button" onClick={closeDrawer}>
            Cancel
          </button>
        </div>
      </>
    );
  }

  const permissionMirrorDetails = getPermissionMirrorDetails(selectedFeature);

  return (
    <>
      <KeyValueList
        items={[
          {
            label: "Module state",
            value: selectedFeature.effective_enabled ? "On" : "Off",
          },
          {
            label: "Current signal",
            value: summarizeAdvancedRoleSignal(selectedFeature),
          },
          {
            label: "Current actor role",
            value: formatRoleValue(
              permissionMirrorDetails.actorRoleId,
              roleOptions,
              "No guard role",
            ),
          },
        ]}
      />

      <label className="field-stack">
        <span className="field-label">Actor role</span>
        <select
          aria-label="Actor role"
          value={actorRoleDraft}
          onChange={(event) => setActorRoleDraft(event.target.value)}
        >
          <option value="">No guard role</option>
          {roleOptions.map((role) => (
            <option key={role.id} value={role.id}>
              {formatRoleOptionLabel(role)}
            </option>
          ))}
        </select>
        <span className="meta-note">
          Use a guard role only when permission mirror changes should stay scoped
          to a specific operator role.
        </span>
      </label>

      {selectedFeature.blockers?.some(
        (blocker) => blocker.code === "runtime_kill_switch",
      ) ? (
        <div className="surface-subsection">
          <p className="section-label">Needs diagnostics</p>
          <p className="meta-note">
            Runtime permission mirror settings are reviewed in Settings
            diagnostics.
          </p>
          <div className="sidebar-actions">
            <Link className="button-secondary" to={`${appRoutes.settings}#diagnostics`}>
              Open Settings diagnostics
            </Link>
          </div>
        </div>
      ) : null}

      <div className="drawer-actions">
        <button
          className="button-primary"
          type="button"
          disabled={mutationSaving}
          onClick={() => void handleSavePermissionMirror()}
        >
          {mutationSaving && pendingFeatureId === selectedFeature.id
            ? "Saving..."
            : "Save guard role"}
        </button>
        <button className="button-secondary" type="button" onClick={closeDrawer}>
          Cancel
        </button>
      </div>
    </>
  );
}

function getDrawerLabel(feature: FeatureRecord) {
  switch (feature.id) {
    case "auto_role_assignment":
      return "Configure automatic role assignment";
    case "presence_watch.bot":
      return "Configure bot presence watch";
    case "presence_watch.user":
      return "Configure user presence watch";
    case "safety.bot_role_perm_mirror":
      return "Configure permission mirror";
    default:
      return `Configure ${feature.label}`;
  }
}
