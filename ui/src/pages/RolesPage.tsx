import { useMemo } from "react";
import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, Skeleton, Select, SettingsGroupSkeleton, FormControl, TransitionState } from "../components/ui";
import { Stack } from "../components/layout";
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

  const roleOptions = useMemo(() => {
    return roles.map(r => <option key={r.id} value={r.id}>{r.name}</option>);
  }, [roles]);

  if (!selectedGuildID) {
    return <div>Select a guild</div>;
  }

  return (
    <TransitionState
      isLoading={isLoading}
      fallback={
        <PageContainer>
          <Stack spacing="lg">
            <PageHeader 
              title="Roles Configuration" 
              description="Manage which roles grant dashboard access, and configure server-wide specific roles like AutoAssignment, Mute, and Booster."
              badge={<Badge variant="success">Active</Badge>}
            />
            <Stack spacing="sm">
              <Skeleton className="h-6 w-48" />
              <SettingsGroupSkeleton rows={2} />
            </Stack>
            <Stack spacing="sm">
              <Skeleton className="h-6 w-48" />
              <SettingsGroupSkeleton rows={3} />
            </Stack>
            <Stack spacing="sm">
              <Skeleton className="h-6 w-48" />
              <SettingsGroupSkeleton rows={2} />
            </Stack>
          </Stack>
        </PageContainer>
      }
    >
      <PageContainer>
        <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
          <Stack as="form" spacing="lg" onSubmit={onSubmit}>
        <PageHeader 
          title="Roles Configuration" 
          description="Manage which roles grant dashboard access, and configure server-wide specific roles like AutoAssignment, Mute, and Booster."
          badge={<Badge variant="success">Active</Badge>}
        />

        <Stack spacing="sm">
          <h2 className="text-lg font-semibold tracking-tight text-text-primary">Dashboard Access</h2>
          <SettingsGroup>
            <SettingsRow>
              <SettingsRow.Info>
                <SettingsRow.Title>Read Access Roles</SettingsRow.Title>
                <SettingsRow.Description>Roles allowed to view dashboard settings</SettingsRow.Description>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <FormControl asChild>
                  <Select multiple {...form.register("dashboard_read")}>
                    {roleOptions}
                  </Select>
                </FormControl>
              </SettingsRow.Control>
            </SettingsRow>
            <SettingsRow>
              <SettingsRow.Info>
                <SettingsRow.Title>Write Access Roles</SettingsRow.Title>
                <SettingsRow.Description>Roles allowed to view and edit dashboard settings</SettingsRow.Description>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <FormControl asChild>
                  <Select multiple {...form.register("dashboard_write")}>
                    {roleOptions}
                  </Select>
                </FormControl>
              </SettingsRow.Control>
            </SettingsRow>
          </SettingsGroup>
        </Stack>

        <Stack spacing="sm">
          <h2 className="text-lg font-semibold tracking-tight text-text-primary">Auto Assignment</h2>
          <SettingsGroup>
            <SettingsRow>
              <SettingsRow.Info>
                <SettingsRow.Title>Enable Auto Assignment</SettingsRow.Title>
                <SettingsRow.Description>Automatically assign the target role to users that have required roles</SettingsRow.Description>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <input 
                  type="checkbox" 
                  className="form-checkbox"
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
                <FormControl asChild>
                  <Select {...form.register("auto_assignment.target_role")}>
                    <option value="">-- None --</option>
                    {roleOptions}
                  </Select>
                </FormControl>
              </SettingsRow.Control>
            </SettingsRow>
            <SettingsRow>
              <SettingsRow.Info>
                <SettingsRow.Title>Required Roles</SettingsRow.Title>
                <SettingsRow.Description>Users must have all these roles to get the target role</SettingsRow.Description>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <FormControl asChild>
                  <Select multiple {...form.register("auto_assignment.required_roles")}>
                    {roleOptions}
                  </Select>
                </FormControl>
              </SettingsRow.Control>
            </SettingsRow>
          </SettingsGroup>
        </Stack>

        <Stack spacing="sm">
          <h2 className="text-lg font-semibold tracking-tight text-text-primary">Special Roles</h2>
          <SettingsGroup>
            <SettingsRow>
              <SettingsRow.Info>
                <SettingsRow.Title>Mute Role</SettingsRow.Title>
                <SettingsRow.Description>Role applied to muted users</SettingsRow.Description>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <FormControl asChild>
                  <Select {...form.register("mute_role")}>
                    <option value="">-- None --</option>
                    {roleOptions}
                  </Select>
                </FormControl>
              </SettingsRow.Control>
            </SettingsRow>
            <SettingsRow>
              <SettingsRow.Info>
                <SettingsRow.Title>Booster Role</SettingsRow.Title>
                <SettingsRow.Description>Role representing Nitro Boosters</SettingsRow.Description>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <FormControl asChild>
                  <Select {...form.register("booster_role")}>
                    <option value="">-- None --</option>
                    {roleOptions}
                  </Select>
                </FormControl>
              </SettingsRow.Control>
            </SettingsRow>
          </SettingsGroup>
        </Stack>

        <Stack direction="horizontal" spacing="sm" align="center">
          <Button variant="primary" type="submit" isLoading={isSaving}>
            Save Changes
          </Button>
        </Stack>
          </Stack>
        </fieldset>
      </PageContainer>
    </TransitionState>
  );
}
