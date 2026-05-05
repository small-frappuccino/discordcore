import { useEffect, useState } from "react";
import {
  DashboardPageSurface,
  EntityMultiPickerField,
  EmptyState,
  PageHeader,
  SettingsSelectField,
  UnsavedChangesBar,
} from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";
import { formatRoleOptionLabel } from "../features/features/roles";
import { useGuildRolesSettings } from "../features/control-panel/useGuildRolesSettings";
import { formatError } from "../app/utils";
import type { Notice } from "../app/types";

export function ControlPanelPage() {
  const {
    authState,
    beginLogin,
    canEditSelectedGuild,
    selectedGuild,
    selectedGuildID,
    selectedGuildAccessLevel,
    client,
  } = useDashboardSession();
  const roleOptions = useGuildRoleOptions();
  const rolesSettings = useGuildRolesSettings();
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const {
    dashboardReadRoleIds,
    dashboardWriteRoleIds,
    hasUnsavedChanges: hasRoleDraftChanges,
    replaceDrafts: replaceRoleDrafts,
    toggleDashboardReadRole,
    toggleDashboardWriteRole,
  } = useDashboardAccessRoleDrafts({
    authState,
    selectedGuildID,
    dashboardReadRoleIds: rolesSettings.roles.dashboardReadRoleIds,
    dashboardWriteRoleIds: rolesSettings.roles.dashboardWriteRoleIds,
  });
  const {
    botInstanceID,
    domainBotInstanceIDs,
    hasUnsavedChanges: hasBotRoutingDraftChanges,
    replaceDrafts: replaceBotRoutingDrafts,
    setBotInstanceID,
    setDomainBotInstanceID,
  } = useGuildBotRoutingDrafts({
    authState,
    selectedGuildID,
    botInstanceID: rolesSettings.botRouting.botInstanceID,
    domainBotInstanceIDs: rolesSettings.botRouting.domainBotInstanceIDs,
  });
  const hasUnsavedChanges = hasRoleDraftChanges || hasBotRoutingDraftChanges;

  const controlsDisabled =
    authState !== "signed_in" ||
    selectedGuild === null ||
    !canEditSelectedGuild ||
    roleOptions.loading ||
    rolesSettings.loading ||
    saving;
  const rolePickerOptions = roleOptions.roles.map((role) => ({
    value: role.id,
    label: formatRoleOptionLabel(role),
  }));
  const botInstanceOptions = rolesSettings.botRouting.availableBotInstanceIDs.map(
    (instanceID) => ({
      value: instanceID,
      label: formatBotInstanceLabel(instanceID),
    }),
  );
  const domainBotInstanceOptions = rolesSettings.botRouting.domainOverrideBotInstanceIDs.map(
    (instanceID) => ({
      value: instanceID,
      label: formatBotInstanceLabel(instanceID),
    }),
  );

  async function handleSave() {
    if (authState !== "signed_in" || selectedGuildID.trim() === "" || !canEditSelectedGuild) {
      return;
    }

    setSaving(true);

    try {
      const roles = {
        dashboard_read: normalizeRoleIds(dashboardReadRoleIds),
        dashboard_write: normalizeRoleIds(dashboardWriteRoleIds),
      };
      const nextBotRoutingPayload = {
        bot_instance_id: normalizeBotInstanceID(botInstanceID),
        domain_bot_instance_ids: buildRequestedDomainBotInstanceIDs(
          domainBotInstanceIDs,
          rolesSettings.botRouting.editableDomains,
          botInstanceID,
        ),
      };
      const response = await client.updateGuildSettings(selectedGuildID.trim(), {
        bot_routing: nextBotRoutingPayload,
        roles,
      });
      const nextReadRoles = normalizeRoleIds(
        response.workspace.sections.roles.dashboard_read,
      );
      const nextWriteRoles = normalizeRoleIds(
        response.workspace.sections.roles.dashboard_write,
      );
      const nextBotRouting = readBotRoutingSnapshot(response.workspace);
      replaceRoleDrafts(nextReadRoles, nextWriteRoles);
      replaceBotRoutingDrafts(
        nextBotRouting.botInstanceID,
        nextBotRouting.domainBotInstanceIDs,
      );
      rolesSettings.updateCachedSettings({
        roles: {
          ...rolesSettings.roles,
          dashboardReadRoleIds: nextReadRoles,
          dashboardWriteRoleIds: nextWriteRoles,
        },
        botRouting: nextBotRouting,
      });
      setNotice(null);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setSaving(false);
    }
  }

  function handleReset() {
    replaceRoleDrafts(
      normalizeRoleIds(rolesSettings.roles.dashboardReadRoleIds),
      normalizeRoleIds(rolesSettings.roles.dashboardWriteRoleIds),
    );
    replaceBotRoutingDrafts(
      normalizeBotInstanceID(rolesSettings.botRouting.botInstanceID),
      normalizeDomainBotInstanceIDs(rolesSettings.botRouting.domainBotInstanceIDs),
    );
    setNotice(null);
    rolesSettings.clearNotice();
  }

  function renderBody() {
    if (authState === "checking") {
      return (
        <div className="workspace-state" aria-busy="true">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>Checking access</h2>
            <p className="section-description">
              The dashboard is checking your Discord session and current server context.
            </p>
          </div>
        </div>
      );
    }

    if (authState !== "signed_in") {
      return (
        <EmptyState
          title="Sign in required"
          description="Sign in with Discord before reviewing dashboard access roles."
          action={
            <button
              className="button-primary"
              type="button"
              onClick={() => void beginLogin()}
            >
              Sign in with Discord
            </button>
          }
        />
      );
    }

    if (selectedGuild === null) {
      return (
        <EmptyState
          title="Select a server"
          description="Choose a server from the top bar before editing dashboard access roles."
        />
      );
    }

    return (
      <div className="workspace-view control-panel-workspace">
        <section className="control-panel-flat-section">
          <div className="workspace-view-header">
            <div className="card-copy">
              <p className="section-label">Bot routing</p>
              <h2>Domain ownership</h2>
              <p className="section-description">
                Choose which bot instance owns the default server surface and specialized domains like QOTD.
              </p>
            </div>
            <div className="workspace-view-meta">
              <span className="meta-note">
                {botInstanceOptions.length} bot instances
              </span>
              <span className="meta-note">
                {countConfiguredDomainOverrides(domainBotInstanceIDs)} domain overrides
              </span>
            </div>
          </div>

          <div className="control-panel-grid">
            <SettingsSelectField
              label="Default bot instance"
              value={botInstanceID}
              options={botInstanceOptions}
              placeholder="Select bot instance"
              disabled={controlsDisabled || botInstanceOptions.length === 0}
              onChange={setBotInstanceID}
              note="Domains without an explicit override inherit this bot instance."
            />

            {rolesSettings.botRouting.editableDomains.map((domain) => (
              <SettingsSelectField
                key={domain}
                label={`${formatBotRoutingDomainLabel(domain)} domain`}
                value={domainBotInstanceIDs[domain] ?? ""}
                options={domainBotInstanceOptions}
                placeholder={`Inherit ${formatBotInstanceLabel(botInstanceID)}`}
                disabled={controlsDisabled || domainBotInstanceOptions.length === 0}
                onChange={(value) => setDomainBotInstanceID(domain, value)}
                note={`Leave this inherited to keep ${formatBotRoutingDomainLabel(domain)} on the default bot instance.`}
              />
            ))}
          </div>

          {botInstanceOptions.length === 0 ? (
            <p className="meta-note">
              No bot instances are currently available for this server.
            </p>
          ) : null}
        </section>

        <section className="control-panel-flat-section">
          <div className="workspace-view-header">
            <div className="card-copy">
              <p className="section-label">Access roles</p>
              <h2>Dashboard access</h2>
              <p className="section-description">
                Choose which roles can open the dashboard in read-only or writable mode.
              </p>
            </div>
            <div className="workspace-view-meta">
              <span className="meta-note">
                {dashboardReadRoleIds.length} read roles
              </span>
              <span className="meta-note">
                {dashboardWriteRoleIds.length} write roles
              </span>
            </div>
          </div>

          {roleOptions.notice ? (
            <div className="flat-inline-message">
              <p className="meta-note">{roleOptions.notice.message}</p>
              <div className="inline-actions">
                <button
                  className="button-secondary"
                  type="button"
                  disabled={roleOptions.loading}
                  onClick={() => void roleOptions.refresh()}
                >
                  Retry role lookup
                </button>
              </div>
            </div>
          ) : null}

          <div className="control-panel-grid">
            <EntityMultiPickerField
              label="Read access roles"
              options={rolePickerOptions}
              selectedValues={dashboardReadRoleIds}
              disabled={controlsDisabled}
              onToggle={toggleDashboardReadRole}
              note="Read access allows navigation and inspection without writes."
            />

            <EntityMultiPickerField
              label="Write access roles"
              options={rolePickerOptions}
              selectedValues={dashboardWriteRoleIds}
              disabled={controlsDisabled}
              onToggle={toggleDashboardWriteRole}
              note="Write access also implies read access."
            />
          </div>
        </section>

        <section className="control-panel-flat-section">
          <div className="workspace-view-header">
            <div className="card-copy">
              <p className="section-label">Access rules</p>
              <h2>Implicit administrator access</h2>
              <p className="section-description">
                Discord server owners, administrators, and members with Manage Server remain implicitly allowed with write access.
              </p>
            </div>
          </div>

          <p className="meta-note">
            {selectedGuildAccessLevel === "read"
              ? "You currently have read-only access to this server. Role changes are disabled."
              : "Save changes after reviewing both role lists. Existing admin command roles remain separate."}
          </p>
        </section>
      </div>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Core"
        title="Control Panel"
        description="Configure which roles can read or write the dashboard for the selected server."
      />

      <DashboardPageSurface
        notice={notice ?? rolesSettings.notice}
        actionBar={
          <UnsavedChangesBar
            hasUnsavedChanges={hasUnsavedChanges}
            saveLabel={saving ? "Saving..." : "Save changes"}
            saving={saving}
            disabled={controlsDisabled}
            onReset={handleReset}
            onSave={handleSave}
          />
        }
      >
        {renderBody()}
      </DashboardPageSurface>
    </section>
  );
}

interface DashboardAccessRoleDraftsArgs {
  authState: string;
  selectedGuildID: string;
  dashboardReadRoleIds: string[];
  dashboardWriteRoleIds: string[];
}

interface DashboardAccessRoleDraftsState {
  guildID: string;
  dashboardReadRoleIds: string[];
  dashboardWriteRoleIds: string[];
  hasUnsavedChanges: boolean;
}

interface GuildBotRoutingDraftsArgs {
  authState: string;
  selectedGuildID: string;
  botInstanceID: string;
  domainBotInstanceIDs: Record<string, string>;
}

interface GuildBotRoutingDraftsState {
  guildID: string;
  botInstanceID: string;
  domainBotInstanceIDs: Record<string, string>;
  hasUnsavedChanges: boolean;
}

function useDashboardAccessRoleDrafts({
  authState,
  selectedGuildID,
  dashboardReadRoleIds,
  dashboardWriteRoleIds,
}: DashboardAccessRoleDraftsArgs) {
  const normalizedGuildID = selectedGuildID.trim();
  const [drafts, setDrafts] = useState<DashboardAccessRoleDraftsState>(() =>
    createDashboardAccessRoleDraftsState(
      authState,
      normalizedGuildID,
      dashboardReadRoleIds,
      dashboardWriteRoleIds,
    ),
  );

  useEffect(() => {
    setDrafts((currentDrafts) => {
      const nextDrafts = createDashboardAccessRoleDraftsState(
        authState,
        normalizedGuildID,
        dashboardReadRoleIds,
        dashboardWriteRoleIds,
      );

      if (
        authState !== "signed_in" ||
        normalizedGuildID === "" ||
        currentDrafts.guildID !== normalizedGuildID ||
        !currentDrafts.hasUnsavedChanges
      ) {
        return areDashboardAccessRoleDraftsEqual(currentDrafts, nextDrafts)
          ? currentDrafts
          : nextDrafts;
      }

      return currentDrafts;
    });
  }, [
    authState,
    dashboardReadRoleIds,
    dashboardWriteRoleIds,
    normalizedGuildID,
  ]);

  function replaceDrafts(nextReadRoleIds: string[], nextWriteRoleIds: string[]) {
    setDrafts({
      guildID: normalizedGuildID,
      dashboardReadRoleIds: nextReadRoleIds,
      dashboardWriteRoleIds: nextWriteRoleIds,
      hasUnsavedChanges: false,
    });
  }

  function toggleDashboardReadRole(roleId: string) {
    setDrafts((currentDrafts) => ({
      ...currentDrafts,
      dashboardReadRoleIds: toggleRole(currentDrafts.dashboardReadRoleIds, roleId),
      hasUnsavedChanges: true,
    }));
  }

  function toggleDashboardWriteRole(roleId: string) {
    setDrafts((currentDrafts) => ({
      ...currentDrafts,
      dashboardWriteRoleIds: toggleRole(currentDrafts.dashboardWriteRoleIds, roleId),
      hasUnsavedChanges: true,
    }));
  }

  return {
    dashboardReadRoleIds: drafts.dashboardReadRoleIds,
    dashboardWriteRoleIds: drafts.dashboardWriteRoleIds,
    hasUnsavedChanges: drafts.hasUnsavedChanges,
    replaceDrafts,
    toggleDashboardReadRole,
    toggleDashboardWriteRole,
  };
}

function useGuildBotRoutingDrafts({
  authState,
  selectedGuildID,
  botInstanceID,
  domainBotInstanceIDs,
}: GuildBotRoutingDraftsArgs) {
  const normalizedGuildID = selectedGuildID.trim();
  const [drafts, setDrafts] = useState<GuildBotRoutingDraftsState>(() =>
    createGuildBotRoutingDraftsState(
      authState,
      normalizedGuildID,
      botInstanceID,
      domainBotInstanceIDs,
    ),
  );

  useEffect(() => {
    setDrafts((currentDrafts) => {
      const nextDrafts = createGuildBotRoutingDraftsState(
        authState,
        normalizedGuildID,
        botInstanceID,
        domainBotInstanceIDs,
      );

      if (
        authState !== "signed_in" ||
        normalizedGuildID === "" ||
        currentDrafts.guildID !== normalizedGuildID ||
        !currentDrafts.hasUnsavedChanges
      ) {
        return areGuildBotRoutingDraftsEqual(currentDrafts, nextDrafts)
          ? currentDrafts
          : nextDrafts;
      }

      return currentDrafts;
    });
  }, [authState, botInstanceID, domainBotInstanceIDs, normalizedGuildID]);

  function replaceDrafts(
    nextBotInstanceID: string,
    nextDomainBotInstanceIDs: Record<string, string>,
  ) {
    setDrafts({
      guildID: normalizedGuildID,
      botInstanceID: normalizeBotInstanceID(nextBotInstanceID),
      domainBotInstanceIDs: normalizeDomainBotInstanceIDs(nextDomainBotInstanceIDs),
      hasUnsavedChanges: false,
    });
  }

  function setBotInstanceID(nextBotInstanceID: string) {
    setDrafts((currentDrafts) => ({
      ...currentDrafts,
      botInstanceID: normalizeBotInstanceID(nextBotInstanceID),
      hasUnsavedChanges: true,
    }));
  }

  function setDomainBotInstanceID(domain: string, nextBotInstanceID: string) {
    setDrafts((currentDrafts) => ({
      ...currentDrafts,
      domainBotInstanceIDs: updateDomainBotInstanceID(
        currentDrafts.domainBotInstanceIDs,
        domain,
        nextBotInstanceID,
      ),
      hasUnsavedChanges: true,
    }));
  }

  return {
    botInstanceID: drafts.botInstanceID,
    domainBotInstanceIDs: drafts.domainBotInstanceIDs,
    hasUnsavedChanges: drafts.hasUnsavedChanges,
    replaceDrafts,
    setBotInstanceID,
    setDomainBotInstanceID,
  };
}

function createDashboardAccessRoleDraftsState(
  authState: string,
  normalizedGuildID: string,
  dashboardReadRoleIds: string[],
  dashboardWriteRoleIds: string[],
): DashboardAccessRoleDraftsState {
  if (authState !== "signed_in" || normalizedGuildID === "") {
    return {
      guildID: "",
      dashboardReadRoleIds: [],
      dashboardWriteRoleIds: [],
      hasUnsavedChanges: false,
    };
  }

  return {
    guildID: normalizedGuildID,
    dashboardReadRoleIds,
    dashboardWriteRoleIds,
    hasUnsavedChanges: false,
  };
}

function createGuildBotRoutingDraftsState(
  authState: string,
  normalizedGuildID: string,
  botInstanceID: string,
  domainBotInstanceIDs: Record<string, string>,
): GuildBotRoutingDraftsState {
  if (authState !== "signed_in" || normalizedGuildID === "") {
    return {
      guildID: "",
      botInstanceID: "",
      domainBotInstanceIDs: {},
      hasUnsavedChanges: false,
    };
  }

  return {
    guildID: normalizedGuildID,
    botInstanceID: normalizeBotInstanceID(botInstanceID),
    domainBotInstanceIDs: normalizeDomainBotInstanceIDs(domainBotInstanceIDs),
    hasUnsavedChanges: false,
  };
}

function areDashboardAccessRoleDraftsEqual(
  currentDrafts: DashboardAccessRoleDraftsState,
  nextDrafts: DashboardAccessRoleDraftsState,
) {
  return (
    currentDrafts.guildID === nextDrafts.guildID &&
    currentDrafts.hasUnsavedChanges === nextDrafts.hasUnsavedChanges &&
    areStringListsEqual(
      currentDrafts.dashboardReadRoleIds,
      nextDrafts.dashboardReadRoleIds,
    ) &&
    areStringListsEqual(
      currentDrafts.dashboardWriteRoleIds,
      nextDrafts.dashboardWriteRoleIds,
    )
  );
}

function areGuildBotRoutingDraftsEqual(
  currentDrafts: GuildBotRoutingDraftsState,
  nextDrafts: GuildBotRoutingDraftsState,
) {
  return (
    currentDrafts.guildID === nextDrafts.guildID &&
    currentDrafts.botInstanceID === nextDrafts.botInstanceID &&
    currentDrafts.hasUnsavedChanges === nextDrafts.hasUnsavedChanges &&
    areStringMapsEqual(
      currentDrafts.domainBotInstanceIDs,
      nextDrafts.domainBotInstanceIDs,
    )
  );
}

function areStringListsEqual(currentValues: string[], nextValues: string[]) {
  return (
    currentValues.length === nextValues.length &&
    currentValues.every((value, index) => value === nextValues[index])
  );
}

function areStringMapsEqual(
  currentValues: Record<string, string>,
  nextValues: Record<string, string>,
) {
  const currentEntries = Object.entries(currentValues).sort(([left], [right]) =>
    left.localeCompare(right),
  );
  const nextEntries = Object.entries(nextValues).sort(([left], [right]) =>
    left.localeCompare(right),
  );

  return (
    currentEntries.length === nextEntries.length &&
    currentEntries.every(([key, value], index) => {
      const [nextKey, nextValue] = nextEntries[index] ?? [];
      return key === nextKey && value === nextValue;
    })
  );
}

function toggleRole(currentRoleIds: string[], roleId: string) {
  const next = new Set(
    currentRoleIds
      .map((value) => value.trim())
      .filter((value) => value !== ""),
  );

  if (next.has(roleId)) {
    next.delete(roleId);
  } else {
    next.add(roleId);
  }

  return Array.from(next);
}

function updateDomainBotInstanceID(
  currentDomainBotInstanceIDs: Record<string, string>,
  domain: string,
  botInstanceID: string,
) {
  const normalized = normalizeDomainBotInstanceIDs(currentDomainBotInstanceIDs);
  const normalizedDomain = domain.trim().toLowerCase();
  const normalizedBotInstanceID = normalizeBotInstanceID(botInstanceID);
  if (normalizedDomain === "") {
    return normalized;
  }
  if (normalizedBotInstanceID === "") {
    delete normalized[normalizedDomain];
    return normalized;
  }
  normalized[normalizedDomain] = normalizedBotInstanceID;
  return normalized;
}

function normalizeBotInstanceID(botInstanceID: string | undefined) {
  if (typeof botInstanceID !== "string") {
    return "";
  }
  return botInstanceID.trim();
}

function normalizeDomainBotInstanceIDs(domainBotInstanceIDs: Record<string, string>) {
  const normalized: Record<string, string> = {};
  for (const [domain, botInstanceID] of Object.entries(domainBotInstanceIDs)) {
    const normalizedDomain = domain.trim().toLowerCase();
    const normalizedBotInstanceID = normalizeBotInstanceID(botInstanceID);
    if (normalizedDomain === "" || normalizedBotInstanceID === "") {
      continue;
    }
    normalized[normalizedDomain] = normalizedBotInstanceID;
  }
  return normalized;
}

function buildRequestedDomainBotInstanceIDs(
  domainBotInstanceIDs: Record<string, string>,
  editableDomains: string[],
  botInstanceID: string,
) {
  const normalizedBotInstanceID = normalizeBotInstanceID(botInstanceID);
  const requested: Record<string, string> = {};

  for (const domain of editableDomains) {
    const normalizedDomain = domain.trim().toLowerCase();
    const normalizedDomainBotInstanceID = normalizeBotInstanceID(
      domainBotInstanceIDs[normalizedDomain],
    );
    if (
      normalizedDomain === "" ||
      normalizedDomainBotInstanceID === "" ||
      normalizedDomainBotInstanceID === normalizedBotInstanceID
    ) {
      continue;
    }
    requested[normalizedDomain] = normalizedDomainBotInstanceID;
  }

  return requested;
}

function countConfiguredDomainOverrides(domainBotInstanceIDs: Record<string, string>) {
  return Object.keys(normalizeDomainBotInstanceIDs(domainBotInstanceIDs)).length;
}

function formatBotInstanceLabel(botInstanceID: string) {
  const normalizedBotInstanceID = normalizeBotInstanceID(botInstanceID);
  if (normalizedBotInstanceID === "") {
    return "the default bot";
  }
  return normalizedBotInstanceID
    .split(/[-_\s]+/)
    .filter((segment) => segment !== "")
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function formatBotRoutingDomainLabel(domain: string) {
  const normalizedDomain = domain.trim().toLowerCase();
  if (normalizedDomain === "qotd") {
    return "QOTD";
  }
  return formatBotInstanceLabel(normalizedDomain);
}

function readBotRoutingSnapshot(workspace: {
  bot_instance_id?: string;
  available_bot_instance_ids?: string[];
  sections: {
    bot_routing?: {
      bot_instance_id?: string;
      available_bot_instance_ids?: string[];
      domain_override_bot_instance_ids?: string[];
      domain_bot_instance_ids?: Record<string, string>;
      editable_domains?: string[];
    };
  };
}) {
  const section = workspace.sections.bot_routing;
  const availableBotInstanceIDs = normalizeRoleIds(
    section?.available_bot_instance_ids ?? workspace.available_bot_instance_ids,
  );
  const domainOverrideBotInstanceIDs = normalizeRoleIds(
    section?.domain_override_bot_instance_ids,
  );
  return {
    botInstanceID: normalizeBotInstanceID(
      section?.bot_instance_id ?? workspace.bot_instance_id,
    ),
    availableBotInstanceIDs,
    domainOverrideBotInstanceIDs:
      domainOverrideBotInstanceIDs.length > 0
        ? domainOverrideBotInstanceIDs
        : availableBotInstanceIDs,
    domainBotInstanceIDs: normalizeDomainBotInstanceIDs(
      section?.domain_bot_instance_ids ?? {},
    ),
    editableDomains: normalizeRoleIds(section?.editable_domains),
  };
}

function normalizeRoleIds(roleIds: string[] | undefined) {
  if (!Array.isArray(roleIds)) {
    return [];
  }
  return roleIds
    .filter((roleId): roleId is string => typeof roleId === "string")
    .map((roleId) => roleId.trim())
    .filter((roleId) => roleId !== "");
}
