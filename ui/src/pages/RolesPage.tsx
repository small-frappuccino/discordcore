import { useState } from "react";
import { useLocation } from "react-router-dom";
import type {
  FeatureRecord,
  GuildMemberOption,
  GuildRoleOption,
} from "../api/control";
import {
  AlertBanner,
  EmptyState,
  KeyValueList,
  LookupNotice,
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
  formatMemberOptionLabel,
  formatMemberValue,
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
import { useGuildMemberOptions } from "../features/features/useGuildMemberOptions";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";

type RolesDrawerKind =
  | "auto_role_assignment"
  | "presence_watch.bot"
  | "presence_watch.user"
  | "safety.bot_role_perm_mirror";

interface RolesDrawerState {
  featureId: RolesDrawerKind;
}

type RolesWorkspaceState = ReturnType<typeof useFeatureWorkspace>["workspaceState"];
type RolesAreaSummary = ReturnType<typeof summarizeFeatureArea>;
type AutoRoleDetails = ReturnType<typeof getAutoRoleFeatureDetails>;
type PermissionMirrorDetails = ReturnType<typeof getPermissionMirrorDetails>;
type DashboardNotice = {
  tone: "info" | "success" | "error";
  message: string;
};

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
  const [memberSearchDraft, setMemberSearchDraft] = useState("");
  const [actorRoleDraft, setActorRoleDraft] = useState("");
  const memberOptions = useGuildMemberOptions({
    enabled: drawerState?.featureId === "presence_watch.user",
    query: memberSearchDraft,
    selectedMemberId:
      drawerState?.featureId === "presence_watch.user" ? userIdDraft : "",
  });

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
  const firstBlockedFeature =
    areaFeatures.find((feature) => feature.readiness === "blocked") ?? null;
  const selectedFeature =
    drawerState === null
      ? null
      : areaFeatures.find((feature) => feature.id === drawerState.featureId) ?? null;
  const rolePickerUnavailable = roleOptions.notice !== null || roleOptions.roles.length === 0;

  async function handleRefreshRoles() {
    await Promise.all([
      workspace.refresh(),
      roleOptions.refresh(),
      memberOptions.refresh(),
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
        setMemberSearchDraft("");
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
    setMemberSearchDraft("");
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

  const headerActions =
    authState !== "signed_in" ? (
      <button
        className="button-primary"
        type="button"
        onClick={() => void beginLogin(nextPath)}
      >
        Sign in with Discord
      </button>
    ) : selectedGuild === null ? null : (
      <button
        className="button-secondary"
        type="button"
        disabled={workspace.loading || roleOptions.loading || mutation.saving}
        onClick={() => void handleRefreshRoles()}
      >
        Refresh roles
      </button>
    );

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
          actions={headerActions}
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

                <RolesWorkspaceContent
                  areaLabel={areaLabel}
                  authState={authState}
                  autoRoleDetails={autoRoleDetails}
                  autoRoleFeature={autoRoleFeature}
                  advancedEnabledCount={advancedEnabledCount}
                  advancedFeatures={advancedFeatures}
                  pendingFeatureId={pendingFeatureId}
                  roleOptions={roleOptions.roles}
                  roleLookupNotice={roleOptions.notice}
                  rolePickerUnavailable={rolePickerUnavailable}
                  roleOptionsLoading={roleOptions.loading}
                  workspaceState={workspace.workspaceState}
                  mutationSaving={mutation.saving}
                  onLogin={() => void beginLogin(nextPath)}
                  onOpenDrawer={openDrawer}
                  onRefresh={() => void handleRefreshRoles()}
                  onSetFeatureEnabled={(feature, enabled) => {
                    void handleSetFeatureEnabled(feature, enabled);
                  }}
                  onUseDefaultState={(feature) => {
                    void handleUseDefaultState(feature);
                  }}
                />
              </div>
            </SurfaceCard>
          </div>

          <RolesAside
            areaSummary={areaSummary}
            autoRoleDetails={autoRoleDetails}
            autoRoleFeature={autoRoleFeature}
            firstBlockedFeature={firstBlockedFeature}
            roleOptions={roleOptions.roles}
            selectedServerLabel={selectedServerLabel}
          />
        </section>
      </section>

      <RolesFeatureDrawer
        actorRoleDraft={actorRoleDraft}
        boosterRoleDraft={boosterRoleDraft}
        closeDrawer={closeDrawer}
        configEnabledDraft={configEnabledDraft}
        levelRoleDraft={levelRoleDraft}
        memberLookupLoading={memberOptions.loading}
        memberLookupNotice={memberOptions.notice}
        memberOptions={memberOptions.members}
        memberSearchDraft={memberSearchDraft}
        mutationNotice={mutation.notice}
        mutationSaving={mutation.saving}
        pendingFeatureId={pendingFeatureId}
        roleOptions={roleOptions.roles}
        selectedFeature={selectedFeature}
        targetRoleDraft={targetRoleDraft}
        userIdDraft={userIdDraft}
        watchBotDraft={watchBotDraft}
        refreshMemberOptions={memberOptions.refresh}
        setActorRoleDraft={setActorRoleDraft}
        setBoosterRoleDraft={setBoosterRoleDraft}
        setConfigEnabledDraft={setConfigEnabledDraft}
        setLevelRoleDraft={setLevelRoleDraft}
        setMemberSearchDraft={setMemberSearchDraft}
        setTargetRoleDraft={setTargetRoleDraft}
        setUserIdDraft={setUserIdDraft}
        setWatchBotDraft={setWatchBotDraft}
        onSaveAutoRole={() => void handleSaveAutoRole()}
        onSavePermissionMirror={() => void handleSavePermissionMirror()}
        onSavePresenceWatchBot={() => void handleSavePresenceWatchBot()}
        onSavePresenceWatchUser={() => void handleSavePresenceWatchUser()}
      />
    </>
  );
}

interface RolesWorkspaceContentProps {
  areaLabel: string;
  authState: string;
  autoRoleDetails: AutoRoleDetails | null;
  autoRoleFeature: FeatureRecord | null;
  advancedEnabledCount: number;
  advancedFeatures: FeatureRecord[];
  pendingFeatureId: string;
  roleOptions: GuildRoleOption[];
  roleLookupNotice: DashboardNotice | null;
  rolePickerUnavailable: boolean;
  roleOptionsLoading: boolean;
  workspaceState: RolesWorkspaceState;
  mutationSaving: boolean;
  onLogin: () => void;
  onOpenDrawer: (feature: FeatureRecord) => void;
  onRefresh: () => void;
  onSetFeatureEnabled: (feature: FeatureRecord, enabled: boolean) => void;
  onUseDefaultState: (feature: FeatureRecord) => void;
}

function RolesWorkspaceContent({
  areaLabel,
  authState,
  autoRoleDetails,
  autoRoleFeature,
  advancedEnabledCount,
  advancedFeatures,
  pendingFeatureId,
  roleOptions,
  roleLookupNotice,
  rolePickerUnavailable,
  roleOptionsLoading,
  workspaceState,
  mutationSaving,
  onLogin,
  onOpenDrawer,
  onRefresh,
  onSetFeatureEnabled,
  onUseDefaultState,
}: RolesWorkspaceContentProps) {
  if (workspaceState !== "ready") {
    return (
      <EmptyState
        title={formatWorkspaceStateTitle(areaLabel, workspaceState)}
        description={formatWorkspaceStateDescription(areaLabel, workspaceState)}
        action={
          authState !== "signed_in" ? (
            <button className="button-primary" type="button" onClick={onLogin}>
              Sign in with Discord
            </button>
          ) : workspaceState === "unavailable" ? (
            <button className="button-secondary" type="button" onClick={onRefresh}>
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
      <RolesPrimaryWorkflow
        autoRoleDetails={autoRoleDetails}
        autoRoleFeature={autoRoleFeature}
        mutationSaving={mutationSaving}
        pendingFeatureId={pendingFeatureId}
        roleOptions={roleOptions}
        roleOptionsLoading={roleOptionsLoading}
        rolePickerUnavailable={rolePickerUnavailable}
        onOpenDrawer={onOpenDrawer}
        onSetFeatureEnabled={onSetFeatureEnabled}
        onUseDefaultState={onUseDefaultState}
      />

      {roleLookupNotice ? (
        <LookupNotice
          title="Role lookup unavailable"
          message="The dashboard could not load server roles right now. Refresh roles before opening role-based editors."
        />
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

      <RolesAdvancedControls
        advancedEnabledCount={advancedEnabledCount}
        advancedFeatures={advancedFeatures}
        mutationSaving={mutationSaving}
        pendingFeatureId={pendingFeatureId}
        roleOptionsLoading={roleOptionsLoading}
        rolePickerUnavailable={rolePickerUnavailable}
        onOpenDrawer={onOpenDrawer}
        onSetFeatureEnabled={onSetFeatureEnabled}
        onUseDefaultState={onUseDefaultState}
      />
    </>
  );
}

interface RolesPrimaryWorkflowProps {
  autoRoleDetails: AutoRoleDetails;
  autoRoleFeature: FeatureRecord;
  mutationSaving: boolean;
  pendingFeatureId: string;
  roleOptions: GuildRoleOption[];
  roleOptionsLoading: boolean;
  rolePickerUnavailable: boolean;
  onOpenDrawer: (feature: FeatureRecord) => void;
  onSetFeatureEnabled: (feature: FeatureRecord, enabled: boolean) => void;
  onUseDefaultState: (feature: FeatureRecord) => void;
}

function RolesPrimaryWorkflow({
  autoRoleDetails,
  autoRoleFeature,
  mutationSaving,
  pendingFeatureId,
  roleOptions,
  roleOptionsLoading,
  rolePickerUnavailable,
  onOpenDrawer,
  onSetFeatureEnabled,
  onUseDefaultState,
}: RolesPrimaryWorkflowProps) {
  return (
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
            disabled={
              roleOptionsLoading ||
              rolePickerUnavailable ||
              !canEditAutoRole(autoRoleFeature)
            }
            onClick={() => onOpenDrawer(autoRoleFeature)}
          >
            Configure auto role
          </button>
          <button
            className="button-ghost"
            type="button"
            disabled={mutationSaving}
            aria-label={`${autoRoleFeature.effective_enabled ? "Disable" : "Enable"} ${autoRoleFeature.label}`}
            onClick={() =>
              onSetFeatureEnabled(
                autoRoleFeature,
                !autoRoleFeature.effective_enabled,
              )
            }
          >
            {mutationSaving && pendingFeatureId === autoRoleFeature.id
              ? "Saving..."
              : autoRoleFeature.effective_enabled
                ? "Disable"
                : "Enable"}
          </button>
          {autoRoleFeature.override_state !== "inherit" ? (
            <button
              className="button-ghost"
              type="button"
              disabled={mutationSaving}
              onClick={() => onUseDefaultState(autoRoleFeature)}
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
            value: formatRoleValue(autoRoleDetails.targetRoleId, roleOptions),
          },
          {
            label: "Level role",
            value: formatRoleValue(autoRoleDetails.levelRoleId, roleOptions),
          },
          {
            label: "Booster role",
            value: formatRoleValue(autoRoleDetails.boosterRoleId, roleOptions),
          },
        ]}
      />
    </div>
  );
}

interface RolesAdvancedControlsProps {
  advancedEnabledCount: number;
  advancedFeatures: FeatureRecord[];
  mutationSaving: boolean;
  pendingFeatureId: string;
  roleOptionsLoading: boolean;
  rolePickerUnavailable: boolean;
  onOpenDrawer: (feature: FeatureRecord) => void;
  onSetFeatureEnabled: (feature: FeatureRecord, enabled: boolean) => void;
  onUseDefaultState: (feature: FeatureRecord) => void;
}

function RolesAdvancedControls({
  advancedEnabledCount,
  advancedFeatures,
  mutationSaving,
  pendingFeatureId,
  roleOptionsLoading,
  rolePickerUnavailable,
  onOpenDrawer,
  onSetFeatureEnabled,
  onUseDefaultState,
}: RolesAdvancedControlsProps) {
  return (
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
            const isPending = mutationSaving && pendingFeatureId === feature.id;
            const canOpenDrawer =
              feature.id === "presence_watch.user" ||
              feature.id === "presence_watch.bot" ||
              (feature.id === "safety.bot_role_perm_mirror" &&
                !roleOptionsLoading &&
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
                      onClick={() => onOpenDrawer(feature)}
                    >
                      Configure
                    </button>
                  ) : null}
                  <button
                    className="button-ghost"
                    type="button"
                    disabled={mutationSaving}
                    aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
                    onClick={() =>
                      onSetFeatureEnabled(feature, !feature.effective_enabled)
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
                      disabled={mutationSaving}
                      onClick={() => onUseDefaultState(feature)}
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
  );
}

interface RolesAsideProps {
  areaSummary: RolesAreaSummary;
  autoRoleDetails: AutoRoleDetails | null;
  autoRoleFeature: FeatureRecord | null;
  firstBlockedFeature: FeatureRecord | null;
  roleOptions: GuildRoleOption[];
  selectedServerLabel: string;
}

function RolesAside({
  areaSummary,
  autoRoleDetails,
  autoRoleFeature,
  firstBlockedFeature,
  roleOptions,
  selectedServerLabel,
}: RolesAsideProps) {
  return (
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
                  : formatRoleValue(autoRoleDetails.targetRoleId, roleOptions),
            },
            {
              label: "Requirement roles",
              value:
                autoRoleFeature === null
                  ? "Not available"
                  : formatRequirementRolesValue(autoRoleFeature, roleOptions),
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
          <li>Runtime issues stay in blockers and notices instead of taking over the main roles workspace.</li>
        </ul>

        {firstBlockedFeature?.id === "safety.bot_role_perm_mirror" ? (
          <div className="surface-subsection">
            <p className="section-label">Runtime dependency</p>
            <p className="meta-note">
              Permission mirror blockers from runtime state are reported by the
              control server and should be reviewed before saving.
            </p>
          </div>
        ) : null}
      </SurfaceCard>
    </aside>
  );
}

interface RolesFeatureDrawerProps {
  actorRoleDraft: string;
  boosterRoleDraft: string;
  closeDrawer: () => void;
  configEnabledDraft: string;
  levelRoleDraft: string;
  memberLookupLoading: boolean;
  memberLookupNotice: DashboardNotice | null;
  memberOptions: GuildMemberOption[];
  memberSearchDraft: string;
  mutationNotice: DashboardNotice | null;
  mutationSaving: boolean;
  pendingFeatureId: string;
  roleOptions: GuildRoleOption[];
  selectedFeature: FeatureRecord | null;
  targetRoleDraft: string;
  userIdDraft: string;
  watchBotDraft: string;
  refreshMemberOptions: () => Promise<void>;
  setActorRoleDraft: (value: string) => void;
  setBoosterRoleDraft: (value: string) => void;
  setConfigEnabledDraft: (value: string) => void;
  setLevelRoleDraft: (value: string) => void;
  setMemberSearchDraft: (value: string) => void;
  setTargetRoleDraft: (value: string) => void;
  setUserIdDraft: (value: string) => void;
  setWatchBotDraft: (value: string) => void;
  onSaveAutoRole: () => void;
  onSavePermissionMirror: () => void;
  onSavePresenceWatchBot: () => void;
  onSavePresenceWatchUser: () => void;
}

function RolesFeatureDrawer({
  actorRoleDraft,
  boosterRoleDraft,
  closeDrawer,
  configEnabledDraft,
  levelRoleDraft,
  memberLookupLoading,
  memberLookupNotice,
  memberOptions,
  memberSearchDraft,
  mutationNotice,
  mutationSaving,
  pendingFeatureId,
  roleOptions,
  selectedFeature,
  targetRoleDraft,
  userIdDraft,
  watchBotDraft,
  refreshMemberOptions,
  setActorRoleDraft,
  setBoosterRoleDraft,
  setConfigEnabledDraft,
  setLevelRoleDraft,
  setMemberSearchDraft,
  setTargetRoleDraft,
  setUserIdDraft,
  setWatchBotDraft,
  onSaveAutoRole,
  onSavePermissionMirror,
  onSavePresenceWatchBot,
  onSavePresenceWatchUser,
}: RolesFeatureDrawerProps) {
  if (selectedFeature === null) {
    return null;
  }

  return (
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

        {mutationNotice ? <AlertBanner notice={mutationNotice} /> : null}

        {selectedFeature.id === "auto_role_assignment" ? (
          <AutoRoleDrawerBody
            boosterRoleDraft={boosterRoleDraft}
            closeDrawer={closeDrawer}
            configEnabledDraft={configEnabledDraft}
            levelRoleDraft={levelRoleDraft}
            mutationSaving={mutationSaving}
            pendingFeatureId={pendingFeatureId}
            roleOptions={roleOptions}
            selectedFeature={selectedFeature}
            targetRoleDraft={targetRoleDraft}
            setBoosterRoleDraft={setBoosterRoleDraft}
            setConfigEnabledDraft={setConfigEnabledDraft}
            setLevelRoleDraft={setLevelRoleDraft}
            setTargetRoleDraft={setTargetRoleDraft}
            onSave={onSaveAutoRole}
          />
        ) : selectedFeature.id === "presence_watch.bot" ? (
          <PresenceWatchBotDrawerBody
            closeDrawer={closeDrawer}
            mutationSaving={mutationSaving}
            pendingFeatureId={pendingFeatureId}
            selectedFeature={selectedFeature}
            watchBotDraft={watchBotDraft}
            setWatchBotDraft={setWatchBotDraft}
            onSave={onSavePresenceWatchBot}
          />
        ) : selectedFeature.id === "presence_watch.user" ? (
          <PresenceWatchUserDrawerBody
            closeDrawer={closeDrawer}
            memberLookupLoading={memberLookupLoading}
            memberLookupNotice={memberLookupNotice}
            memberOptions={memberOptions}
            memberSearchDraft={memberSearchDraft}
            mutationSaving={mutationSaving}
            pendingFeatureId={pendingFeatureId}
            selectedFeature={selectedFeature}
            userIdDraft={userIdDraft}
            refreshMemberOptions={refreshMemberOptions}
            setMemberSearchDraft={setMemberSearchDraft}
            setUserIdDraft={setUserIdDraft}
            onSave={onSavePresenceWatchUser}
          />
        ) : (
          <PermissionMirrorDrawerBody
            actorRoleDraft={actorRoleDraft}
            closeDrawer={closeDrawer}
            mutationSaving={mutationSaving}
            pendingFeatureId={pendingFeatureId}
            permissionMirrorDetails={getPermissionMirrorDetails(selectedFeature)}
            roleOptions={roleOptions}
            selectedFeature={selectedFeature}
            setActorRoleDraft={setActorRoleDraft}
            onSave={onSavePermissionMirror}
          />
        )}
      </aside>
    </div>
  );
}

interface AutoRoleDrawerBodyProps {
  boosterRoleDraft: string;
  closeDrawer: () => void;
  configEnabledDraft: string;
  levelRoleDraft: string;
  mutationSaving: boolean;
  pendingFeatureId: string;
  roleOptions: GuildRoleOption[];
  selectedFeature: FeatureRecord;
  targetRoleDraft: string;
  setBoosterRoleDraft: (value: string) => void;
  setConfigEnabledDraft: (value: string) => void;
  setLevelRoleDraft: (value: string) => void;
  setTargetRoleDraft: (value: string) => void;
  onSave: () => void;
}

function AutoRoleDrawerBody({
  boosterRoleDraft,
  closeDrawer,
  configEnabledDraft,
  levelRoleDraft,
  mutationSaving,
  pendingFeatureId,
  roleOptions,
  selectedFeature,
  targetRoleDraft,
  setBoosterRoleDraft,
  setConfigEnabledDraft,
  setLevelRoleDraft,
  setTargetRoleDraft,
  onSave,
}: AutoRoleDrawerBodyProps) {
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
          onClick={onSave}
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

interface PresenceWatchBotDrawerBodyProps {
  closeDrawer: () => void;
  mutationSaving: boolean;
  pendingFeatureId: string;
  selectedFeature: FeatureRecord;
  watchBotDraft: string;
  setWatchBotDraft: (value: string) => void;
  onSave: () => void;
}

function PresenceWatchBotDrawerBody({
  closeDrawer,
  mutationSaving,
  pendingFeatureId,
  selectedFeature,
  watchBotDraft,
  setWatchBotDraft,
  onSave,
}: PresenceWatchBotDrawerBodyProps) {
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
          onClick={onSave}
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

interface PresenceWatchUserDrawerBodyProps {
  closeDrawer: () => void;
  memberLookupLoading: boolean;
  memberLookupNotice: DashboardNotice | null;
  memberOptions: GuildMemberOption[];
  memberSearchDraft: string;
  mutationSaving: boolean;
  pendingFeatureId: string;
  selectedFeature: FeatureRecord;
  userIdDraft: string;
  refreshMemberOptions: () => Promise<void>;
  setMemberSearchDraft: (value: string) => void;
  setUserIdDraft: (value: string) => void;
  onSave: () => void;
}

function PresenceWatchUserDrawerBody({
  closeDrawer,
  memberLookupLoading,
  memberLookupNotice,
  memberOptions,
  memberSearchDraft,
  mutationSaving,
  pendingFeatureId,
  selectedFeature,
  userIdDraft,
  refreshMemberOptions,
  setMemberSearchDraft,
  setUserIdDraft,
  onSave,
}: PresenceWatchUserDrawerBodyProps) {
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
            label: "Current member",
            value: formatMemberValue(userIdDraft, memberOptions),
          },
        ]}
      />

      <div className="field-grid roles-form-grid">
        <label className="field-stack">
          <span className="field-label">Search members</span>
          <input
            aria-label="Search members"
            value={memberSearchDraft}
            onChange={(event) => setMemberSearchDraft(event.target.value)}
            placeholder="Search by username, nickname, or user ID"
          />
          <span className="meta-note">
            Type to narrow the server member list without exposing raw IDs in
            the primary control.
          </span>
        </label>

        <label className="field-stack">
          <span className="field-label">Member</span>
          <select
            aria-label="Member"
            value={userIdDraft}
            disabled={memberLookupLoading || memberOptions.length === 0}
            onChange={(event) => setUserIdDraft(event.target.value)}
          >
            <option value="">
              {memberLookupLoading
                ? "Loading members..."
                : memberOptions.length === 0
                  ? "No matching members"
                  : "No member selected"}
            </option>
            {memberOptions.map((member) => (
              <option key={member.id} value={member.id}>
                {formatMemberOptionLabel(member)}
              </option>
            ))}
          </select>
          <span className="meta-note">
            The selected member stays available while you refine the search.
          </span>
        </label>
      </div>

      {memberLookupNotice ? (
        <LookupNotice
          title="Member lookup unavailable"
          message={memberLookupNotice.message}
          retryDisabled={memberLookupLoading}
          retryLabel="Retry member lookup"
          onRetry={refreshMemberOptions}
        />
      ) : null}

      {!memberLookupNotice && !memberLookupLoading && memberOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">No matches</p>
          <p className="meta-note">
            Adjust the search text to find a different member from the
            selected server.
          </p>
        </div>
      ) : null}

      <div className="drawer-actions">
        <button
          className="button-primary"
          type="button"
          disabled={mutationSaving}
          onClick={onSave}
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

interface PermissionMirrorDrawerBodyProps {
  actorRoleDraft: string;
  closeDrawer: () => void;
  mutationSaving: boolean;
  pendingFeatureId: string;
  permissionMirrorDetails: PermissionMirrorDetails;
  roleOptions: GuildRoleOption[];
  selectedFeature: FeatureRecord;
  setActorRoleDraft: (value: string) => void;
  onSave: () => void;
}

function PermissionMirrorDrawerBody({
  actorRoleDraft,
  closeDrawer,
  mutationSaving,
  pendingFeatureId,
  permissionMirrorDetails,
  roleOptions,
  selectedFeature,
  setActorRoleDraft,
  onSave,
}: PermissionMirrorDrawerBodyProps) {
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
          <p className="section-label">Runtime dependency</p>
          <p className="meta-note">
            Runtime permission mirror blockers are reported by the control
            server and should be reviewed before saving.
          </p>
        </div>
      ) : null}

      <div className="drawer-actions">
        <button
          className="button-primary"
          type="button"
          disabled={mutationSaving}
          onClick={onSave}
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
