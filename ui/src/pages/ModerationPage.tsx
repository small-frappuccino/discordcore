import { useEffect, useId, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  EmptyState,
  FlatPageLayout,
  GroupedSettingsCopy,
  GroupedSettingsGroup,
  GroupedSettingsHeading,
  GroupedSettingsInlineMessage,
  GroupedSettingsItem,
  GroupedSettingsMainRow,
  GroupedSettingsSection,
  GroupedSettingsStack,
  GroupedSettingsSubrow,
  GroupedSettingsSwitch,
  SettingsSelectField,
  UnsavedChangesBar,
} from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { buildMessageRouteChannelPickerOptions } from "../features/features/discordEntities";
import {
  getFeatureAreaDefinition,
  getFeatureAreaRecords,
} from "../features/features/areas";
import {
  canEditLoggingChannel,
  getLoggingFeatureDetails,
  summarizeLoggingGuidance,
} from "../features/features/logging";
import {
  canEditMuteRole,
  getModerationCommandFeatures,
  getModerationLogFeatures,
  getMuteRoleFeatureDetails,
} from "../features/features/moderation";
import { formatRoleOptionLabel } from "../features/features/roles";
import {
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
} from "../features/features/presentation";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";

export function ModerationPage() {
  const definition = getFeatureAreaDefinition("moderation");
  const location = useLocation();
  const {
    authState,
    beginLogin,
    canEditSelectedGuild,
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

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "moderation");
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const automodFeature =
    areaFeatures.find((feature) => feature.id === "services.automod") ?? null;
  const muteRoleFeature =
    areaFeatures.find((feature) => feature.id === "moderation.mute_role") ?? null;
  const moderationCommandFeatures = getModerationCommandFeatures(areaFeatures);
  const moderationRouteFeatures = getModerationLogFeatures(areaFeatures);
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
  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;

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
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveMuteRole(feature: FeatureRecord, roleId: string) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        role_id: roleId.trim(),
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveDestination(feature: FeatureRecord, channelId: string) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        channel_id: channelId.trim(),
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
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
      moderationCommandFeatures.length === 0 &&
      moderationRouteFeatures.length === 0
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
        moderationCommandFeatures={moderationCommandFeatures}
        muteRoleFeature={muteRoleFeature}
        moderationRouteFeatures={moderationRouteFeatures}
        canEditSelectedGuild={canEditSelectedGuild}
        channelOptions={channelOptions}
        roleOptions={roleOptions}
        messageRouteChannelOptions={messageRouteChannelOptions}
        muteRoleOptions={muteRoleOptions}
        mutation={mutation}
        pendingFeatureId={pendingFeatureId}
        onSaveMuteRole={handleSaveMuteRole}
        onSaveDestination={handleSaveDestination}
        onSetFeatureEnabled={handleSetFeatureEnabled}
      />
    );
  }

  return (
    <section className="page-shell moderation-page">
      <FlatPageLayout
        notice={workspaceNotice}
        workspaceEyebrow={null}
        workspaceTitle={null}
        workspaceDescription={null}
      >
        <div className="moderation-page-intro">
          <div className="card-copy">
            <div className="moderation-page-title-row">
              <h1>{areaLabel}</h1>
            </div>
            <p className="section-description">{definition.description}</p>
          </div>
        </div>

        {renderWorkspaceContent()}
      </FlatPageLayout>
    </section>
  );
}

interface ModerationWorkspacePanelsProps {
  automodFeature: FeatureRecord | null;
  moderationCommandFeatures: FeatureRecord[];
  muteRoleFeature: FeatureRecord | null;
  moderationRouteFeatures: FeatureRecord[];
  canEditSelectedGuild: boolean;
  channelOptions: ReturnType<typeof useGuildChannelOptions>;
  roleOptions: ReturnType<typeof useGuildRoleOptions>;
  messageRouteChannelOptions: Array<{
    value: string;
    label: string;
    description?: string;
  }>;
  muteRoleOptions: Array<{
    value: string;
    label: string;
    disabled?: boolean;
  }>;
  mutation: ReturnType<typeof useFeatureMutation>;
  pendingFeatureId: string;
  onSaveMuteRole: (feature: FeatureRecord, roleId: string) => Promise<void>;
  onSaveDestination: (
    feature: FeatureRecord,
    channelId: string,
  ) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
}

