import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer } from "../components/ui";
import { useRolesPageLogic } from "./hooks/useRolesPageLogic";
import type { Path } from "react-hook-form";
import type { RolesFormData } from "./schemas/roles";

export function RolesPage() {
  const {
    selectedGuildID,
    isLoading,
    isSaving,
    roles,
    form,
    onSubmit,
  } = useRolesPageLogic();

  if (!selectedGuildID) {
    return <div>Select a guild</div>;
  }

  if (isLoading) {
    return <div>Loading roles settings...</div>;
  }



  const renderMultiSelect = (name: Path<RolesFormData>) => (
    <select
      multiple
      className="form-select w-full max-w-xs"
      {...form.register(name)}
    >
      {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
    </select>
  );

  const renderSelect = (name: Path<RolesFormData>) => (
    <select
      className="form-select w-full max-w-xs"
      {...form.register(name)}
    >
      <option value="">-- None --</option>
      {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
    </select>
  );

  return (
    <PageContainer>
      <form className="flex flex-col" onSubmit={onSubmit}>
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
            control={renderMultiSelect("dashboard_read")}
          />
          <SettingsRow 
            title="Write Access Roles"
            description="Roles allowed to view and edit dashboard settings"
            control={renderMultiSelect("dashboard_write")}
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
                {...form.register("auto_assignment.enabled")}
              />
            }
          />
          <SettingsRow 
            title="Target Role"
            description="The role to assign automatically"
            control={renderSelect("auto_assignment.target_role")}
          />
          <SettingsRow 
            title="Required Roles"
            description="Users must have all these roles to get the target role"
            control={renderMultiSelect("auto_assignment.required_roles")}
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
            control={renderSelect("mute_role")}
          />
          <SettingsRow 
            title="Booster Role"
            description="Role representing Nitro Boosters"
            control={renderSelect("booster_role")}
            isLast
          />
        </SettingsGroup>
      </div>

      <div className="mt-8 flex items-center gap-2">
        <Button variant="primary" type="submit" disabled={isSaving}>
          {isSaving ? "Saving..." : "Save Changes"}
        </Button>
      </div>
      </form>
    </PageContainer>
  );
}
