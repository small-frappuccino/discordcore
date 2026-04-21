import { useEffect, useState } from "react";
import {
  DashboardPageSurface,
  EntityMultiPickerField,
  EntityPickerField,
  EmptyState,
  PageHeader,
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
    verificationRoleId,
    hasUnsavedChanges,
    replaceDrafts,
    toggleDashboardReadRole,
    toggleDashboardWriteRole,
    setVerificationRoleId,
  } = useDashboardAccessRoleDrafts({
    authState,
    selectedGuildID,
    dashboardReadRoleIds: rolesSettings.roles.dashboardReadRoleIds,
    dashboardWriteRoleIds: rolesSettings.roles.dashboardWriteRoleIds,
    verificationRoleId: rolesSettings.roles.verificationRoleId,
  });

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

  async function handleSave() {
    if (authState !== "signed_in" || selectedGuildID.trim() === "" || !canEditSelectedGuild) {
      return;
    }

    setSaving(true);

    try {
      const roles = {
        dashboard_read: normalizeRoleIds(dashboardReadRoleIds),
        dashboard_write: normalizeRoleIds(dashboardWriteRoleIds),
        verification_role: verificationRoleId,
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
      const nextVerificationRoleId = response.workspace.sections.roles.verification_role ?? "";

      replaceDrafts(nextReadRoles, nextWriteRoles, nextVerificationRoleId);
      rolesSettings.updateCachedRoles({
        ...rolesSettings.roles,
        dashboardReadRoleIds: nextReadRoles,
        dashboardWriteRoleIds: nextWriteRoles,
        verificationRoleId: nextVerificationRoleId,
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
    replaceDrafts(
      normalizeRoleIds(rolesSettings.roles.dashboardReadRoleIds),
      normalizeRoleIds(rolesSettings.roles.dashboardWriteRoleIds),
      rolesSettings.roles.verificationRoleId,
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
              <p className="section-label">Server roles</p>
              <h2>Verification role</h2>
              <p className="section-description">
                Select the role assigned to verified members.
              </p>
            </div>
          </div>

          <div className="control-panel-grid">
            <EntityPickerField
              label="Verified role"
              value={verificationRoleId}
              onChange={setVerificationRoleId}
              options={rolePickerOptions}
              disabled={controlsDisabled}
              placeholder="No primary role configured"
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
  verificationRoleId: string;
}

interface DashboardAccessRoleDraftsState {
  guildID: string;
  dashboardReadRoleIds: string[];
  dashboardWriteRoleIds: string[];
  verificationRoleId: string;
  hasUnsavedChanges: boolean;
}

function useDashboardAccessRoleDrafts({
  authState,
  selectedGuildID,
  dashboardReadRoleIds,
  dashboardWriteRoleIds,
  verificationRoleId,
}: DashboardAccessRoleDraftsArgs) {
  const normalizedGuildID = selectedGuildID.trim();
  const [drafts, setDrafts] = useState<DashboardAccessRoleDraftsState>(() =>
    createDashboardAccessRoleDraftsState(
      authState,
      normalizedGuildID,
      dashboardReadRoleIds,
      dashboardWriteRoleIds,
      verificationRoleId,
    ),
  );

  useEffect(() => {
    setDrafts((currentDrafts) => {
      const nextDrafts = createDashboardAccessRoleDraftsState(
        authState,
        normalizedGuildID,
        dashboardReadRoleIds,
        dashboardWriteRoleIds,
        verificationRoleId,
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
    verificationRoleId,
    normalizedGuildID,
  ]);

  function replaceDrafts(
    nextReadRoleIds: string[],
    nextWriteRoleIds: string[],
    nextVerificationRoleId: string,
  ) {
    setDrafts({
      guildID: normalizedGuildID,
      dashboardReadRoleIds: nextReadRoleIds,
      dashboardWriteRoleIds: nextWriteRoleIds,
      verificationRoleId: nextVerificationRoleId,
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

  function setVerificationRoleId(roleId: string) {
    setDrafts((currentDrafts) => ({
      ...currentDrafts,
      verificationRoleId: roleId,
      hasUnsavedChanges: true,
    }));
  }

  return {
    dashboardReadRoleIds: drafts.dashboardReadRoleIds,
    dashboardWriteRoleIds: drafts.dashboardWriteRoleIds,
    verificationRoleId: drafts.verificationRoleId,
    hasUnsavedChanges: drafts.hasUnsavedChanges,
    replaceDrafts,
    toggleDashboardReadRole,
    toggleDashboardWriteRole,
    setVerificationRoleId,
  };
}

function createDashboardAccessRoleDraftsState(
  authState: string,
  normalizedGuildID: string,
  dashboardReadRoleIds: string[],
  dashboardWriteRoleIds: string[],
  verificationRoleId: string,
): DashboardAccessRoleDraftsState {
  if (authState !== "signed_in" || normalizedGuildID === "") {
    return {
      guildID: "",
      dashboardReadRoleIds: [],
      dashboardWriteRoleIds: [],
      verificationRoleId: "",
      hasUnsavedChanges: false,
    };
  }

  return {
    guildID: normalizedGuildID,
    dashboardReadRoleIds,
    dashboardWriteRoleIds,
    verificationRoleId,
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
    currentDrafts.verificationRoleId === nextDrafts.verificationRoleId &&
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
