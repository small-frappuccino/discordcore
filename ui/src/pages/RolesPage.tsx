
import { PageHeader, SettingsGroup, SettingsRow, Button, Badge } from "../components/ui";
import { useRolesPageLogic } from "./hooks/useRolesPageLogic";

export function RolesPage() {
  const {
    selectedGuildID,
    isLoading,
    isSaving,
    roles,
    dashboardRead, setDashboardRead,
    dashboardWrite, setDashboardWrite,
    boosterRole, setBoosterRole,
    muteRole, setMuteRole,
    autoAssignEnabled, setAutoAssignEnabled,
    autoAssignTarget, setAutoAssignTarget,
    autoAssignRequired, setAutoAssignRequired,
    handleSave,
  } = useRolesPageLogic();

  if (!selectedGuildID) {
    return <div>Select a guild</div>;
  }

  if (isLoading) {
    return <div>Loading roles settings...</div>;
  }

  const selectStyle: React.CSSProperties = {
    backgroundColor: "var(--bg-base)",
    color: "var(--text-primary)",
    border: "1px solid var(--border-subtle)",
    borderRadius: "var(--radius-sm)",
    padding: "6px 8px",
    outline: "none",
    minWidth: "200px",
    fontFamily: "inherit"
  };

  const renderMultiSelect = (val: string[], setVal: (v: string[]) => void) => (
    <select
      multiple
      style={{...selectStyle, height: "100px"}}
      value={val}
      onChange={e => setVal(Array.from(e.target.selectedOptions, o => o.value))}
    >
      {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
    </select>
  );

  const renderSelect = (val: string, setVal: (v: string) => void) => (
    <select
      style={selectStyle}
      value={val}
      onChange={e => setVal(e.target.value)}
    >
      <option value="">-- None --</option>
      {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
    </select>
  );

  return (
    <div style={{ display: "flex", flexDirection: "column" }}>
      <PageHeader 
        title="Roles Configuration" 
        description="Manage which roles grant dashboard access, and configure server-wide specific roles like AutoAssignment, Mute, and Booster."
        badge={<Badge variant="success">Active</Badge>}
      />

      <div className="mt-8 mb-4">
        <h2 className="text-lg mb-2">Dashboard Access</h2>
        <SettingsGroup>
          <SettingsRow 
            title="Read Access Roles"
            description="Roles allowed to view dashboard settings"
            control={renderMultiSelect(dashboardRead, setDashboardRead)}
          />
          <SettingsRow 
            title="Write Access Roles"
            description="Roles allowed to view and edit dashboard settings"
            control={renderMultiSelect(dashboardWrite, setDashboardWrite)}
            isLast
          />
        </SettingsGroup>
      </div>

      <div className="mb-4">
        <h2 className="text-lg mb-2">Auto Assignment</h2>
        <SettingsGroup>
          <SettingsRow 
            title="Enable Auto Assignment"
            description="Automatically assign the target role to users that have required roles"
            control={
              <input 
                type="checkbox" 
                checked={autoAssignEnabled} 
                onChange={e => setAutoAssignEnabled(e.target.checked)} 
              />
            }
          />
          <SettingsRow 
            title="Target Role"
            description="The role to assign automatically"
            control={renderSelect(autoAssignTarget, setAutoAssignTarget)}
          />
          <SettingsRow 
            title="Required Roles"
            description="Users must have all these roles to get the target role"
            control={renderMultiSelect(autoAssignRequired, setAutoAssignRequired)}
            isLast
          />
        </SettingsGroup>
      </div>

      <div className="mb-4">
        <h2 className="text-lg mb-2">Special Roles</h2>
        <SettingsGroup>
          <SettingsRow 
            title="Mute Role"
            description="Role applied to muted users"
            control={renderSelect(muteRole, setMuteRole)}
          />
          <SettingsRow 
            title="Booster Role"
            description="Role representing Nitro Boosters"
            control={renderSelect(boosterRole, setBoosterRole)}
            isLast
          />
        </SettingsGroup>
      </div>

      <div className="mt-8 flex-row">
        <Button variant="primary" onClick={handleSave} disabled={isSaving}>
          {isSaving ? "Saving..." : "Save Changes"}
        </Button>
      </div>
    </div>
  );
}
