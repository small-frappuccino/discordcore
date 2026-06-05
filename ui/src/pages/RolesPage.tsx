import { useMemo } from "react";
import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, Skeleton, SettingsGroupSkeleton, FormControl, FormProvider, FormSelect, FormCheckbox } from "../components/ui";
import { Stack, Box } from "../components/layout";
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
    <PageContainer>
      <Box as="fieldset" p="none" m="none" className="border-none min-w-0">
        <FormProvider {...form}>
          <Stack as="form" spacing="lg" onSubmit={onSubmit}>
            <PageHeader>
              <PageHeader.TitleRow>
                <PageHeader.Title>Roles Configuration</PageHeader.Title>
                <Badge variant={isLoading ? "neutral" : "success"}>{isLoading ? "Loading" : "Active"}</Badge>
              </PageHeader.TitleRow>
              <PageHeader.Description>Manage which roles grant dashboard access, and configure server-wide specific roles like AutoAssignment, Mute, and Booster.</PageHeader.Description>
            </PageHeader>

            {isLoading ? (
              <>
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
              </>
            ) : (
              <>
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
                          <FormSelect multiple name="dashboard_read">
                            {roleOptions}
                          </FormSelect>
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
                          <FormSelect multiple name="dashboard_write">
                            {roleOptions}
                          </FormSelect>
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
                        <FormCheckbox name="auto_assignment.enabled" />
                      </SettingsRow.Control>
                    </SettingsRow>
                    <SettingsRow>
                      <SettingsRow.Info>
                        <SettingsRow.Title>Target Role</SettingsRow.Title>
                        <SettingsRow.Description>The role to assign automatically</SettingsRow.Description>
                      </SettingsRow.Info>
                      <SettingsRow.Control>
                        <FormControl asChild>
                          <FormSelect name="auto_assignment.target_role">
                            <option value="">-- None --</option>
                            {roleOptions}
                          </FormSelect>
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
                          <FormSelect multiple name="auto_assignment.required_roles">
                            {roleOptions}
                          </FormSelect>
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
                          <FormSelect name="mute_role">
                            <option value="">-- None --</option>
                            {roleOptions}
                          </FormSelect>
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
                          <FormSelect name="booster_role">
                            <option value="">-- None --</option>
                            {roleOptions}
                          </FormSelect>
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
              </>
            )}
          </Stack>
        </FormProvider>
      </Box>
    </PageContainer>
  );
}
