import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, Skeleton, Select, SettingsGroupSkeleton } from "../components/ui";
import { useRolesPageLogic } from "./hooks/useRolesPageLogic";


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
        <div className="mt-8 mb-4">
          <Skeleton className="h-6 w-48 mb-2" />
          <SettingsGroupSkeleton rows={2} />
        </div>
        <div className="mb-4">
          <Skeleton className="h-6 w-48 mb-2" />
          <SettingsGroupSkeleton rows={3} />
        </div>
        <div className="mb-4">
          <Skeleton className="h-6 w-48 mb-2" />
          <SettingsGroupSkeleton rows={2} />
        </div>
      </PageContainer>
    );
  }





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
              <Select multiple className="w-full max-w-xs" {...form.register("dashboard_read")}>
                {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Write Access Roles</SettingsRow.Title>
              <SettingsRow.Description>Roles allowed to view and edit dashboard settings</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              <Select multiple className="w-full max-w-xs" {...form.register("dashboard_write")}>
                {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
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
              <Select className="w-full max-w-xs" {...form.register("auto_assignment.target_role")}>
                <option value="">-- None --</option>
                {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Required Roles</SettingsRow.Title>
              <SettingsRow.Description>Users must have all these roles to get the target role</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              <Select multiple className="w-full max-w-xs" {...form.register("auto_assignment.required_roles")}>
                {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
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
              <Select className="w-full max-w-xs" {...form.register("mute_role")}>
                <option value="">-- None --</option>
                {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Booster Role</SettingsRow.Title>
              <SettingsRow.Description>Role representing Nitro Boosters</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              <Select className="w-full max-w-xs" {...form.register("booster_role")}>
                <option value="">-- None --</option>
                {roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
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
