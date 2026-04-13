import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import type {
  FeatureRecord,
  GuildMemberOption,
  GuildRoleOption,
} from "../api/control";
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
  featureHasTag,
  featureTags,
  findFeatureByTag,
  filterFeaturesByTag,
} from "../features/features/featureContract";
import {
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import {
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
import {
  featureSupportsAnyField,
  featureSupportsField,
} from "../features/features/model";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildMemberOptions } from "../features/features/useGuildMemberOptions";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";

type RolesWorkspaceState = ReturnType<
  typeof useFeatureWorkspace
>["workspaceState"];
type AutoRoleDetails = ReturnType<typeof getAutoRoleFeatureDetails>;
type PermissionMirrorDetails = ReturnType<typeof getPermissionMirrorDetails>;
type DashboardNotice = {
  tone: "info" | "success" | "error";
  message: string;
};

export function RolesPage() {
  const definition = getFeatureAreaDefinition("roles");
  const location = useLocation();
  const { authState, beginLogin, canEditSelectedGuild } = useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const roleOptions = useGuildRoleOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");
  const [configEnabledDraft, setConfigEnabledDraft] = useState("enabled");
  const [targetRoleDraft, setTargetRoleDraft] = useState("");
  const [levelRoleDraft, setLevelRoleDraft] = useState("");
  const [boosterRoleDraft, setBoosterRoleDraft] = useState("");
  const [watchBotDraft, setWatchBotDraft] = useState("enabled");
  const [userIdDraft, setUserIdDraft] = useState("");
  const [memberSearchDraft, setMemberSearchDraft] = useState("");
  const [actorRoleDraft, setActorRoleDraft] = useState("");
  const areaLabel = definition?.label ?? "Roles";
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "roles");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const autoRoleFeature = findFeatureByTag(
    areaFeatures,
    featureTags.rolesAutoAssignment,
  );
  const autoRoleDetails =
    autoRoleFeature === null
      ? null
      : getAutoRoleFeatureDetails(autoRoleFeature);
  const advancedFeatures = filterFeaturesByTag(areaFeatures, featureTags.rolesAdvanced);
  const presenceWatchBotFeature = findFeatureByTag(
    advancedFeatures,
    featureTags.rolesPresenceWatchBot,
  );
  const presenceWatchUserFeature = findFeatureByTag(
    advancedFeatures,
    featureTags.rolesPresenceWatchUser,
  );
  const permissionMirrorFeature = findFeatureByTag(
    advancedFeatures,
    featureTags.rolesPermissionMirror,
  );
  const firstBlockedFeature =
    areaFeatures.find((feature) => feature.readiness === "blocked") ?? null;
  const rolePickerUnavailable =
    roleOptions.notice !== null || roleOptions.roles.length === 0;
  const autoRoleConfigEnabled = autoRoleDetails?.configEnabled ?? null;
  const autoRoleTargetRoleId = autoRoleDetails?.targetRoleId ?? null;
  const autoRoleLevelRoleId = autoRoleDetails?.levelRoleId ?? null;
  const autoRoleBoosterRoleId = autoRoleDetails?.boosterRoleId ?? null;
  const presenceWatchBotEnabled =
    presenceWatchBotFeature === null
      ? null
      : getPresenceWatchBotDetails(presenceWatchBotFeature).watchBot;
  const presenceWatchUserId =
    presenceWatchUserFeature === null
      ? null
      : getPresenceWatchUserDetails(presenceWatchUserFeature).userId;
  const permissionMirrorRoleId =
    permissionMirrorFeature === null
      ? null
      : getPermissionMirrorDetails(permissionMirrorFeature).actorRoleId;
  const memberOptions = useGuildMemberOptions({
    enabled:
      workspace.workspaceState === "ready" && presenceWatchUserFeature !== null,
    query: memberSearchDraft,
    selectedMemberId: userIdDraft,
  });

  useEffect(() => {
    if (
      autoRoleConfigEnabled === null ||
      autoRoleTargetRoleId === null ||
      autoRoleLevelRoleId === null ||
      autoRoleBoosterRoleId === null
    ) {
      return;
    }
    setConfigEnabledDraft(autoRoleConfigEnabled ? "enabled" : "disabled");
    setTargetRoleDraft(autoRoleTargetRoleId);
    setLevelRoleDraft(autoRoleLevelRoleId);
    setBoosterRoleDraft(autoRoleBoosterRoleId);
  }, [
    autoRoleBoosterRoleId,
    autoRoleConfigEnabled,
    autoRoleLevelRoleId,
    autoRoleTargetRoleId,
  ]);

  useEffect(() => {
    if (presenceWatchBotEnabled === null) {
      return;
    }
    setWatchBotDraft(presenceWatchBotEnabled ? "enabled" : "disabled");
  }, [presenceWatchBotEnabled]);

  useEffect(() => {
    if (presenceWatchUserId === null) {
      return;
    }
    setUserIdDraft(presenceWatchUserId);
    setMemberSearchDraft("");
  }, [presenceWatchUserId]);

  useEffect(() => {
    if (permissionMirrorRoleId === null) {
      return;
    }
    setActorRoleDraft(permissionMirrorRoleId);
  }, [permissionMirrorRoleId]);

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
        workspace.updateFeature(updated);
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
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveAutoRole() {
    if (autoRoleFeature === null) {
      return;
    }

    setPendingFeatureId(autoRoleFeature.id);

    try {
      const updated = await mutation.patchFeature(autoRoleFeature.id, {
        config_enabled: configEnabledDraft === "enabled",
        target_role_id: targetRoleDraft,
        required_role_ids: [levelRoleDraft, boosterRoleDraft].filter(
          (value) => value.trim() !== "",
        ),
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSavePresenceWatchBot() {
    if (presenceWatchBotFeature === null) {
      return;
    }

    setPendingFeatureId(presenceWatchBotFeature.id);

    try {
      const updated = await mutation.patchFeature(presenceWatchBotFeature.id, {
        watch_bot: watchBotDraft === "enabled",
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSavePresenceWatchUser() {
    if (presenceWatchUserFeature === null) {
      return;
    }

    setPendingFeatureId(presenceWatchUserFeature.id);

    try {
      const updated = await mutation.patchFeature(presenceWatchUserFeature.id, {
        user_id: userIdDraft.trim(),
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSavePermissionMirror() {
    if (permissionMirrorFeature === null) {
      return;
    }

    setPendingFeatureId(permissionMirrorFeature.id);

    try {
      const updated = await mutation.patchFeature(permissionMirrorFeature.id, {
        actor_role_id: actorRoleDraft,
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  const autoRoleHasUnsavedChanges =
    autoRoleDetails !== null &&
    (autoRoleDetails.configEnabled !== (configEnabledDraft === "enabled") ||
      autoRoleDetails.targetRoleId !== targetRoleDraft.trim() ||
      autoRoleDetails.levelRoleId !== levelRoleDraft.trim() ||
      autoRoleDetails.boosterRoleId !== boosterRoleDraft.trim());
  const presenceWatchBotHasUnsavedChanges =
    presenceWatchBotFeature !== null &&
    getPresenceWatchBotDetails(presenceWatchBotFeature).watchBot !==
      (watchBotDraft === "enabled");
  const presenceWatchUserHasUnsavedChanges =
    presenceWatchUserFeature !== null &&
    getPresenceWatchUserDetails(presenceWatchUserFeature).userId !==
      userIdDraft.trim();
  const permissionMirrorHasUnsavedChanges =
    permissionMirrorFeature !== null &&
    getPermissionMirrorDetails(permissionMirrorFeature).actorRoleId !==
      actorRoleDraft.trim();

  function resetAutoRoleDrafts() {
    if (autoRoleDetails === null) {
      return;
    }

    mutation.clearNotice();
    setConfigEnabledDraft(
      autoRoleDetails.configEnabled ? "enabled" : "disabled",
    );
    setTargetRoleDraft(autoRoleDetails.targetRoleId);
    setLevelRoleDraft(autoRoleDetails.levelRoleId);
    setBoosterRoleDraft(autoRoleDetails.boosterRoleId);
  }

  function resetPresenceWatchBotDraft() {
    if (presenceWatchBotFeature === null) {
      return;
    }

    mutation.clearNotice();
    setWatchBotDraft(
      getPresenceWatchBotDetails(presenceWatchBotFeature).watchBot
        ? "enabled"
        : "disabled",
    );
  }

  function resetPresenceWatchUserDraft() {
    if (presenceWatchUserFeature === null) {
      return;
    }

    mutation.clearNotice();
    setUserIdDraft(
      getPresenceWatchUserDetails(presenceWatchUserFeature).userId,
    );
    setMemberSearchDraft("");
  }

  function resetPermissionMirrorDraft() {
    if (permissionMirrorFeature === null) {
      return;
    }

    mutation.clearNotice();
    setActorRoleDraft(
      getPermissionMirrorDetails(permissionMirrorFeature).actorRoleId,
    );
  }

  if (definition === null) {
    return null;
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
    ) : null;

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Feature area"
        title={areaLabel}
        description="Configure automatic role assignment and the related presence-aware controls inline, without hiding the main role editors behind drawers or disclosure panels."
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
        actions={headerActions}
      />

      <FlatPageLayout
        notice={workspaceNotice}
        summary={
          workspace.workspaceState === "ready" &&
          autoRoleFeature !== null &&
          autoRoleDetails !== null ? (
            <section
              className="overview-summary-strip"
              aria-label="Roles summary"
            >
              <MetricCard
                label="Auto role"
                value={formatFeatureStatusLabel(autoRoleFeature)}
                description={summarizeAutoRoleSignal(autoRoleFeature)}
                tone={getFeatureStatusTone(autoRoleFeature)}
              />
              <MetricCard
                label="Target role"
                value={formatRoleValue(
                  autoRoleDetails.targetRoleId,
                  roleOptions.roles,
                )}
                description="The role that gets assigned automatically."
              />
              <MetricCard
                label="Requirement roles"
                value={formatRequirementRolesValue(
                  autoRoleFeature,
                  roleOptions.roles,
                )}
                description={
                  autoRoleDetails.requiredRoleCount === 2
                    ? "Both requirement roles are configured."
                    : "Choose the level and booster roles that gate the assignment."
                }
                tone={
                  autoRoleDetails.requiredRoleCount === 2
                    ? "success"
                    : "neutral"
                }
              />
            </section>
          ) : null
        }
        workspaceTitle="Manage roles"
        workspaceDescription="Keep auto role assignment, presence watching, and the permission mirror in one flat workspace so the main role controls stay visible while you edit them."
      >
        <RolesWorkspaceContent
          areaLabel={areaLabel}
          authState={authState}
          autoRoleDetails={autoRoleDetails}
          autoRoleFeature={autoRoleFeature}
          configEnabledDraft={configEnabledDraft}
          targetRoleDraft={targetRoleDraft}
          levelRoleDraft={levelRoleDraft}
          boosterRoleDraft={boosterRoleDraft}
          watchBotDraft={watchBotDraft}
          userIdDraft={userIdDraft}
          memberSearchDraft={memberSearchDraft}
          actorRoleDraft={actorRoleDraft}
          autoRoleHasUnsavedChanges={autoRoleHasUnsavedChanges}
          presenceWatchBotHasUnsavedChanges={presenceWatchBotHasUnsavedChanges}
          presenceWatchUserHasUnsavedChanges={
            presenceWatchUserHasUnsavedChanges
          }
          permissionMirrorHasUnsavedChanges={permissionMirrorHasUnsavedChanges}
          advancedFeatures={advancedFeatures}
          firstBlockedFeature={firstBlockedFeature}
          pendingFeatureId={pendingFeatureId}
          roleOptions={roleOptions.roles}
          roleLookupNotice={roleOptions.notice}
          rolePickerUnavailable={rolePickerUnavailable}
          roleOptionsLoading={roleOptions.loading}
          memberOptions={memberOptions.members}
          memberLookupLoading={memberOptions.loading}
          memberLookupNotice={memberOptions.notice}
          workspaceState={workspace.workspaceState}
          mutationSaving={mutation.saving}
          canEditSelectedGuild={canEditSelectedGuild}
          onLogin={() => void beginLogin(nextPath)}
          onRefresh={() => void handleRefreshRoles()}
          onRefreshMemberOptions={memberOptions.refresh}
          onSetFeatureEnabled={(feature, enabled) => {
            void handleSetFeatureEnabled(feature, enabled);
          }}
          onUseDefaultState={(feature) => {
            void handleUseDefaultState(feature);
          }}
          onSaveAutoRole={() => void handleSaveAutoRole()}
          onSavePresenceWatchBot={() => void handleSavePresenceWatchBot()}
          onSavePresenceWatchUser={() => void handleSavePresenceWatchUser()}
          onSavePermissionMirror={() => void handleSavePermissionMirror()}
          onResetAutoRole={resetAutoRoleDrafts}
          onResetPresenceWatchBot={resetPresenceWatchBotDraft}
          onResetPresenceWatchUser={resetPresenceWatchUserDraft}
          onResetPermissionMirror={resetPermissionMirrorDraft}
          setConfigEnabledDraft={setConfigEnabledDraft}
          setTargetRoleDraft={setTargetRoleDraft}
          setLevelRoleDraft={setLevelRoleDraft}
          setBoosterRoleDraft={setBoosterRoleDraft}
          setWatchBotDraft={setWatchBotDraft}
          setUserIdDraft={setUserIdDraft}
          setMemberSearchDraft={setMemberSearchDraft}
          setActorRoleDraft={setActorRoleDraft}
        />
      </FlatPageLayout>
    </section>
  );
}

interface RolesWorkspaceContentProps {
  areaLabel: string;
  authState: string;
  autoRoleDetails: AutoRoleDetails | null;
  autoRoleFeature: FeatureRecord | null;
  configEnabledDraft: string;
  targetRoleDraft: string;
  levelRoleDraft: string;
  boosterRoleDraft: string;
  watchBotDraft: string;
  userIdDraft: string;
  memberSearchDraft: string;
  actorRoleDraft: string;
  autoRoleHasUnsavedChanges: boolean;
  presenceWatchBotHasUnsavedChanges: boolean;
  presenceWatchUserHasUnsavedChanges: boolean;
  permissionMirrorHasUnsavedChanges: boolean;
  advancedFeatures: FeatureRecord[];
  firstBlockedFeature: FeatureRecord | null;
  pendingFeatureId: string;
  roleOptions: GuildRoleOption[];
  roleLookupNotice: DashboardNotice | null;
  rolePickerUnavailable: boolean;
  roleOptionsLoading: boolean;
  memberOptions: GuildMemberOption[];
  memberLookupLoading: boolean;
  memberLookupNotice: DashboardNotice | null;
  workspaceState: RolesWorkspaceState;
  mutationSaving: boolean;
  canEditSelectedGuild: boolean;
  onLogin: () => void;
  onRefresh: () => void;
  onRefreshMemberOptions: () => Promise<void>;
  onSetFeatureEnabled: (feature: FeatureRecord, enabled: boolean) => void;
  onUseDefaultState: (feature: FeatureRecord) => void;
  onSaveAutoRole: () => void;
  onSavePresenceWatchBot: () => void;
  onSavePresenceWatchUser: () => void;
  onSavePermissionMirror: () => void;
  onResetAutoRole: () => void;
  onResetPresenceWatchBot: () => void;
  onResetPresenceWatchUser: () => void;
  onResetPermissionMirror: () => void;
  setConfigEnabledDraft: (value: string) => void;
  setTargetRoleDraft: (value: string) => void;
  setLevelRoleDraft: (value: string) => void;
  setBoosterRoleDraft: (value: string) => void;
  setWatchBotDraft: (value: string) => void;
  setUserIdDraft: (value: string) => void;
  setMemberSearchDraft: (value: string) => void;
  setActorRoleDraft: (value: string) => void;
}

function RolesWorkspaceContent({
  areaLabel,
  authState,
  autoRoleDetails,
  autoRoleFeature,
  configEnabledDraft,
  targetRoleDraft,
  levelRoleDraft,
  boosterRoleDraft,
  watchBotDraft,
  userIdDraft,
  memberSearchDraft,
  actorRoleDraft,
  autoRoleHasUnsavedChanges,
  presenceWatchBotHasUnsavedChanges,
  presenceWatchUserHasUnsavedChanges,
  permissionMirrorHasUnsavedChanges,
  advancedFeatures,
  firstBlockedFeature,
  pendingFeatureId,
  roleOptions,
  roleLookupNotice,
  rolePickerUnavailable,
  roleOptionsLoading,
  memberOptions,
  memberLookupLoading,
  memberLookupNotice,
  workspaceState,
  mutationSaving,
  canEditSelectedGuild,
  onLogin,
  onRefresh,
  onRefreshMemberOptions,
  onSetFeatureEnabled,
  onUseDefaultState,
  onSaveAutoRole,
  onSavePresenceWatchBot,
  onSavePresenceWatchUser,
  onSavePermissionMirror,
  onResetAutoRole,
  onResetPresenceWatchBot,
  onResetPresenceWatchUser,
  onResetPermissionMirror,
  setConfigEnabledDraft,
  setTargetRoleDraft,
  setLevelRoleDraft,
  setBoosterRoleDraft,
  setWatchBotDraft,
  setUserIdDraft,
  setMemberSearchDraft,
  setActorRoleDraft,
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
            <button
              className="button-secondary"
              type="button"
              onClick={onRefresh}
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
      {roleLookupNotice ? (
        <LookupNotice
          title="Role lookup unavailable"
          message="The dashboard could not load server roles right now. Retry the role lookup before saving role-based editors."
          retryLabel="Retry role lookup"
          retryDisabled={roleOptionsLoading}
          onRetry={onRefresh}
        />
      ) : null}

      {firstBlockedFeature ? (
        <div className="surface-subsection">
          <p className="section-label">Needs setup</p>
          <strong>{firstBlockedFeature.label}</strong>
          <p className="meta-note">
            {featureHasTag(firstBlockedFeature, featureTags.rolesAutoAssignment)
              ? summarizeAutoRoleSignal(firstBlockedFeature)
              : summarizeAdvancedRoleSignal(firstBlockedFeature)}
          </p>
        </div>
      ) : null}

      <div className="flat-config-stack">
        <RolesPrimaryWorkflow
          autoRoleFeature={autoRoleFeature}
          configEnabledDraft={configEnabledDraft}
          targetRoleDraft={targetRoleDraft}
          levelRoleDraft={levelRoleDraft}
          boosterRoleDraft={boosterRoleDraft}
          hasUnsavedChanges={autoRoleHasUnsavedChanges}
          mutationSaving={mutationSaving}
          pendingFeatureId={pendingFeatureId}
          roleOptions={roleOptions}
          roleOptionsLoading={roleOptionsLoading}
          rolePickerUnavailable={rolePickerUnavailable}
          canEditSelectedGuild={canEditSelectedGuild}
          onReset={onResetAutoRole}
          onSave={onSaveAutoRole}
          onSetConfigEnabledDraft={setConfigEnabledDraft}
          onSetTargetRoleDraft={setTargetRoleDraft}
          onSetLevelRoleDraft={setLevelRoleDraft}
          onSetBoosterRoleDraft={setBoosterRoleDraft}
          onSetFeatureEnabled={onSetFeatureEnabled}
          onUseDefaultState={onUseDefaultState}
        />

        <RolesAdvancedControls
          advancedFeatures={advancedFeatures}
          watchBotDraft={watchBotDraft}
          userIdDraft={userIdDraft}
          memberSearchDraft={memberSearchDraft}
          actorRoleDraft={actorRoleDraft}
          presenceWatchBotHasUnsavedChanges={presenceWatchBotHasUnsavedChanges}
          presenceWatchUserHasUnsavedChanges={
            presenceWatchUserHasUnsavedChanges
          }
          permissionMirrorHasUnsavedChanges={permissionMirrorHasUnsavedChanges}
          pendingFeatureId={pendingFeatureId}
          roleOptions={roleOptions}
          roleLookupNotice={roleLookupNotice}
          rolePickerUnavailable={rolePickerUnavailable}
          roleOptionsLoading={roleOptionsLoading}
          memberOptions={memberOptions}
          memberLookupLoading={memberLookupLoading}
          memberLookupNotice={memberLookupNotice}
          mutationSaving={mutationSaving}
          canEditSelectedGuild={canEditSelectedGuild}
          onRefreshMemberOptions={onRefreshMemberOptions}
          onResetPresenceWatchBot={onResetPresenceWatchBot}
          onResetPresenceWatchUser={onResetPresenceWatchUser}
          onResetPermissionMirror={onResetPermissionMirror}
          onSavePresenceWatchBot={onSavePresenceWatchBot}
          onSavePresenceWatchUser={onSavePresenceWatchUser}
          onSavePermissionMirror={onSavePermissionMirror}
          onSetWatchBotDraft={setWatchBotDraft}
          onSetUserIdDraft={setUserIdDraft}
          onSetMemberSearchDraft={setMemberSearchDraft}
          onSetActorRoleDraft={setActorRoleDraft}
          onSetFeatureEnabled={onSetFeatureEnabled}
          onUseDefaultState={onUseDefaultState}
        />
      </div>
    </>
  );
}

interface RolesPrimaryWorkflowProps {
  autoRoleFeature: FeatureRecord;
  configEnabledDraft: string;
  targetRoleDraft: string;
  levelRoleDraft: string;
  boosterRoleDraft: string;
  hasUnsavedChanges: boolean;
  mutationSaving: boolean;
  pendingFeatureId: string;
  roleOptions: GuildRoleOption[];
  roleOptionsLoading: boolean;
  rolePickerUnavailable: boolean;
  canEditSelectedGuild: boolean;
  onReset: () => void;
  onSave: () => void;
  onSetConfigEnabledDraft: (value: string) => void;
  onSetTargetRoleDraft: (value: string) => void;
  onSetLevelRoleDraft: (value: string) => void;
  onSetBoosterRoleDraft: (value: string) => void;
  onSetFeatureEnabled: (feature: FeatureRecord, enabled: boolean) => void;
  onUseDefaultState: (feature: FeatureRecord) => void;
}

function RolesPrimaryWorkflow({
  autoRoleFeature,
  configEnabledDraft,
  targetRoleDraft,
  levelRoleDraft,
  boosterRoleDraft,
  hasUnsavedChanges,
  mutationSaving,
  pendingFeatureId,
  roleOptions,
  roleOptionsLoading,
  rolePickerUnavailable,
  canEditSelectedGuild,
  onReset,
  onSave,
  onSetConfigEnabledDraft,
  onSetTargetRoleDraft,
  onSetLevelRoleDraft,
  onSetBoosterRoleDraft,
  onSetFeatureEnabled,
  onUseDefaultState,
}: RolesPrimaryWorkflowProps) {
  const canEditSettings =
    canEditSelectedGuild &&
    featureSupportsAnyField(autoRoleFeature, [
      "config_enabled",
      "target_role_id",
      "required_role_ids",
    ]) &&
    !roleOptionsLoading &&
    !rolePickerUnavailable;

  return (
    <section className="flat-config-section roles-primary-section">
      <div className="flat-config-header">
        <div className="card-copy flat-config-copy">
          <p className="section-label">Primary workflow</p>
          <div className="flat-config-title-row">
            <h2>Automatic role assignment</h2>
            <StatusBadge tone={getFeatureStatusTone(autoRoleFeature)}>
              {formatFeatureStatusLabel(autoRoleFeature)}
            </StatusBadge>
          </div>
          <p className="section-description">
            Configure the role that should be assigned automatically and the
            role requirements that gate the assignment flow without leaving this
            page.
          </p>
        </div>
      </div>

      <AutoRoleDrawerBody
        boosterRoleDraft={boosterRoleDraft}
        configEnabledDraft={configEnabledDraft}
        levelRoleDraft={levelRoleDraft}
        roleOptions={roleOptions}
        selectedFeature={autoRoleFeature}
        targetRoleDraft={targetRoleDraft}
        disabled={!canEditSettings}
        setBoosterRoleDraft={onSetBoosterRoleDraft}
        setConfigEnabledDraft={onSetConfigEnabledDraft}
        setLevelRoleDraft={onSetLevelRoleDraft}
        setTargetRoleDraft={onSetTargetRoleDraft}
      />

      <div className="feature-row-actions roles-primary-actions flat-config-actions">
        <button
          className="button-ghost"
          type="button"
          disabled={mutationSaving || !canEditSelectedGuild}
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
            disabled={mutationSaving || !canEditSelectedGuild}
            onClick={() => onUseDefaultState(autoRoleFeature)}
          >
            Use default
          </button>
        ) : null}
      </div>

      <UnsavedChangesBar
        hasUnsavedChanges={hasUnsavedChanges}
        saveLabel={
          mutationSaving && pendingFeatureId === autoRoleFeature.id
            ? "Saving..."
            : "Save changes"
        }
        saving={mutationSaving && pendingFeatureId === autoRoleFeature.id}
        disabled={!canEditSettings}
        onReset={onReset}
        onSave={onSave}
      />
    </section>
  );
}

interface RolesAdvancedControlsProps {
  advancedFeatures: FeatureRecord[];
  watchBotDraft: string;
  userIdDraft: string;
  memberSearchDraft: string;
  actorRoleDraft: string;
  presenceWatchBotHasUnsavedChanges: boolean;
  presenceWatchUserHasUnsavedChanges: boolean;
  permissionMirrorHasUnsavedChanges: boolean;
  pendingFeatureId: string;
  roleOptions: GuildRoleOption[];
  roleLookupNotice: DashboardNotice | null;
  rolePickerUnavailable: boolean;
  roleOptionsLoading: boolean;
  memberOptions: GuildMemberOption[];
  memberLookupLoading: boolean;
  memberLookupNotice: DashboardNotice | null;
  mutationSaving: boolean;
  canEditSelectedGuild: boolean;
  onRefreshMemberOptions: () => Promise<void>;
  onResetPresenceWatchBot: () => void;
  onResetPresenceWatchUser: () => void;
  onResetPermissionMirror: () => void;
  onSavePresenceWatchBot: () => void;
  onSavePresenceWatchUser: () => void;
  onSavePermissionMirror: () => void;
  onSetWatchBotDraft: (value: string) => void;
  onSetUserIdDraft: (value: string) => void;
  onSetMemberSearchDraft: (value: string) => void;
  onSetActorRoleDraft: (value: string) => void;
  onSetFeatureEnabled: (feature: FeatureRecord, enabled: boolean) => void;
  onUseDefaultState: (feature: FeatureRecord) => void;
}

function RolesAdvancedControls({
  advancedFeatures,
  watchBotDraft,
  userIdDraft,
  memberSearchDraft,
  actorRoleDraft,
  presenceWatchBotHasUnsavedChanges,
  presenceWatchUserHasUnsavedChanges,
  permissionMirrorHasUnsavedChanges,
  pendingFeatureId,
  roleOptions,
  roleLookupNotice,
  rolePickerUnavailable,
  roleOptionsLoading,
  memberOptions,
  memberLookupLoading,
  memberLookupNotice,
  mutationSaving,
  canEditSelectedGuild,
  onRefreshMemberOptions,
  onResetPresenceWatchBot,
  onResetPresenceWatchUser,
  onResetPermissionMirror,
  onSavePresenceWatchBot,
  onSavePresenceWatchUser,
  onSavePermissionMirror,
  onSetWatchBotDraft,
  onSetUserIdDraft,
  onSetMemberSearchDraft,
  onSetActorRoleDraft,
  onSetFeatureEnabled,
  onUseDefaultState,
}: RolesAdvancedControlsProps) {
  return (
    <>
      {advancedFeatures.map((feature) => {
        if (featureHasTag(feature, featureTags.rolesPresenceWatchBot)) {
          const canEditSettings =
            canEditSelectedGuild && featureSupportsField(feature, "watch_bot");

          return (
            <section className="flat-config-section" key={feature.id}>
              <div className="flat-config-header">
                <div className="card-copy flat-config-copy">
                  <p className="section-label">Advanced control</p>
                  <div className="flat-config-title-row">
                    <h2>{feature.label}</h2>
                    <StatusBadge tone={getFeatureStatusTone(feature)}>
                      {formatFeatureStatusLabel(feature)}
                    </StatusBadge>
                  </div>
                  <p className="section-description">{feature.description}</p>
                </div>
              </div>

              <PresenceWatchBotDrawerBody
                selectedFeature={feature}
                watchBotDraft={watchBotDraft}
                disabled={!canEditSettings}
                setWatchBotDraft={onSetWatchBotDraft}
              />

              <div className="feature-row-actions roles-advanced-actions flat-config-actions">
                <button
                  className="button-ghost"
                  type="button"
                  disabled={mutationSaving || !canEditSelectedGuild}
                  aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
                  onClick={() =>
                    onSetFeatureEnabled(feature, !feature.effective_enabled)
                  }
                >
                  {mutationSaving && pendingFeatureId === feature.id
                    ? "Saving..."
                    : feature.effective_enabled
                      ? "Disable"
                      : "Enable"}
                </button>
                {feature.override_state !== "inherit" ? (
                  <button
                    className="button-ghost"
                    type="button"
                    disabled={mutationSaving || !canEditSelectedGuild}
                    onClick={() => onUseDefaultState(feature)}
                  >
                    Use default
                  </button>
                ) : null}
              </div>

              <UnsavedChangesBar
                hasUnsavedChanges={presenceWatchBotHasUnsavedChanges}
                saveLabel={
                  mutationSaving && pendingFeatureId === feature.id
                    ? "Saving..."
                    : "Save changes"
                }
                saving={mutationSaving && pendingFeatureId === feature.id}
                disabled={!canEditSettings}
                onReset={onResetPresenceWatchBot}
                onSave={onSavePresenceWatchBot}
              />
            </section>
          );
        }

        if (featureHasTag(feature, featureTags.rolesPresenceWatchUser)) {
          const canEditSettings =
            canEditSelectedGuild && featureSupportsField(feature, "user_id");

          return (
            <section className="flat-config-section" key={feature.id}>
              <div className="flat-config-header">
                <div className="card-copy flat-config-copy">
                  <p className="section-label">Advanced control</p>
                  <div className="flat-config-title-row">
                    <h2>{feature.label}</h2>
                    <StatusBadge tone={getFeatureStatusTone(feature)}>
                      {formatFeatureStatusLabel(feature)}
                    </StatusBadge>
                  </div>
                  <p className="section-description">{feature.description}</p>
                </div>
              </div>

              <PresenceWatchUserDrawerBody
                memberLookupLoading={memberLookupLoading}
                memberLookupNotice={memberLookupNotice}
                memberOptions={memberOptions}
                memberSearchDraft={memberSearchDraft}
                selectedFeature={feature}
                userIdDraft={userIdDraft}
                disabled={!canEditSettings}
                refreshMemberOptions={onRefreshMemberOptions}
                setMemberSearchDraft={onSetMemberSearchDraft}
                setUserIdDraft={onSetUserIdDraft}
              />

              <div className="feature-row-actions roles-advanced-actions flat-config-actions">
                <button
                  className="button-ghost"
                  type="button"
                  disabled={mutationSaving || !canEditSelectedGuild}
                  aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
                  onClick={() =>
                    onSetFeatureEnabled(feature, !feature.effective_enabled)
                  }
                >
                  {mutationSaving && pendingFeatureId === feature.id
                    ? "Saving..."
                    : feature.effective_enabled
                      ? "Disable"
                      : "Enable"}
                </button>
                {feature.override_state !== "inherit" ? (
                  <button
                    className="button-ghost"
                    type="button"
                    disabled={mutationSaving || !canEditSelectedGuild}
                    onClick={() => onUseDefaultState(feature)}
                  >
                    Use default
                  </button>
                ) : null}
              </div>

              <UnsavedChangesBar
                hasUnsavedChanges={presenceWatchUserHasUnsavedChanges}
                saveLabel={
                  mutationSaving && pendingFeatureId === feature.id
                    ? "Saving..."
                    : "Save changes"
                }
                saving={mutationSaving && pendingFeatureId === feature.id}
                disabled={!canEditSettings}
                onReset={onResetPresenceWatchUser}
                onSave={onSavePresenceWatchUser}
              />
            </section>
          );
        }

        const canEditSettings =
          canEditSelectedGuild &&
          featureSupportsField(feature, "actor_role_id") &&
          !roleOptionsLoading &&
          !rolePickerUnavailable &&
          roleLookupNotice === null;

        return (
          <section className="flat-config-section" key={feature.id}>
            <div className="flat-config-header">
              <div className="card-copy flat-config-copy">
                <p className="section-label">Advanced control</p>
                <div className="flat-config-title-row">
                  <h2>{feature.label}</h2>
                  <StatusBadge tone={getFeatureStatusTone(feature)}>
                    {formatFeatureStatusLabel(feature)}
                  </StatusBadge>
                </div>
                <p className="section-description">{feature.description}</p>
              </div>
            </div>

            <PermissionMirrorDrawerBody
              actorRoleDraft={actorRoleDraft}
              permissionMirrorDetails={getPermissionMirrorDetails(feature)}
              roleOptions={roleOptions}
              selectedFeature={feature}
              disabled={!canEditSettings}
              setActorRoleDraft={onSetActorRoleDraft}
            />

            <div className="feature-row-actions roles-advanced-actions flat-config-actions">
              <button
                className="button-ghost"
                type="button"
                disabled={mutationSaving || !canEditSelectedGuild}
                aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
                onClick={() =>
                  onSetFeatureEnabled(feature, !feature.effective_enabled)
                }
              >
                {mutationSaving && pendingFeatureId === feature.id
                  ? "Saving..."
                  : feature.effective_enabled
                    ? "Disable"
                    : "Enable"}
              </button>
              {feature.override_state !== "inherit" ? (
                <button
                  className="button-ghost"
                  type="button"
                  disabled={mutationSaving || !canEditSelectedGuild}
                  onClick={() => onUseDefaultState(feature)}
                >
                  Use default
                </button>
              ) : null}
            </div>

            <UnsavedChangesBar
              hasUnsavedChanges={permissionMirrorHasUnsavedChanges}
              saveLabel={
                mutationSaving && pendingFeatureId === feature.id
                  ? "Saving..."
                  : "Save changes"
              }
              saving={mutationSaving && pendingFeatureId === feature.id}
              disabled={!canEditSettings}
              onReset={onResetPermissionMirror}
              onSave={onSavePermissionMirror}
            />
          </section>
        );
      })}
    </>
  );
}

interface AutoRoleDrawerBodyProps {
  boosterRoleDraft: string;
  configEnabledDraft: string;
  levelRoleDraft: string;
  roleOptions: GuildRoleOption[];
  selectedFeature: FeatureRecord;
  targetRoleDraft: string;
  disabled?: boolean;
  setBoosterRoleDraft: (value: string) => void;
  setConfigEnabledDraft: (value: string) => void;
  setLevelRoleDraft: (value: string) => void;
  setTargetRoleDraft: (value: string) => void;
}

function AutoRoleDrawerBody({
  boosterRoleDraft,
  configEnabledDraft,
  levelRoleDraft,
  roleOptions,
  selectedFeature,
  targetRoleDraft,
  disabled = false,
  setBoosterRoleDraft,
  setConfigEnabledDraft,
  setLevelRoleDraft,
  setTargetRoleDraft,
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
            disabled={disabled}
            value={configEnabledDraft}
            onChange={(event) => setConfigEnabledDraft(event.target.value)}
          >
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>
          <span className="meta-note">
            Leave the module on while pausing assignment.
          </span>
        </label>

        <label className="field-stack">
          <span className="field-label">Target role</span>
          <select
            aria-label="Target role"
            disabled={disabled}
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
            disabled={disabled}
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
            disabled={disabled}
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
    </>
  );
}

interface PresenceWatchBotDrawerBodyProps {
  selectedFeature: FeatureRecord;
  watchBotDraft: string;
  disabled?: boolean;
  setWatchBotDraft: (value: string) => void;
}

function PresenceWatchBotDrawerBody({
  selectedFeature,
  watchBotDraft,
  disabled = false,
  setWatchBotDraft,
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
          disabled={disabled}
          value={watchBotDraft}
          onChange={(event) => setWatchBotDraft(event.target.value)}
        >
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
        </select>
        <span className="meta-note">
          Controls whether the runtime watches the bot account.
        </span>
      </label>
    </>
  );
}

interface PresenceWatchUserDrawerBodyProps {
  memberLookupLoading: boolean;
  memberLookupNotice: DashboardNotice | null;
  memberOptions: GuildMemberOption[];
  memberSearchDraft: string;
  selectedFeature: FeatureRecord;
  userIdDraft: string;
  disabled?: boolean;
  refreshMemberOptions: () => Promise<void>;
  setMemberSearchDraft: (value: string) => void;
  setUserIdDraft: (value: string) => void;
}

function PresenceWatchUserDrawerBody({
  memberLookupLoading,
  memberLookupNotice,
  memberOptions,
  memberSearchDraft,
  selectedFeature,
  userIdDraft,
  disabled = false,
  refreshMemberOptions,
  setMemberSearchDraft,
  setUserIdDraft,
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
            disabled={disabled}
            value={memberSearchDraft}
            onChange={(event) => setMemberSearchDraft(event.target.value)}
            placeholder="Search by username, nickname, or user ID"
          />
          <span className="meta-note">Filter the server member list.</span>
        </label>

        <label className="field-stack">
          <span className="field-label">Member</span>
          <select
            aria-label="Member"
            value={userIdDraft}
            disabled={
              disabled || memberLookupLoading || memberOptions.length === 0
            }
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
            Keep the current member or pick a new one.
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

      {!memberLookupNotice &&
      !memberLookupLoading &&
      memberOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">No matches</p>
          <p className="meta-note">
            Adjust the search text to find a different member from the selected
            server.
          </p>
        </div>
      ) : null}
    </>
  );
}

interface PermissionMirrorDrawerBodyProps {
  actorRoleDraft: string;
  permissionMirrorDetails: PermissionMirrorDetails;
  roleOptions: GuildRoleOption[];
  selectedFeature: FeatureRecord;
  disabled?: boolean;
  setActorRoleDraft: (value: string) => void;
}

function PermissionMirrorDrawerBody({
  actorRoleDraft,
  permissionMirrorDetails,
  roleOptions,
  selectedFeature,
  disabled = false,
  setActorRoleDraft,
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
          disabled={disabled}
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
        <span className="meta-note">Leave empty to keep the guard global.</span>
      </label>
    </>
  );
}
