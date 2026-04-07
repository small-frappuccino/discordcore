import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  AdvancedTextInput,
  EmptyState,
  EntityPickerField,
  FlatPageLayout,
  KeyValueList,
  LookupNotice,
  StatusBadge,
  UnsavedChangesBar,
} from "../components/ui";
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
  getLoggingFeatureDetails,
  summarizeLoggingDestination,
  summarizeLoggingGuidance,
} from "../features/features/logging";
import {
  canEditMuteRole,
  formatAutomodModeValue,
  getModerationLogFeatures,
  getMuteRoleFeatureDetails,
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
  const areaSummary = summarizeFeatureArea(areaFeatures);
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
        onUseDefault={handleUseDefault}
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
              <StatusBadge
                tone={
                  workspace.workspaceState === "ready" ? areaSummary.tone : "info"
                }
              >
                {workspace.workspaceState === "ready"
                  ? areaSummary.label
                  : formatWorkspaceStateTitle(areaLabel, workspace.workspaceState)}
              </StatusBadge>
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
  onUseDefault: (feature: FeatureRecord) => Promise<void>;
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
  onUseDefault,
}: ModerationWorkspacePanelsProps) {
  const hasConfiguredModerationRoute = moderationLogFeatures.some((feature) => {
    const details = getLoggingFeatureDetails(feature);
    return details.channelId !== "" || !details.requiresChannel;
  });
  const hasModerationRouteBlocker = moderationLogFeatures.some(
    (feature) => feature.readiness === "blocked",
  );

  return (
    <div className="flat-config-stack moderation-flat-stack">
      {automodFeature !== null ? (
        <section className="flat-config-section moderation-flat-section moderation-service-panel">
          <div className="flat-config-header">
            <div className="card-copy flat-config-copy moderation-section-copy">
              <p className="section-label">Moderation</p>
              <div className="flat-config-title-row moderation-title-row">
                <h2>{automodFeature.label}</h2>
                <StatusBadge tone={getFeatureStatusTone(automodFeature)}>
                  {formatFeatureStatusLabel(automodFeature)}
                </StatusBadge>
              </div>
            <p className="section-description">
              Keep the service toggle visible here and use the moderation routes
              below for destination-specific configuration.
            </p>
          </div>
          </div>

          <KeyValueList
            items={[
              {
                label: "Mode",
                value: formatAutomodModeValue(automodFeature),
              },
              {
                label: "Current signal",
                value: summarizeAutomodSignal(automodFeature),
              },
            ]}
          />

          <div className="inline-actions moderation-service-actions flat-config-actions">
            <button
              className="button-primary"
              type="button"
              disabled={mutation.saving || !canEditSelectedGuild}
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
                disabled={mutation.saving || !canEditSelectedGuild}
                onClick={() => void onUseDefault(automodFeature)}
              >
                Use default
              </button>
            ) : null}
          </div>
        </section>
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
          onUseDefault={onUseDefault}
        />
      ) : null}

      <section className="flat-config-section moderation-flat-section moderation-log-panel">
        <div className="flat-config-header">
          <div className="card-copy flat-config-copy moderation-section-copy">
            <p className="section-label">Moderation routes</p>
            <div className="flat-config-title-row moderation-title-row">
              <h2>Route destinations</h2>
              <StatusBadge
                tone={
                  hasModerationRouteBlocker
                    ? "error"
                    : hasConfiguredModerationRoute
                      ? "success"
                      : "neutral"
                }
              >
                {moderationLogFeatures.length === 0
                  ? "Not mapped"
                  : hasModerationRouteBlocker
                    ? "Needs attention"
                    : hasConfiguredModerationRoute
                      ? "Ready"
                      : "Needs setup"}
              </StatusBadge>
            </div>
            <p className="section-description">
              Edit moderation case and route destinations directly here instead of
              opening a separate route drawer.
            </p>
          </div>
        </div>

        {channelOptions.notice ? (
          <LookupNotice
            title="Channel references unavailable"
            message={channelOptions.notice.message}
            retryLabel="Retry channel lookup"
            retryDisabled={channelOptions.loading}
            onRetry={channelOptions.refresh}
          />
        ) : null}

        {moderationLogFeatures.length === 0 ? (
          <div className="table-empty-state table-empty-state-compact">
            <div className="card-copy">
              <h2>No moderation routes</h2>
              <p className="section-description">
                No moderation routes are exposed yet.
              </p>
            </div>
          </div>
        ) : (
          <div className="flat-config-stack">
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
                onUseDefault={onUseDefault}
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
  onUseDefault: (feature: FeatureRecord) => Promise<void>;
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
  onUseDefault,
}: MuteRoleSectionProps) {
  const currentRoleId = getMuteRoleFeatureDetails(feature).roleId;
  const [roleDraft, setRoleDraft] = useState(currentRoleId);
  const canEditRole = canEditSelectedGuild && canEditMuteRole(feature);
  const hasUnsavedChanges = currentRoleId !== roleDraft.trim();

  useEffect(() => {
    setRoleDraft(currentRoleId);
  }, [currentRoleId]);

  function handleReset() {
    onClearNotice();
    setRoleDraft(currentRoleId);
  }

  return (
    <section className="flat-config-section moderation-flat-section moderation-service-panel">
      <div className="flat-config-header">
        <div className="card-copy flat-config-copy moderation-section-copy">
          <p className="section-label">Role-based mute</p>
          <div className="flat-config-title-row moderation-title-row">
            <h2>{feature.label}</h2>
            <StatusBadge tone={getFeatureStatusTone(feature)}>
              {formatFeatureStatusLabel(feature)}
            </StatusBadge>
          </div>
          <p className="section-description">
            Keep the mute role selector visible in the workspace so the moderation
            flow does not depend on a separate editor panel.
          </p>
        </div>
      </div>

      <MuteRoleDrawerBody
        selectedFeature={feature}
        roleDraft={roleDraft}
        setRoleDraft={setRoleDraft}
        muteRoleOptions={muteRoleOptions}
        roleOptions={roleOptions}
        disabled={!canEditRole}
      />

      <div className="inline-actions moderation-service-actions flat-config-actions">
        <button
          className="button-secondary"
          type="button"
          disabled={mutationSaving || !canEditSelectedGuild}
          onClick={() => void onSetFeatureEnabled(feature, !feature.effective_enabled)}
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
            onClick={() => void onUseDefault(feature)}
          >
            Use inherited
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
        disabled={!canEditRole || roleOptions.loading}
        onReset={handleReset}
        onSave={() => onSave(feature, roleDraft)}
      />
    </section>
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
  onUseDefault: (feature: FeatureRecord) => Promise<void>;
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
  onUseDefault,
}: ModerationRouteSectionProps) {
  const currentChannelId = getLoggingFeatureDetails(feature).channelId;
  const [channelDraft, setChannelDraft] = useState(currentChannelId);
  const canEditDestination =
    canEditSelectedGuild && canEditLoggingChannel(feature);
  const hasUnsavedChanges = currentChannelId !== channelDraft.trim();

  useEffect(() => {
    setChannelDraft(currentChannelId);
  }, [currentChannelId]);

  function handleReset() {
    onClearNotice();
    setChannelDraft(currentChannelId);
  }

  return (
    <section className="flat-config-section moderation-flat-section">
      <div className="flat-config-header">
        <div className="card-copy flat-config-copy moderation-section-copy">
          <div className="flat-config-title-row moderation-title-row">
            <h3>{feature.label}</h3>
            <StatusBadge tone={getFeatureStatusTone(feature)}>
              {formatFeatureStatusLabel(feature)}
            </StatusBadge>
          </div>
          <p className="section-description">{feature.description}</p>
        </div>
      </div>

      <ModerationDestinationDrawerBody
        selectedFeature={feature}
        channelDraft={channelDraft}
        setChannelDraft={setChannelDraft}
        channelOptions={channelOptions}
        messageRouteChannelOptions={messageRouteChannelOptions}
        disabled={!canEditDestination}
        showLookupNotice={false}
      />

      <div className="inline-actions flat-config-actions">
        <button
          className="button-secondary"
          type="button"
          disabled={mutationSaving || !canEditSelectedGuild}
          aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
          onClick={() => void onSetFeatureEnabled(feature, !feature.effective_enabled)}
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
            aria-label={`Use inherited setting for ${feature.label}`}
            onClick={() => void onUseDefault(feature)}
          >
            Use inherited
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
        disabled={!canEditDestination || channelOptions.loading}
        onReset={handleReset}
        onSave={() => onSave(feature, channelDraft)}
      />
    </section>
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
  disabled?: boolean;
}

function MuteRoleDrawerBody({
  selectedFeature,
  roleDraft,
  setRoleDraft,
  muteRoleOptions,
  roleOptions,
  disabled = false,
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

      <div className="flat-config-fields">
        <EntityPickerField
          label="Mute role"
          value={roleDraft}
          disabled={disabled || roleOptions.loading}
          onChange={setRoleDraft}
          options={muteRoleOptions}
          placeholder={
            roleOptions.loading
              ? "Loading roles..."
              : muteRoleOptions.length === 0
                ? "No roles available"
                : "No mute role"
          }
          note="Choose a role the bot can assign."
        />

        <AdvancedTextInput
          label="Mute role ID fallback"
          inputLabel="Mute role ID fallback"
          value={roleDraft}
          disabled={disabled}
          onChange={setRoleDraft}
          placeholder="Discord role ID"
          note="Use only if role lookup fails."
        />
      </div>

      {roleOptions.notice ? (
        <LookupNotice
          title="Role references unavailable"
          message={roleOptions.notice.message}
          retryLabel="Retry role lookup"
          retryDisabled={roleOptions.loading}
          onRetry={roleOptions.refresh}
        />
      ) : null}
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
  disabled?: boolean;
  showLookupNotice?: boolean;
}

function ModerationDestinationDrawerBody({
  selectedFeature,
  channelDraft,
  setChannelDraft,
  channelOptions,
  messageRouteChannelOptions,
  disabled = false,
  showLookupNotice = true,
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
            label: "Destination",
            value: formatGuildChannelValue(
              getLoggingFeatureDetails(selectedFeature).channelId,
              channelOptions.channels,
              summarizeLoggingDestination(selectedFeature),
            ),
          },
          {
            label: "Current signal",
            value: summarizeLoggingGuidance(selectedFeature),
          },
        ]}
      />

      <div className="flat-config-fields">
        <EntityPickerField
          label="Destination channel"
          value={channelDraft}
          disabled={disabled || channelOptions.loading}
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
            getLoggingFeatureDetails(selectedFeature).requiresChannel
              ? undefined
              : "Leave empty to clear the destination."
          }
        />

        <AdvancedTextInput
          label="Channel ID fallback"
          inputLabel="Destination channel ID fallback"
          value={channelDraft}
          disabled={disabled}
          onChange={setChannelDraft}
          placeholder="Discord channel ID"
          note="Use only if channel lookup fails."
        />
      </div>

      {showLookupNotice && channelOptions.notice ? (
        <LookupNotice
          title="Channel references unavailable"
          message={channelOptions.notice.message}
          retryLabel="Retry channel lookup"
          retryDisabled={channelOptions.loading}
          onRetry={channelOptions.refresh}
        />
      ) : null}
    </>
  );
}
