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
import {
  AdvancedTextInput,
  AlertBanner,
  EntityPickerField,
  EmptyState,
  FeatureWorkspaceLayout,
  KeyValueList,
  LookupNotice,
  PageHeader,
  StatusBadge,
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
          description="Manage the AutoMod listener, mute role, and moderation routes."
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
          busyLabel={mutation.saving ? "Saving moderation settings..." : undefined}
          workspaceEyebrow={null}
          workspaceTitle={null}
          workspaceDescription={null}
          workspaceContent={renderWorkspaceContent()}
        />
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
    <div className="moderation-flat-stack">
      {automodFeature !== null ? (
        <section className="moderation-flat-section moderation-service-panel">
          <div className="flat-inline-message">
            <div className="card-copy moderation-section-copy">
              <div className="moderation-title-row">
                <h3>{automodFeature.label}</h3>
                <StatusBadge tone={getFeatureStatusTone(automodFeature)}>
                  {formatFeatureStatusLabel(automodFeature)}
                </StatusBadge>
              </div>
            </div>

            <p className="meta-note">
              {automodFeature.override_state === "inherit"
                ? "Using default"
                : "Configured here"}
            </p>
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
        <section className="moderation-flat-section moderation-service-panel">
          <div className="flat-inline-message">
            <div className="card-copy moderation-section-copy">
              <div className="moderation-title-row">
                <h3>{muteRoleFeature.label}</h3>
                <StatusBadge tone={getFeatureStatusTone(muteRoleFeature)}>
                  {formatFeatureStatusLabel(muteRoleFeature)}
                </StatusBadge>
              </div>
            </div>

            <p className="meta-note">
              {muteRoleFeature.override_state === "inherit"
                ? "Using default"
                : "Configured here"}
            </p>
          </div>

          <KeyValueList
            items={[
              {
                label: "Configured role",
                value: formatRoleValue(muteRoleId, roleOptions.roles),
              },
              {
                label: "Current signal",
                value: summarizeMuteRoleSignal(muteRoleFeature),
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

      <section className="moderation-flat-section moderation-log-panel">
        <div className="flat-inline-message">
          <div className="card-copy moderation-section-copy">
            <div className="moderation-title-row">
              <h3>Moderation routes</h3>
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
                        <strong>{feature.label}</strong>
                      </td>
                      <td>
                        <strong>
                          {formatGuildChannelValue(
                            details.channelId,
                            channelOptions.channels,
                            summarizeLoggingDestination(feature),
                          )}
                        </strong>
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
        note="Choose a role the bot can assign."
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
        note="Use only if role lookup fails."
      />
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
        note={
          getLoggingFeatureDetails(selectedFeature).requiresChannel
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
    </>
  );
}
