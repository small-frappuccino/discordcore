import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import type {
  FeatureRecord,
  GuildChannelOption,
  GuildRoleOption,
} from "../api/control";
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
  canEditAdminCommands,
  canEditCommandsChannel,
  formatAllowedRoleCountValue,
  formatAllowedRoleOptionLabel,
  formatAllowedRolesValue,
  formatCommandServerSetting,
  getAdminCommandsFeatureDetails,
  getCommandsFeatureDetails,
  summarizeAdminCommandsSignal,
  summarizeCommandsSignal,
  toggleAllowedRole,
} from "../features/features/commands";
import {
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";
import {
  AdvancedTextInput,
  AlertBanner,
  EntityMultiPickerField,
  EntityPickerField,
  EmptyState,
  KeyValueList,
  LookupNotice,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";

export function CommandsPage() {
  const definition = getFeatureAreaDefinition("commands");
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
  const [allowedRoleIdsDraft, setAllowedRoleIdsDraft] = useState<string[]>([]);

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "commands");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const commandsFeature =
    areaFeatures.find((feature) => feature.id === "services.commands") ?? null;
  const adminCommandsFeature =
    areaFeatures.find((feature) => feature.id === "services.admin_commands") ?? null;
  const selectedFeature =
    areaFeatures.find((feature) => feature.id === selectedFeatureId) ?? null;
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;
  const enabledModules = areaFeatures.filter(
    (feature) => feature.effective_enabled,
  ).length;
  const firstBlockedFeature = useMemo(
    () => areaFeatures.find((feature) => feature.readiness === "blocked") ?? null,
    [areaFeatures],
  );
  const messageRouteChannelOptions = useMemo(
    () => buildMessageRouteChannelPickerOptions(channelOptions.channels),
    [channelOptions.channels],
  );

  useEffect(() => {
    if (selectedFeature === null) {
      return;
    }

    switch (selectedFeature.id) {
      case "services.commands":
        setChannelDraft(getCommandsFeatureDetails(selectedFeature).channelId);
        return;
      case "services.admin_commands":
        setAllowedRoleIdsDraft(
          getAdminCommandsFeatureDetails(selectedFeature).allowedRoleIds,
        );
        return;
      default:
        return;
    }
  }, [selectedFeature]);

  function closeDrawer() {
    setSelectedFeatureId("");
    setChannelDraft("");
    setAllowedRoleIdsDraft([]);
  }

  function openDrawer(feature: FeatureRecord) {
    if (
      feature.id === "services.commands" &&
      !canEditCommandsChannel(feature)
    ) {
      return;
    }
    if (
      feature.id === "services.admin_commands" &&
      !canEditAdminCommands(feature)
    ) {
      return;
    }

    setSelectedFeatureId(feature.id);
  }

  async function handleRefreshCommands() {
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

  async function handleSaveCommandChannel() {
    if (commandsFeature === null) {
      return;
    }

    setPendingFeatureId(commandsFeature.id);

    try {
      const updated = await mutation.patchFeature(commandsFeature.id, {
        channel_id: channelDraft.trim(),
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveAdminAccess() {
    if (adminCommandsFeature === null) {
      return;
    }

    setPendingFeatureId(adminCommandsFeature.id);

    try {
      const updated = await mutation.patchFeature(adminCommandsFeature.id, {
        allowed_role_ids: allowedRoleIdsDraft,
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
        disabled={
          workspace.loading ||
          mutation.saving ||
          channelOptions.loading ||
          roleOptions.loading
        }
        onClick={() => void handleRefreshCommands()}
      >
        Refresh commands
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
                onClick={() => void handleRefreshCommands()}
              >
                Retry loading
              </button>
            ) : undefined
          }
        />
      );
    }

    if (commandsFeature === null || adminCommandsFeature === null) {
      return (
        <div className="table-empty-state table-empty-state-compact">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>No command controls yet</h2>
            <p className="section-description">
              The selected server does not expose the command feature records
              required by this workspace yet.
            </p>
          </div>
        </div>
      );
    }

    const commandDetails = getCommandsFeatureDetails(commandsFeature);
    const adminDetails = getAdminCommandsFeatureDetails(adminCommandsFeature);

    return (
      <div className="commands-module-grid">
        <section className="surface-subsection commands-module-panel">
          <div className="commands-module-head">
            <div className="card-copy commands-module-copy">
              <p className="section-label">Commands</p>
              <div className="commands-module-title-row">
                <h3>{commandsFeature.label}</h3>
                <StatusBadge tone={getFeatureStatusTone(commandsFeature)}>
                  {formatFeatureStatusLabel(commandsFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">
                Keep slash-command handling available for the selected server
                and optionally route configuration through a dedicated command
                channel.
              </p>
            </div>

            <span className="meta-pill subtle-pill">
              {formatCommandServerSetting(commandsFeature)}
            </span>
          </div>

          <KeyValueList
            items={[
              {
                label: "Module state",
                value: commandsFeature.effective_enabled ? "On" : "Off",
              },
              {
                label: "Command channel",
                value: formatGuildChannelValue(
                  commandDetails.channelId,
                  channelOptions.channels,
                  "No dedicated channel",
                ),
              },
              {
                label: "Current signal",
                value: summarizeCommandsSignal(commandsFeature),
              },
            ]}
          />

          <p className="meta-note commands-module-note">
            {commandDetails.channelId === ""
              ? "Leave the command channel empty when slash-command handling should stay available without a dedicated routing destination."
              : "The configured command channel keeps command setup and related follow-up actions in one place."}
          </p>

          <div className="inline-actions commands-module-actions">
            <button
              className="button-primary"
              type="button"
              onClick={() => openDrawer(commandsFeature)}
            >
              Configure command channel
            </button>
            <button
              className="button-secondary"
              type="button"
              disabled={mutation.saving}
              onClick={() =>
                void handleSetFeatureEnabled(
                  commandsFeature,
                  !commandsFeature.effective_enabled,
                )
              }
            >
              {mutation.saving && pendingFeatureId === commandsFeature.id
                ? "Saving..."
                : commandsFeature.effective_enabled
                  ? "Disable commands"
                  : "Enable commands"}
            </button>
            {commandsFeature.override_state !== "inherit" ? (
              <button
                className="button-ghost"
                type="button"
                disabled={mutation.saving}
                onClick={() => void handleUseDefault(commandsFeature)}
              >
                Use default
              </button>
            ) : null}
          </div>
        </section>

        <section className="surface-subsection commands-module-panel">
          <div className="commands-module-head">
            <div className="card-copy commands-module-copy">
              <p className="section-label">Admin access</p>
              <div className="commands-module-title-row">
                <h3>{adminCommandsFeature.label}</h3>
                <StatusBadge tone={getFeatureStatusTone(adminCommandsFeature)}>
                  {formatFeatureStatusLabel(adminCommandsFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">
                Limit privileged command workflows to the roles configured for
                this server.
              </p>
            </div>

            <span className="meta-pill subtle-pill">
              {formatCommandServerSetting(adminCommandsFeature)}
            </span>
          </div>

          <KeyValueList
            items={[
              {
                label: "Module state",
                value: adminCommandsFeature.effective_enabled ? "On" : "Off",
              },
              {
                label: "Allowed roles",
                value: formatAllowedRolesValue(adminCommandsFeature, roleOptions.roles),
              },
              {
                label: "Current signal",
                value: summarizeAdminCommandsSignal(adminCommandsFeature),
              },
            ]}
          />

          <p className="meta-note commands-module-note">
            {adminDetails.allowedRoleCount === 0
              ? "Choose which server roles should be allowed to use admin-only command workflows."
              : "Review the selected roles whenever command privileges need to change for this server."}
          </p>

          <div className="inline-actions commands-module-actions">
            <button
              className="button-primary"
              type="button"
              onClick={() => openDrawer(adminCommandsFeature)}
            >
              Configure admin access
            </button>
            <button
              className="button-secondary"
              type="button"
              disabled={mutation.saving}
              onClick={() =>
                void handleSetFeatureEnabled(
                  adminCommandsFeature,
                  !adminCommandsFeature.effective_enabled,
                )
              }
            >
              {mutation.saving && pendingFeatureId === adminCommandsFeature.id
                ? "Saving..."
                : adminCommandsFeature.effective_enabled
                  ? "Disable admin commands"
                  : "Enable admin commands"}
            </button>
            {adminCommandsFeature.override_state !== "inherit" ? (
              <button
                className="button-ghost"
                type="button"
                disabled={mutation.saving}
                onClick={() => void handleUseDefault(adminCommandsFeature)}
              >
                Use default
              </button>
            ) : null}
          </div>
        </section>
      </div>
    );
  }

  return (
    <>
      <section className="page-shell">
        <PageHeader
          eyebrow="Feature area"
          title={areaLabel}
          description="Configure command routing and privileged command access for the selected server without falling back to the generic feature list."
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

        {workspace.workspaceState === "ready" &&
        commandsFeature !== null &&
        adminCommandsFeature !== null ? (
          <section
            className="overview-summary-strip"
            aria-label="Commands summary"
          >
            <MetricCard
              label="Commands"
              value={formatFeatureStatusLabel(commandsFeature)}
              description={summarizeCommandsSignal(commandsFeature)}
              tone={getFeatureStatusTone(commandsFeature)}
            />
            <MetricCard
              label="Command channel"
              value={formatGuildChannelValue(
                getCommandsFeatureDetails(commandsFeature).channelId,
                channelOptions.channels,
                "Not set",
              )}
              description="The optional routing destination used for command workflows."
            />
            <MetricCard
              label="Admin access"
              value={formatAllowedRoleCountValue(adminCommandsFeature)}
              description={formatAllowedRolesValue(adminCommandsFeature, roleOptions.roles)}
              tone={
                getAdminCommandsFeatureDetails(adminCommandsFeature).allowedRoleCount > 0
                  ? "info"
                  : "neutral"
              }
            />
            <MetricCard
              label="Overrides"
              value={String(localOverrides)}
              description={`${enabledModules}/${areaFeatures.length} command modules enabled for this server.`}
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
                    <h2>Command routing</h2>
                    <p className="section-description">
                      Keep command handling and privileged access in one place.
                      The default workspace answers the two most common admin
                      tasks here: where commands should route and which roles can
                      use the privileged ones.
                    </p>
                  </div>
                  <div className="workspace-view-meta">
                    {workspace.workspaceState === "ready" ? (
                      <>
                        <span className="meta-pill subtle-pill">
                          {localOverrides} local overrides
                        </span>
                        <span className="meta-pill subtle-pill">
                          {enabledModules}/{areaFeatures.length} enabled
                        </span>
                      </>
                    ) : null}
                  </div>
                </div>

                <AlertBanner
                  notice={workspaceNotice}
                  busyLabel={
                    mutation.saving
                      ? "Saving command settings..."
                      : workspace.loading ||
                          channelOptions.loading ||
                          roleOptions.loading
                        ? "Refreshing commands workspace..."
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
                <h2>Current command setup</h2>
                <p className="section-description">
                  Use this panel to confirm the selected server, the current
                  command destination, and whether privileged access is already
                  restricted by roles.
                </p>
              </div>

              <KeyValueList
                items={[
                  {
                    label: "Server",
                    value: selectedServerLabel,
                  },
                  {
                    label: "Command channel",
                    value:
                      commandsFeature === null
                        ? "Not available"
                        : formatGuildChannelValue(
                            getCommandsFeatureDetails(commandsFeature).channelId,
                            channelOptions.channels,
                            "No dedicated channel",
                          ),
                  },
                  {
                    label: "Admin access",
                    value:
                      adminCommandsFeature === null
                        ? "Not available"
                        : formatAllowedRoleCountValue(adminCommandsFeature),
                  },
                  {
                    label: "Current signal",
                    value:
                      firstBlockedFeature !== null
                        ? firstBlockedFeature.id === "services.commands"
                          ? summarizeCommandsSignal(firstBlockedFeature)
                          : summarizeAdminCommandsSignal(firstBlockedFeature)
                        : areaSummary.signal,
                  },
                ]}
              />
            </SurfaceCard>

            <SurfaceCard>
              <div className="card-copy">
                <p className="section-label">Guidance</p>
                <h2>How this page works</h2>
                <p className="section-description">
                  Keep the main workspace centered on the two command tasks an
                  admin actually needs to complete: where commands should route
                  and who can use the privileged ones.
                </p>
              </div>

              <ul className="feature-guidance-list">
                <li>Set a command channel only when you want a dedicated destination for command workflows.</li>
                <li>Review admin access roles whenever staff permissions change for the selected server.</li>
                <li>Runtime blockers stay in signals and notices instead of dominating the default commands workspace.</li>
              </ul>

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
        </section>
      </section>

      {selectedFeature !== null ? (
        <div className="drawer-backdrop" onClick={closeDrawer} role="presentation">
          <aside
            aria-label={getDrawerLabel(selectedFeature)}
            aria-modal="true"
            className="drawer-panel commands-drawer"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="card-copy">
              <p className="section-label">Commands</p>
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
              channelDraft,
              allowedRoleIdsDraft,
              availableChannels: channelOptions.channels,
              channelOptions: messageRouteChannelOptions,
              channelOptionsNotice: channelOptions.notice,
              channelOptionsLoading: channelOptions.loading,
              roleOptions: roleOptions.roles,
              roleOptionsLoading: roleOptions.loading,
              roleOptionsNotice: roleOptions.notice,
              refreshChannelOptions: channelOptions.refresh,
              setChannelDraft,
              setAllowedRoleIdsDraft,
              closeDrawer,
              refreshRoleOptions: roleOptions.refresh,
              handleSaveCommandChannel,
              handleSaveAdminAccess,
            })}
          </aside>
        </div>
      ) : null}
    </>
  );
}

function getDrawerLabel(feature: FeatureRecord) {
  switch (feature.id) {
    case "services.commands":
      return "Configure commands";
    case "services.admin_commands":
      return "Configure admin commands";
    default:
      return `Configure ${feature.label}`;
  }
}

interface RenderDrawerBodyProps {
  selectedFeature: FeatureRecord;
  pendingFeatureId: string;
  mutationSaving: boolean;
  channelDraft: string;
  allowedRoleIdsDraft: string[];
  availableChannels: GuildChannelOption[];
  channelOptions: Array<{ value: string; label: string; description?: string }>;
  channelOptionsLoading: boolean;
  channelOptionsNotice: { tone: "info" | "success" | "error"; message: string } | null;
  roleOptions: GuildRoleOption[];
  roleOptionsLoading: boolean;
  roleOptionsNotice: { tone: "info" | "success" | "error"; message: string } | null;
  setChannelDraft: (value: string) => void;
  setAllowedRoleIdsDraft: (
    value: string[] | ((current: string[]) => string[]),
  ) => void;
  closeDrawer: () => void;
  refreshChannelOptions: () => Promise<void>;
  refreshRoleOptions: () => Promise<void>;
  handleSaveCommandChannel: () => Promise<void>;
  handleSaveAdminAccess: () => Promise<void>;
}

function renderDrawerBody({
  selectedFeature,
  pendingFeatureId,
  mutationSaving,
  channelDraft,
  allowedRoleIdsDraft,
  availableChannels,
  channelOptions,
  channelOptionsLoading,
  channelOptionsNotice,
  roleOptions,
  roleOptionsLoading,
  roleOptionsNotice,
  setChannelDraft,
  setAllowedRoleIdsDraft,
  closeDrawer,
  refreshChannelOptions,
  refreshRoleOptions,
  handleSaveCommandChannel,
  handleSaveAdminAccess,
}: RenderDrawerBodyProps) {
  if (selectedFeature.id === "services.commands") {
    const details = getCommandsFeatureDetails(selectedFeature);

    return (
      <>
        <KeyValueList
          items={[
            {
              label: "Module state",
              value: selectedFeature.effective_enabled ? "On" : "Off",
            },
            {
              label: "Current channel",
              value: formatGuildChannelValue(
                details.channelId,
                availableChannels,
                "No dedicated channel",
              ),
            },
            {
              label: "Current signal",
              value: summarizeCommandsSignal(selectedFeature),
            },
          ]}
        />

          {channelOptionsNotice ? (
            <LookupNotice
              title="Channel references unavailable"
              message={channelOptionsNotice.message}
              retryLabel="Retry channel lookup"
              retryDisabled={channelOptionsLoading}
              onRetry={refreshChannelOptions}
            />
          ) : null}

        <EntityPickerField
          label="Command channel"
          value={channelDraft}
          disabled={channelOptionsLoading}
          onChange={setChannelDraft}
          options={channelOptions}
          placeholder={
            channelOptionsLoading
              ? "Loading channels..."
              : channelOptions.length === 0
                ? "No channels available"
                : "No dedicated channel"
          }
          note="Leave this empty when you do not want a dedicated routing channel for command workflows."
        />

          <AdvancedTextInput
            label="Channel ID fallback"
            inputLabel="Command channel ID fallback"
            value={channelDraft}
            onChange={setChannelDraft}
            placeholder="Discord channel ID"
            note="Use this only when the channel picker is unavailable or when you need to paste a channel ID directly."
          />

        <div className="surface-subsection">
          <p className="section-label">Routing guidance</p>
          <ul className="feature-guidance-list">
            <li>Use one command channel when setup and follow-up actions should stay in a single place.</li>
            <li>Leave the channel empty when command handling should stay available without a dedicated destination.</li>
          </ul>
        </div>

        <div className="drawer-actions">
          <button
            className="button-primary"
            type="button"
            disabled={mutationSaving}
            onClick={() => void handleSaveCommandChannel()}
          >
            {mutationSaving && pendingFeatureId === selectedFeature.id
              ? "Saving..."
              : "Save command channel"}
          </button>
          <button className="button-secondary" type="button" onClick={closeDrawer}>
            Cancel
          </button>
        </div>
      </>
    );
  }

  const adminDetails = getAdminCommandsFeatureDetails(selectedFeature);

  return (
    <>
      <KeyValueList
        items={[
          {
            label: "Module state",
            value: selectedFeature.effective_enabled ? "On" : "Off",
          },
          {
            label: "Allowed roles",
            value: formatAllowedRoleCountValue(selectedFeature),
          },
          {
            label: "Current signal",
            value: summarizeAdminCommandsSignal(selectedFeature),
          },
        ]}
      />

        {roleOptionsNotice ? (
          <LookupNotice
            title="Role references unavailable"
            message={roleOptionsNotice.message}
            retryLabel="Retry role lookup"
            retryDisabled={roleOptionsLoading}
            onRetry={refreshRoleOptions}
          />
        ) : roleOptionsLoading && roleOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">Loading roles</p>
          <p className="meta-note">
            Loading the current server roles before privileged access can be
            updated.
          </p>
        </div>
      ) : roleOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">No roles available</p>
          <p className="meta-note">
            This server did not return any selectable roles for privileged
            command access.
          </p>
        </div>
      ) : (
        <EntityMultiPickerField
          label="Allowed roles"
          disabled={roleOptionsLoading}
          selectedValues={allowedRoleIdsDraft}
          onToggle={(roleId) =>
            setAllowedRoleIdsDraft((current) => toggleAllowedRole(current, roleId))
          }
          options={roleOptions.map((role) => ({
            value: role.id,
            label: formatAllowedRoleOptionLabel(role),
            description: role.is_default
              ? "Default role for every member."
              : role.managed
                ? "Managed by an integration."
                : "Available for privileged command access.",
          }))}
        />
      )}

      <div className="surface-subsection">
        <p className="section-label">Access guidance</p>
        <ul className="feature-guidance-list">
          <li>Choose only the roles that should be able to run privileged command workflows.</li>
          <li>Review these selections whenever staff permissions or trusted groups change.</li>
          {adminDetails.allowedRoleCount > 0 ? (
            <li>The current configuration already grants access to {formatAllowedRoleCountValue(selectedFeature).toLowerCase()}.</li>
          ) : null}
        </ul>
      </div>

      <div className="drawer-actions">
        <button
          className="button-primary"
          type="button"
          disabled={mutationSaving || roleOptionsLoading || roleOptionsNotice !== null}
          onClick={() => void handleSaveAdminAccess()}
        >
          {mutationSaving && pendingFeatureId === selectedFeature.id
            ? "Saving..."
            : "Save admin access"}
        </button>
        <button className="button-secondary" type="button" onClick={closeDrawer}>
          Cancel
        </button>
      </div>
    </>
  );
}