function ModerationWorkspacePanels({
  automodFeature,
  moderationCommandFeatures,
  muteRoleFeature,
  moderationRouteFeatures,
  canEditSelectedGuild,
  channelOptions,
  roleOptions,
  messageRouteChannelOptions,
  muteRoleOptions,
  mutation,
  pendingFeatureId,
  onSaveMuteRole,
  onSaveDestination,
  onSetFeatureEnabled,
}: ModerationWorkspacePanelsProps) {
  const automodHeadingId = useId();

  return (
    <GroupedSettingsStack className="flat-config-stack">
      {automodFeature !== null || muteRoleFeature !== null ? (
        <GroupedSettingsSection>
          <GroupedSettingsGroup>
            {automodFeature !== null ? (
              <GroupedSettingsItem
                role="group"
                aria-labelledby={automodHeadingId}
              >
                <GroupedSettingsSubrow>
                  <GroupedSettingsMainRow>
                    <GroupedSettingsCopy>
                      <GroupedSettingsHeading id={automodHeadingId}>
                        Automod service
                      </GroupedSettingsHeading>
                    </GroupedSettingsCopy>
                    <GroupedSettingsSwitch
                      label="Automod service"
                      checked={automodFeature.effective_enabled}
                      disabled={mutation.saving || !canEditSelectedGuild}
                      onChange={(enabled) =>
                        void onSetFeatureEnabled(automodFeature, enabled)
                      }
                    />
                  </GroupedSettingsMainRow>
                </GroupedSettingsSubrow>
              </GroupedSettingsItem>
            ) : null}

            {muteRoleFeature !== null ? (
              <MuteRoleSection
                feature={muteRoleFeature}
                canEditSelectedGuild={canEditSelectedGuild}
                mutationSaving={mutation.saving}
                pendingFeatureId={pendingFeatureId}
                roleOptions={roleOptions}
                muteRoleOptions={muteRoleOptions}
                onClearNotice={mutation.clearNotice}
                onSave={onSaveMuteRole}
                onSetFeatureEnabled={onSetFeatureEnabled}
              />
            ) : null}
          </GroupedSettingsGroup>
        </GroupedSettingsSection>
      ) : null}

      {moderationCommandFeatures.length > 0 ? (
        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <GroupedSettingsHeading variant="section">
              Moderation commands
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          <GroupedSettingsGroup>
            {moderationCommandFeatures.map((feature) => (
              <ModerationCommandSection
                key={feature.id}
                feature={feature}
                canEditSelectedGuild={canEditSelectedGuild}
                mutationSaving={mutation.saving}
                onSetFeatureEnabled={onSetFeatureEnabled}
              />
            ))}
          </GroupedSettingsGroup>
        </GroupedSettingsSection>
      ) : null}

      <GroupedSettingsSection>
        <GroupedSettingsCopy>
          <GroupedSettingsHeading variant="section">
            Moderation routes
          </GroupedSettingsHeading>
        </GroupedSettingsCopy>

        {channelOptions.notice ? (
          <GroupedSettingsInlineMessage
            message={channelOptions.notice.message}
            tone="error"
            action={
              <button
                className="button-secondary"
                type="button"
                disabled={channelOptions.loading}
                onClick={() => void channelOptions.refresh()}
              >
                Retry channel lookup
              </button>
            }
          />
        ) : null}

        {moderationRouteFeatures.length === 0 ? (
          <div className="table-empty-state table-empty-state-compact">
            <div className="card-copy">
              <h2>No moderation routes</h2>
            </div>
          </div>
        ) : (
          <GroupedSettingsGroup>
            {moderationRouteFeatures.map((feature) => (
              <ModerationRouteSection
                key={feature.id}
                feature={feature}
                canEditSelectedGuild={canEditSelectedGuild}
                channelOptions={channelOptions}
                messageRouteChannelOptions={messageRouteChannelOptions}
                mutationSaving={mutation.saving}
                pendingFeatureId={pendingFeatureId}
                onClearNotice={mutation.clearNotice}
                onSave={onSaveDestination}
                onSetFeatureEnabled={onSetFeatureEnabled}
              />
            ))}
          </GroupedSettingsGroup>
        )}
      </GroupedSettingsSection>
    </GroupedSettingsStack>
  );
}

interface ModerationCommandSectionProps {
  feature: FeatureRecord;
  canEditSelectedGuild: boolean;
  mutationSaving: boolean;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
}

