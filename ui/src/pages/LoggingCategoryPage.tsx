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
  PageHeader,
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
  formatEffectiveSourceLabel,
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";

export function LoggingCategoryPage() {
  const definition = getFeatureAreaDefinition("logging");
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
  const [pendingFeatureId, setPendingFeatureId] = useState("");

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "logging");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const firstBlockedFeature = useMemo(
    () => areaFeatures.find((feature) => feature.readiness === "blocked") ?? null,
    [areaFeatures],
  );
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
      <>
        {firstBlockedFeature ? (
          <div className="surface-subsection">
            <p className="section-label">Current blocker</p>
            <strong>{firstBlockedFeature.label}</strong>
            <p className="meta-note">
              {summarizeLoggingGuidance(firstBlockedFeature)}
            </p>
          </div>
        ) : null}

        {channelOptions.notice ? (
          <LookupNotice
            as="section"
            title="Channel references unavailable"
            message={channelOptions.notice.message}
            retryLabel="Retry channel lookup"
            retryDisabled={channelOptions.loading}
            onRetry={channelOptions.refresh}
          />
        ) : null}

        <div className="flat-config-stack">
          {areaFeatures.map((feature) => (
            <LoggingRouteSection
              key={feature.id}
              feature={feature}
              availableChannels={channelOptions.channels}
              channelOptions={messageRouteChannelOptions}
              channelOptionsLoading={channelOptions.loading}
              canEditSelectedGuild={canEditSelectedGuild}
              mutationSaving={mutation.saving}
              pendingFeatureId={pendingFeatureId}
              onClearNotice={mutation.clearNotice}
              onSave={handleSaveDestination}
              onSetFeatureEnabled={handleSetFeatureEnabled}
              onUseInherited={handleUseInherited}
            />
          ))}
        </div>
      </>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Feature area"
        title={areaLabel}
        description="Configure event log routes for the selected server, keep destinations valid, and resolve blockers before relying on operational logging."
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
        actions={renderHeaderActions()}
      />

      <FlatPageLayout
        notice={workspaceNotice}
        workspaceTitle="Manage logging routes"
        workspaceDescription="Keep destinations and route readiness visible in the main workspace instead of using a separate destination drawer."
      >
        {renderWorkspaceContent()}
      </FlatPageLayout>
    </section>
  );
}

interface LoggingRouteSectionProps {
  feature: FeatureRecord;
  availableChannels: ReturnType<typeof useGuildChannelOptions>["channels"];
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
  availableChannels,
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
  const [channelDraft, setChannelDraft] = useState(details.channelId);
  const canEditDestination =
    canEditSelectedGuild && canEditLoggingChannel(feature);
  const hasUnsavedChanges = details.channelId !== channelDraft.trim();

  useEffect(() => {
    setChannelDraft(details.channelId);
  }, [details.channelId]);

  function handleReset() {
    onClearNotice();
    setChannelDraft(details.channelId);
  }

  return (
    <section className="flat-config-section">
      <div className="flat-config-header">
        <div className="card-copy flat-config-copy">
          <p className="section-label">Log route</p>
          <div className="flat-config-title-row">
            <h2>{feature.label}</h2>
            <StatusBadge tone={getFeatureStatusTone(feature)}>
              {formatFeatureStatusLabel(feature)}
            </StatusBadge>
          </div>
          <p className="section-description">{feature.description}</p>
        </div>

        <div className="flat-config-status">
          {details.exclusiveModerationChannel ? (
            <span className="meta-note">
              Requires an exclusive moderation destination.
            </span>
          ) : null}
        </div>
      </div>

      <KeyValueList
        items={[
          {
            label: "Destination",
            value: formatGuildChannelValue(
              details.channelId,
              availableChannels,
              summarizeLoggingDestination(feature),
            ),
          },
          {
            label: "Applied from",
            value: formatEffectiveSourceLabel(feature.effective_source),
          },
          {
            label: "Destination rule",
            value: details.requiresChannel
              ? "Needs destination channel"
              : "No dedicated destination",
          },
          {
            label: "Current signal",
            value: summarizeLoggingGuidance(feature),
          },
        ]}
      />

      <div className="flat-config-fields">
        <EntityPickerField
          label="Destination channel"
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
          note={
            details.requiresChannel
              ? undefined
              : "Leave empty to clear the destination."
          }
        />

        <AdvancedTextInput
          label="Channel ID fallback"
          inputLabel="Destination channel ID fallback"
          value={channelDraft}
          disabled={!canEditDestination}
          onChange={setChannelDraft}
          placeholder="Discord channel ID"
          note="Use only if channel lookup fails."
        />
      </div>

      <div className="inline-actions flat-config-actions">
        <button
          className="button-secondary"
          type="button"
          disabled={mutationSaving || !canEditSelectedGuild}
          aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
          onClick={() =>
            void onSetFeatureEnabled(feature, !feature.effective_enabled)
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
            aria-label={`Use inherited setting for ${feature.label}`}
            onClick={() => void onUseInherited(feature)}
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
        disabled={!canEditDestination}
        onReset={handleReset}
        onSave={() => onSave(feature, channelDraft)}
      />
    </section>
  );
}
