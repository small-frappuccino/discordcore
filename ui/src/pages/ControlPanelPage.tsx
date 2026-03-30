import { useEffect, useState } from "react";
import {
  EntityMultiPickerField,
  EmptyState,
  FeatureWorkspaceLayout,
  LookupNotice,
  PageHeader,
  StatusBadge,
  SurfaceCard,
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
    currentOriginLabel,
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
    hasUnsavedChanges,
    replaceDrafts,
    toggleDashboardReadRole,
    toggleDashboardWriteRole,
  } = useDashboardAccessRoleDrafts({
    authState,
    selectedGuildID,
    dashboardReadRoleIds: rolesSettings.roles.dashboardReadRoleIds,
    dashboardWriteRoleIds: rolesSettings.roles.dashboardWriteRoleIds,
  });

  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const selectedGuildModeLabel =
    selectedGuildAccessLevel === "read" ? "Read-only access" : "Writable access";
  const controlsDisabled =
    authState !== "signed_in" ||
    selectedGuild === null ||
    !canEditSelectedGuild ||
    roleOptions.loading;
  const rolePickerOptions = roleOptions.roles.map((role) => ({
    value: role.id,
    label: formatRoleOptionLabel(role),
    description: role.is_default
      ? "Default role for every member."
      : role.managed
        ? "Managed by an integration."
        : "Available for dashboard access.",
  }));

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
      const response = await client.updateGuildSettings(selectedGuildID.trim(), {
        roles,
      });
      const nextReadRoles = normalizeRoleIds(
        response.workspace.sections.roles.dashboard_read,
      );
      const nextWriteRoles = normalizeRoleIds(
        response.workspace.sections.roles.dashboard_write,
      );
      replaceDrafts(nextReadRoles, nextWriteRoles);
      rolesSettings.updateCachedRoles({
        ...rolesSettings.roles,
        dashboardReadRoleIds: nextReadRoles,
        dashboardWriteRoleIds: nextWriteRoles,
      });
      setNotice({
        tone: "success",
        message: "Dashboard access roles updated.",
      });
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setSaving(false);
    }
  }

  function renderBody() {
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
      <div className="control-panel-grid">
        <SurfaceCard className="control-panel-card">
          <div className="card-copy">
            <p className="section-label">Read access</p>
            <h2>Dashboard read access roles</h2>
            <p className="section-description">
              Members with these roles can open the dashboard in read-only mode.
            </p>
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

          <EntityMultiPickerField
            label="Read access roles"
            options={rolePickerOptions}
            selectedValues={dashboardReadRoleIds}
            disabled={controlsDisabled}
            onToggle={toggleDashboardReadRole}
            note="Read access allows navigation and page inspection without enabling writes."
          />
        </SurfaceCard>

        <SurfaceCard className="control-panel-card">
          <div className="card-copy">
            <p className="section-label">Write access</p>
            <h2>Dashboard write access roles</h2>
            <p className="section-description">
              Members with these roles can change settings across the dashboard.
            </p>
          </div>

          <EntityMultiPickerField
            label="Write access roles"
            options={rolePickerOptions}
            selectedValues={dashboardWriteRoleIds}
            disabled={controlsDisabled}
            onToggle={toggleDashboardWriteRole}
            note="Write access also implies read access."
          />
        </SurfaceCard>

        <SurfaceCard className="control-panel-card control-panel-card-wide">
          <div className="card-copy">
            <p className="section-label">Access rules</p>
            <h2>Implicit administrator access</h2>
            <p className="section-description">
              Discord server owners, administrators, and members with Manage Server
              remain implicitly allowed with write access.
            </p>
          </div>

          {selectedGuildAccessLevel === "read" ? (
            <p className="meta-note">
              You currently have read-only access to this server. Role changes are disabled.
            </p>
          ) : (
            <p className="meta-note">
              Save changes after reviewing both role lists. Existing admin command roles remain separate.
            </p>
          )}

          <div className="inline-actions">
            <button
              className="button-primary"
              type="button"
              disabled={controlsDisabled || saving || !hasUnsavedChanges}
              onClick={() => void handleSave()}
            >
              {saving ? "Saving..." : "Save access roles"}
            </button>
          </div>
        </SurfaceCard>
      </div>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Core"
        title="Control Panel"
        description="Configure which roles can read or write the dashboard for the selected server."
        status={
          <StatusBadge tone={selectedGuildAccessLevel === "read" ? "info" : "neutral"}>
            {selectedGuildAccessLevel === "read" ? "Read-only access" : "Access roles"}
          </StatusBadge>
        }
        meta={
          <>
            <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
            <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
          </>
        }
      />

      <FeatureWorkspaceLayout
        notice={notice ?? rolesSettings.notice ?? roleOptions.notice}
        busyLabel={
          saving
            ? "Saving access roles..."
            : rolesSettings.loading
              ? "Loading access roles..."
              : roleOptions.loading
                ? "Loading role references..."
                : undefined
        }
        workspaceTitle="Manage dashboard access"
        workspaceDescription="Keep read and write dashboard access aligned with the selected server's operator roles without exposing raw settings fields in the main workspace."
        workspaceMeta={
          selectedGuild !== null ? (
            <>
              <span className="meta-pill subtle-pill">{selectedGuildModeLabel}</span>
              <span className="meta-pill subtle-pill">
                {hasUnsavedChanges ? "Unsaved changes" : "Saved"}
              </span>
            </>
          ) : null
        }
        workspaceContent={renderBody()}
      />
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

function areStringListsEqual(currentValues: string[], nextValues: string[]) {
  return (
    currentValues.length === nextValues.length &&
    currentValues.every((value, index) => value === nextValues[index])
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

function normalizeRoleIds(roleIds: string[] | undefined) {
  if (!Array.isArray(roleIds)) {
    return [];
  }
  return roleIds
    .filter((roleId): roleId is string => typeof roleId === "string")
    .map((roleId) => roleId.trim())
    .filter((roleId) => roleId !== "");
}