function ModerationCommandSection({
  feature,
  canEditSelectedGuild,
  mutationSaving,
  onSetFeatureEnabled,
}: ModerationCommandSectionProps) {
  const headingId = useId();
  const commandBlockerMessage =
    feature.effective_enabled && feature.readiness === "blocked"
      ? feature.blockers?.[0]?.message ?? null
      : null;

  return (
    <GroupedSettingsItem
      stacked
      role="group"
      aria-labelledby={headingId}
    >
      <GroupedSettingsSubrow>
        <GroupedSettingsMainRow>
          <GroupedSettingsCopy>
            <GroupedSettingsHeading id={headingId}>
              {feature.label}
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>
          <GroupedSettingsSwitch
            label={feature.label}
            checked={feature.effective_enabled}
            disabled={mutationSaving || !canEditSelectedGuild}
            onChange={(enabled) => void onSetFeatureEnabled(feature, enabled)}
          />
        </GroupedSettingsMainRow>
      </GroupedSettingsSubrow>
      {commandBlockerMessage ? (
        <GroupedSettingsSubrow>
          <GroupedSettingsInlineMessage
            message={commandBlockerMessage}
            tone="info"
          />
        </GroupedSettingsSubrow>
      ) : null}
    </GroupedSettingsItem>
  );
}

interface MuteRoleSectionProps {
  feature: FeatureRecord;
  canEditSelectedGuild: boolean;
  mutationSaving: boolean;
  pendingFeatureId: string;
  roleOptions: ReturnType<typeof useGuildRoleOptions>;
  muteRoleOptions: Array<{
    value: string;
    label: string;
    disabled?: boolean;
  }>;
  onClearNotice: () => void;
  onSave: (feature: FeatureRecord, roleId: string) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
}

function MuteRoleSection({
  feature,
  canEditSelectedGuild,
  mutationSaving,
  pendingFeatureId,
  roleOptions,
  muteRoleOptions,
  onClearNotice,
  onSave,
  onSetFeatureEnabled,
}: MuteRoleSectionProps) {
  const currentRoleId = getMuteRoleFeatureDetails(feature).roleId;
  const headingId = useId();
  const [roleDraft, setRoleDraft] = useState(currentRoleId);
  const [muteExpanded, setMuteExpanded] = useState(feature.effective_enabled);
  const canEditRole = canEditSelectedGuild && canEditMuteRole(feature);
  const isMuteTogglePending = mutationSaving && pendingFeatureId === feature.id;
  const muteEnabled = isMuteTogglePending
    ? muteExpanded
    : feature.effective_enabled;
  const hasUnsavedChanges = currentRoleId !== roleDraft.trim();

  useEffect(() => {
    setRoleDraft(currentRoleId);
  }, [currentRoleId]);

  useEffect(() => {
    setMuteExpanded(feature.effective_enabled);
  }, [feature.effective_enabled]);

  function handleReset() {
    onClearNotice();
    setRoleDraft(currentRoleId);
  }

  function handleMuteToggle(enabled: boolean) {
    if (!enabled) {
      onClearNotice();
      setRoleDraft(currentRoleId);
    }

    setMuteExpanded(enabled);
    void onSetFeatureEnabled(feature, enabled);
  }

  return (
    <GroupedSettingsItem
      stacked
      role="group"
      aria-labelledby={headingId}
    >
      <GroupedSettingsSubrow>
        <GroupedSettingsMainRow>
          <GroupedSettingsCopy>
            <GroupedSettingsHeading id={headingId}>Mute command</GroupedSettingsHeading>
          </GroupedSettingsCopy>
          <GroupedSettingsSwitch
            label="Mute command"
            checked={muteEnabled}
            disabled={mutationSaving || !canEditSelectedGuild}
            onChange={handleMuteToggle}
          />
        </GroupedSettingsMainRow>
      </GroupedSettingsSubrow>

      {muteEnabled ? (
        <GroupedSettingsSubrow>
          <SettingsSelectField
            label="Mute role"
            labelClassName="grouped-settings-label"
            value={roleDraft}
            disabled={!canEditRole || roleOptions.loading}
            onChange={setRoleDraft}
            options={muteRoleOptions}
            placeholder={
              roleOptions.loading
                ? "Loading roles..."
                : muteRoleOptions.length === 0
                  ? "No roles available"
                  : "No mute role"
            }
          />
        </GroupedSettingsSubrow>
      ) : null}

      {muteEnabled && roleOptions.notice ? (
        <GroupedSettingsSubrow>
          <GroupedSettingsInlineMessage
            message={roleOptions.notice.message}
            tone="error"
            action={
              <button
                className="button-secondary"
                type="button"
                disabled={roleOptions.loading}
                onClick={() => void roleOptions.refresh()}
              >
                Retry role lookup
              </button>
            }
          />
        </GroupedSettingsSubrow>
      ) : null}

      {muteEnabled && hasUnsavedChanges ? (
        <GroupedSettingsSubrow>
          <UnsavedChangesBar
            className="grouped-settings-unsaved-bar"
            hasUnsavedChanges={hasUnsavedChanges}
            saveLabel={
              mutationSaving && pendingFeatureId === feature.id
                ? "Saving..."
                : "Save changes"
            }
            saving={mutationSaving && pendingFeatureId === feature.id}
            disabled={!canEditRole || roleOptions.loading}
            onReset={handleReset}
            onSave={() => onSave(feature, roleDraft)}
          />
        </GroupedSettingsSubrow>
      ) : null}
    </GroupedSettingsItem>
  );
}

