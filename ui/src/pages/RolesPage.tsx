import { useDashboardSession } from "../context/DashboardSessionContext";
import { PageHeader, SettingsGroup, SettingsRow, Button, Badge } from "../components";
import { useRolesPage } from "./hooks/useRolesPage";

export function RolesPage() {
  const { selectedGuildID } = useDashboardSession();

  const { roles, loading, saving, formControls, handleSave } = useRolesPage(selectedGuildID);

  if (!selectedGuildID) {
    return <div>Select a guild</div>;
  }

  if (loading) {
    return <div>Loading roles settings...</div>;
  }

  const {
    dashboardRead, setDashboardRead,
    dashboardWrite, setDashboardWrite,
    boosterRole, setBoosterRole,
    muteRole, setMuteRole,
    autoAssignEnabled, setAutoAssignEnabled,
    autoAssignTarget, setAutoAssignTarget,
    autoAssignRequired, setAutoAssignRequired,
  } = formControls;

  const selectClass = "bg-[#18181b] text-[#f4f4f5] border border-white/10 rounded-md px-3 py-2 outline-none min-w-[200px] focus:border-[#5865F2] transition-colors";

  const renderMultiSelect = (val: string[], setVal: (v: string[]) => void) => (
    <select
      multiple
      className={`${selectClass} h-24`}
      value={val}
      onChange={e => setVal(Array.from(e.target.selectedOptions, o => o.value))}
    >
      {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
    </select>
  );

  const renderSelect = (val: string, setVal: (v: string) => void) => (
    <select
      className={selectClass}
      value={val}
      onChange={e => setVal(e.target.value)}
    >
      <option value="">-- None --</option>
      {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
    </select>
  );

  return (
    <div className="flex flex-col">
      <PageHeader 
        title="Roles Configuration" 
        description="Manage which roles grant dashboard access, and configure server-wide specific roles like AutoAssignment, Mute, and Booster."
        badge={<Badge variant="success">Active</Badge>}
      />

      <div className="mt-8 mb-4">
        <h2 className="text-lg font-semibold mb-2 text-[#f4f4f5]">Dashboard Access</h2>
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
        <h2 className="text-lg font-semibold mb-2 text-[#f4f4f5]">Auto Assignment</h2>
        <SettingsGroup>
          <SettingsRow 
            title="Enable Auto Assignment"
            description="Automatically assign the target role to users that have required roles"
            control={
              <input 
                type="checkbox" 
                className="w-4 h-4 rounded border-gray-300 text-[#5865F2] focus:ring-[#5865F2]"
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
        <h2 className="text-lg font-semibold mb-2 text-[#f4f4f5]">Special Roles</h2>
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

      <div className="mt-8 flex items-center gap-2">
        <Button variant="primary" onClick={handleSave} disabled={saving}>
          {saving ? "Saving..." : "Save Changes"}
        </Button>
      </div>
    </div>
  );
}
