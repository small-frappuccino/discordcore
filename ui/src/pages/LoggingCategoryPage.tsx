import { useEffect, useId, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import {
  AdvancedTextInput,
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
  getLoggingFeatureDetails,
  summarizeLoggingGuidance,
} from "../features/features/logging";
import { featureSupportsField } from "../features/features/model";
import {
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
} from "../features/features/presentation";
import { shouldRenderDashboardDiagnosticField } from "../features/features/presentationPolicy";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";

export function LoggingCategoryPage() {
  const definition = getFeatureAreaDefinition("logging");
  const location = useLocation();
  const { authState, beginLogin, canEditSelectedGuild } = useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const channelOptions = useGuildChannelOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "logging");
  const workspaceNotice = mutation.notice ?? workspace.notice;
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
        workspace.updateFeature(updated);
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
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveDestination(
    feature: FeatureRecord,
    channelId: string,
  ) {
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
      <LoggingWorkspacePanels
        areaFeatures={areaFeatures}
        canEditSelectedGuild={canEditSelectedGuild}
        channelOptions={channelOptions}
        messageRouteChannelOptions={messageRouteChannelOptions}
        mutation={mutation}
        pendingFeatureId={pendingFeatureId}
        onSaveDestination={handleSaveDestination}
        onSetFeatureEnabled={handleSetFeatureEnabled}
        onUseInherited={handleUseInherited}
      />
    );
  }

  return (
    <section className="page-shell logging-page">
      <FlatPageLayout
        notice={workspaceNotice}
        workspaceEyebrow={null}
        workspaceTitle={null}
        workspaceDescription={null}
      >
        <div className="card-copy">
          <div className="page-title-row">
            <h1>{areaLabel}</h1>
          </div>
        </div>

        {renderWorkspaceContent()}
      </FlatPageLayout>
    </section>
  );
}

interface LoggingWorkspacePanelsProps {
  areaFeatures: FeatureRecord[];
  canEditSelectedGuild: boolean;
  channelOptions: ReturnType<typeof useGuildChannelOptions>;
  messageRouteChannelOptions: Array<{
    value: string;
    label: string;
    description?: string;
  }>;
  mutation: ReturnType<typeof useFeatureMutation>;
  pendingFeatureId: string;
  onSaveDestination: (
    feature: FeatureRecord,
    channelId: string,
  ) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
  onUseInherited: (feature: FeatureRecord) => Promise<void>;
}

function LoggingWorkspacePanels({
  areaFeatures,
  canEditSelectedGuild,
  channelOptions,
  messageRouteChannelOptions,
  mutation,
  pendingFeatureId,
  onSaveDestination,
  onSetFeatureEnabled,
  onUseInherited,
}: LoggingWorkspacePanelsProps) {
  return (
    <GroupedSettingsStack className="flat-config-stack">
      <GroupedSettingsSection>
        <GroupedSettingsCopy>
          <GroupedSettingsHeading variant="section">
            Logging routes
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

        <GroupedSettingsGroup>
          {areaFeatures.map((feature) => (
            <LoggingRouteSection
              key={feature.id}
              feature={feature}
              channelOptions={messageRouteChannelOptions}
              channelOptionsLoading={channelOptions.loading}
              canEditSelectedGuild={canEditSelectedGuild}
              mutationSaving={mutation.saving}
              pendingFeatureId={pendingFeatureId}
              onClearNotice={mutation.clearNotice}
              onSave={onSaveDestination}
              onSetFeatureEnabled={onSetFeatureEnabled}
              onUseInherited={onUseInherited}
            />
          ))}
        </GroupedSettingsGroup>
      </GroupedSettingsSection>
    </GroupedSettingsStack>
  );
}

interface LoggingRouteSectionProps {
  feature: FeatureRecord;
  channelOptions: Array<{ value: string; label: string; description?: string }>;
  channelOptionsLoading: boolean;
  canEditSelectedGuild: boolean;
  mutationSaving: boolean;
  pendingFeatureId: string;
  onClearNotice: () => void;
  onSave: (feature: FeatureRecord, channelId: string) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
  onUseInherited: (feature: FeatureRecord) => Promise<void>;
}

function LoggingRouteSection({
  feature,
  channelOptions,
  channelOptionsLoading,
  canEditSelectedGuild,
  mutationSaving,
  pendingFeatureId,
  onClearNotice,
  onSave,
  onSetFeatureEnabled,
  onUseInherited,
}: LoggingRouteSectionProps) {
  const details = getLoggingFeatureDetails(feature);
  const headingId = useId();
  const [channelDraft, setChannelDraft] = useState(details.channelId);
  const canEditDestination =
    canEditSelectedGuild && featureSupportsField(feature, "channel_id");
  const hasUnsavedChanges = details.channelId !== channelDraft.trim();
  const isPending = mutationSaving && pendingFeatureId === feature.id;
  const routeMessage =
    feature.effective_enabled && feature.readiness === "blocked"
      ? summarizeLoggingGuidance(feature)
      : null;
  const channelNote = details.exclusiveModerationChannel
    ? "Use a dedicated moderation channel for this route."
    : details.requiresChannel
      ? undefined
      : "Leave empty to clear the destination.";

  useEffect(() => {
    setChannelDraft(details.channelId);
  }, [details.channelId]);

  function handleReset() {
    onClearNotice();
    setChannelDraft(details.channelId);
  }

  return (
    <GroupedSettingsItem stacked role="group" aria-labelledby={headingId}>
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
          label="Destination channel"
          labelClassName="grouped-settings-label"
          value={channelDraft}
          disabled={!canEditDestination || channelOptionsLoading}
          onChange={setChannelDraft}
          options={channelOptions}
          placeholder={
            channelOptionsLoading
              ? "Loading channels..."
              : channelOptions.length === 0
                ? "No channels available"
                : "No destination channel"
          }
          note={channelNote}
        />
      </GroupedSettingsSubrow>

      {routeMessage ? (
        <GroupedSettingsSubrow>
          <GroupedSettingsInlineMessage message={routeMessage} tone="error" />
        </GroupedSettingsSubrow>
      ) : null}

      {feature.override_state !== "inherit" ? (
        <GroupedSettingsSubrow>
          <div className="inline-actions">
            <button
              className="button-ghost"
              type="button"
              disabled={mutationSaving || !canEditSelectedGuild}
              aria-label={`Use inherited setting for ${feature.label}`}
              onClick={() => void onUseInherited(feature)}
            >
              Use inherited
            </button>
          </div>
        </GroupedSettingsSubrow>
      ) : null}

      {shouldRenderDashboardDiagnosticField("Channel ID fallback") ? (
        <GroupedSettingsSubrow>
          <AdvancedTextInput
            label="Channel ID fallback"
            inputLabel="Destination channel ID fallback"
            value={channelDraft}
            disabled={!canEditDestination}
            onChange={setChannelDraft}
            placeholder="Discord channel ID"
            note="Use only if channel lookup fails."
          />
        </GroupedSettingsSubrow>
      ) : null}

      {hasUnsavedChanges ? (
        <GroupedSettingsSubrow>
          <UnsavedChangesBar
            className="grouped-settings-unsaved-bar"
            hasUnsavedChanges={hasUnsavedChanges}
            saveLabel={isPending ? "Saving..." : "Save changes"}
            saving={isPending}
            disabled={!canEditDestination}
            onReset={handleReset}
            onSave={() => onSave(feature, channelDraft)}
          />
        </GroupedSettingsSubrow>
      ) : null}
    </GroupedSettingsItem>
  );
}