interface ModerationRouteSectionProps {
  feature: FeatureRecord;
  canEditSelectedGuild: boolean;
  channelOptions: ReturnType<typeof useGuildChannelOptions>;
  messageRouteChannelOptions: Array<{
    value: string;
    label: string;
    description?: string;
  }>;
  mutationSaving: boolean;
  pendingFeatureId: string;
  onClearNotice: () => void;
  onSave: (feature: FeatureRecord, channelId: string) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
}

function ModerationRouteSection({
  feature,
  canEditSelectedGuild,
  channelOptions,
  messageRouteChannelOptions,
  mutationSaving,
  pendingFeatureId,
  onClearNotice,
  onSave,
  onSetFeatureEnabled,
}: ModerationRouteSectionProps) {
  const currentChannelId = getLoggingFeatureDetails(feature).channelId;
  const headingId = useId();
  const [channelDraft, setChannelDraft] = useState(currentChannelId);
  const canEditDestination =
    canEditSelectedGuild && canEditLoggingChannel(feature);
  const hasUnsavedChanges = currentChannelId !== channelDraft.trim();
  const routeMessage =
    feature.effective_enabled && feature.readiness === "blocked"
      ? summarizeLoggingGuidance(feature)
      : null;

  useEffect(() => {
    setChannelDraft(currentChannelId);
  }, [currentChannelId]);

  function handleReset() {
    onClearNotice();
    setChannelDraft(currentChannelId);
  }

  return (
    <GroupedSettingsItem
      stacked
      role="group"
      aria-labelledby={headingId}
    >
      <GroupedSettingsSubrow>
        <GroupedSettingsMainRow>
          <GroupedSettingsCopy>
            <GroupedSettingsHeading id={headingId}>
              {feature.label}
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>
          <GroupedSettingsSwitch
            label={feature.label}
            checked={feature.effective_enabled}
            disabled={mutationSaving || !canEditSelectedGuild}
            onChange={(enabled) => void onSetFeatureEnabled(feature, enabled)}
          />
        </GroupedSettingsMainRow>
      </GroupedSettingsSubrow>

      <GroupedSettingsSubrow>
        <SettingsSelectField
          label="Channel"
          labelClassName="grouped-settings-label"
          value={channelDraft}
          disabled={!canEditDestination || channelOptions.loading}
          onChange={setChannelDraft}
          options={messageRouteChannelOptions}
          placeholder={
            channelOptions.loading
              ? "Loading channels..."
              : messageRouteChannelOptions.length === 0
                ? "No channels available"
              : "No channel"
          }
        />
      </GroupedSettingsSubrow>

      {routeMessage ? (
        <GroupedSettingsSubrow>
          <GroupedSettingsInlineMessage message={routeMessage} tone="error" />
        </GroupedSettingsSubrow>
      ) : null}

      {hasUnsavedChanges ? (
        <GroupedSettingsSubrow>
          <UnsavedChangesBar
            className="grouped-settings-unsaved-bar"
            hasUnsavedChanges={hasUnsavedChanges}
            saveLabel={
              mutationSaving && pendingFeatureId === feature.id
                ? "Saving..."
                : "Save changes"
            }
            saving={mutationSaving && pendingFeatureId === feature.id}
            disabled={!canEditDestination || channelOptions.loading}
            onReset={handleReset}
            onSave={() => onSave(feature, channelDraft)}
          />
        </GroupedSettingsSubrow>
      ) : null}
    </GroupedSettingsItem>
  );
}
