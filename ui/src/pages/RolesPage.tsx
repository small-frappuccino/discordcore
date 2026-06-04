import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, Skeleton } from "../components/ui";
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
    return (
      <PageContainer>
        <PageHeader 
          title="Roles Configuration" 
          description="Manage which roles grant dashboard access, and configure server-wide specific roles like AutoAssignment, Mute, and Booster."
          badge={<Badge variant="success">Active</Badge>}
        />
        <Skeleton className="h-96 w-full mt-8" />
      </PageContainer>
    );
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
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Read Access Roles</SettingsRow.Title>
              <SettingsRow.Description>Roles allowed to view dashboard settings</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              {renderMultiSelect("dashboard_read")}
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow isLast>
            <SettingsRow.Info>
              <SettingsRow.Title>Write Access Roles</SettingsRow.Title>
              <SettingsRow.Description>Roles allowed to view and edit dashboard settings</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              {renderMultiSelect("dashboard_write")}
            </SettingsRow.Control>
          </SettingsRow>
        </SettingsGroup>
      </div>

      <div className="mb-4">
        <h2 className="text-lg mb-2">Auto Assignment</h2>
        <SettingsGroup>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Enable Auto Assignment</SettingsRow.Title>
              <SettingsRow.Description>Automatically assign the target role to users that have required roles</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              <input 
                type="checkbox" 
                {...form.register("auto_assignment.enabled")}
              />
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Target Role</SettingsRow.Title>
              <SettingsRow.Description>The role to assign automatically</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              {renderSelect("auto_assignment.target_role")}
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow isLast>
            <SettingsRow.Info>
              <SettingsRow.Title>Required Roles</SettingsRow.Title>
              <SettingsRow.Description>Users must have all these roles to get the target role</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              {renderMultiSelect("auto_assignment.required_roles")}
            </SettingsRow.Control>
          </SettingsRow>
        </SettingsGroup>
      </div>

      <div className="mb-4">
        <h2 className="text-lg mb-2">Special Roles</h2>
        <SettingsGroup>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Mute Role</SettingsRow.Title>
              <SettingsRow.Description>Role applied to muted users</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              {renderSelect("mute_role")}
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow isLast>
            <SettingsRow.Info>
              <SettingsRow.Title>Booster Role</SettingsRow.Title>
              <SettingsRow.Description>Role representing Nitro Boosters</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              {renderSelect("booster_role")}
            </SettingsRow.Control>
          </SettingsRow>
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
