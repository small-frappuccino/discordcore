import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  EmptyState,
  FlatPageLayout,
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
  const moderationLogFeatures = getModerationLogFeatures(areaFeatures);
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
        moderationLogFeatures={moderationLogFeatures}
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
          </div>
        </div>

        {renderWorkspaceContent()}
      </FlatPageLayout>
    </section>
  );
}

interface ModerationWorkspacePanelsProps {
  automodFeature: FeatureRecord | null;
  muteRoleFeature: FeatureRecord | null;
  moderationLogFeatures: FeatureRecord[];
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
  muteRoleFeature,
  moderationLogFeatures,
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
  return (
    <div className="flat-config-stack moderation-flat-stack moderation-group-stack">
      {automodFeature !== null || muteRoleFeature !== null ? (
        <section className="moderation-group-block">
          <div className="moderation-settings-group">
            {automodFeature !== null ? (
              <div className="moderation-settings-item">
                <div className="moderation-settings-subrow">
                  <div className="moderation-setting-row">
                    <div className="card-copy moderation-section-copy">
                      <h2 className="moderation-section-title">Automod service</h2>
                    </div>
                    <ModerationSwitch
                      label="Automod service"
                      checked={automodFeature.effective_enabled}
                      disabled={mutation.saving || !canEditSelectedGuild}
                      onChange={(enabled) =>
                        void onSetFeatureEnabled(automodFeature, enabled)
                      }
                    />
                  </div>
                </div>
              </div>
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
          </div>
        </section>
      ) : null}

      <section className="moderation-group-block moderation-log-panel">
        <div className="card-copy moderation-section-copy">
          <h2 className="moderation-section-title">Moderation routes</h2>
        </div>

        {channelOptions.notice ? (
          <ModerationInlineMessage
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

        {moderationLogFeatures.length === 0 ? (
          <div className="table-empty-state table-empty-state-compact">
            <div className="card-copy">
              <h2>No moderation routes</h2>
            </div>
          </div>
        ) : (
          <div className="moderation-settings-group">
            {moderationLogFeatures.map((feature) => (
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
          </div>
        )}
      </section>
    </div>
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
    <div className="moderation-settings-item moderation-settings-item-stack">
      <div className="moderation-settings-subrow">
        <div className="moderation-setting-row">
          <div className="card-copy moderation-section-copy">
            <h2 className="moderation-section-title">Mute command</h2>
          </div>
          <ModerationSwitch
            label="Mute command"
            checked={muteEnabled}
            disabled={mutationSaving || !canEditSelectedGuild}
            onChange={handleMuteToggle}
          />
        </div>
      </div>

      {muteEnabled ? (
        <div className="moderation-settings-subrow">
          <SettingsSelectField
            label="Mute role"
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
        </div>
      ) : null}

      {muteEnabled && roleOptions.notice ? (
        <div className="moderation-settings-subrow">
          <ModerationInlineMessage
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
        </div>
      ) : null}

      {muteEnabled && hasUnsavedChanges ? (
        <div className="moderation-settings-subrow">
          <UnsavedChangesBar
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
        </div>
      ) : null}
    </div>
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
    <div className="moderation-settings-item moderation-settings-item-stack moderation-route-section">
      <div className="moderation-settings-subrow">
        <div className="moderation-setting-row">
          <div className="card-copy moderation-section-copy">
            <h3 className="moderation-section-title moderation-route-title">
              {feature.label}
            </h3>
          </div>
          <ModerationSwitch
            label={feature.label}
            checked={feature.effective_enabled}
            disabled={mutationSaving || !canEditSelectedGuild}
            onChange={(enabled) => void onSetFeatureEnabled(feature, enabled)}
          />
        </div>
      </div>

      <div className="moderation-settings-subrow">
        <SettingsSelectField
          label="Channel"
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
      </div>

      {routeMessage ? (
        <div className="moderation-settings-subrow">
          <ModerationInlineMessage message={routeMessage} tone="error" />
        </div>
      ) : null}

      {hasUnsavedChanges ? (
        <div className="moderation-settings-subrow">
          <UnsavedChangesBar
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
        </div>
      ) : null}
    </div>
  );
}

interface ModerationInlineMessageProps {
  message: string;
  tone?: "info" | "error";
  action?: ReactNode;
}

function ModerationInlineMessage({
  message,
  tone = "error",
  action,
}: ModerationInlineMessageProps) {
  return (
    <div className="flat-inline-message moderation-inline-message">
      <p className={`meta-note moderation-inline-message-copy tone-${tone}`}>
        {message}
      </p>
      {action ? <div className="inline-actions">{action}</div> : null}
    </div>
  );
}

interface ModerationSwitchProps {
  label: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}

function ModerationSwitch({
  label,
  checked,
  disabled = false,
  onChange,
}: ModerationSwitchProps) {
  return (
    <label
      className={`moderation-switch${checked ? " is-checked" : ""}${disabled ? " is-disabled" : ""}`}
    >
      <input
        aria-label={label}
        checked={checked}
        disabled={disabled}
        type="checkbox"
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className="moderation-switch-track" aria-hidden="true">
        <span className="moderation-switch-thumb" />
      </span>
    </label>
  );
}
