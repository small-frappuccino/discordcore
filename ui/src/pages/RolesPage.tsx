import { useMemo } from "react";
import { PageHeader, Badge, PageContainer, Skeleton, SettingsGroupSkeleton, FormProvider } from "../components/ui";
import {
  SettingsGroup,
  SettingsRow,
  ToggleSwitch,
  SelectMenu,
  SelectMenuMultiple,
  ActionTrigger
} from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useRolesPageLogic } from "./hooks/useRolesPageLogic";
import { Controller } from "react-hook-form";

export function RolesPage() {
  const {
    selectedGuildID,
    isLoading,
    isSaving,
    roles,
    form,
    onSubmit,
  } = useRolesPageLogic();

  const selectOptions = useMemo(() => {
    return roles.map(r => ({ value: r.id, label: r.name }));
  }, [roles]);

  if (!selectedGuildID) {
    return <div>Select a guild</div>;
  }

  return (
    <PageContainer>
      <FormProvider {...form}>
        <form className="settings-form" onSubmit={onSubmit}>
          <Stack spacing="lg">
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
              <Stack spacing="xl">
                <div>
                  <h3 className="text-lg font-semibold tracking-tight text-text-primary mb-6">Dashboard Access</h3>
                  <SettingsGroup>
                    <SettingsRow
                      isMultiline
                      title="Read Access Roles"
                      description="Roles allowed to view dashboard settings"
                      control={
                        <Controller
                          name="dashboard_read"
                          control={form.control}
                          render={({ field }) => (
                            <SelectMenuMultiple
                              options={selectOptions}
                              value={field.value}
                              onChange={field.onChange}
                              className="w-full max-w-sm"
                            />
                          )}
                        />
                      }
                    />
                    <SettingsRow
                      isMultiline
                      title="Write Access Roles"
                      description="Roles allowed to view and edit dashboard settings"
                      control={
                        <Controller
                          name="dashboard_write"
                          control={form.control}
                          render={({ field }) => (
                            <SelectMenuMultiple
                              options={selectOptions}
                              value={field.value}
                              onChange={field.onChange}
                              className="w-full max-w-sm"
                            />
                          )}
                        />
                      }
                    />
                  </SettingsGroup>
                </div>

                <div>
                  <h3 className="text-lg font-semibold tracking-tight text-text-primary mb-6">Auto Assignment</h3>
                  <SettingsGroup>
                    <SettingsRow
                      title="Enable Auto Assignment"
                      description="Automatically assign the target role to users that have required roles"
                      control={<ToggleSwitch {...form.register("auto_assignment.enabled")} />}
                    />
                    <SettingsRow
                      title="Target Role"
                      description="The role to assign automatically"
                      control={
                        <Controller
                          name="auto_assignment.target_role"
                          control={form.control}
                          render={({ field }) => (
                            <SelectMenu
                              options={[{ value: "", label: "-- None --" }, ...selectOptions]}
                              value={field.value || ""}
                              onChange={field.onChange}
                            />
                          )}
                        />
                      }
                    />
                    <SettingsRow
                      isMultiline
                      title="Required Roles"
                      description="Users must have all these roles to get the target role"
                      control={
                        <Controller
                          name="auto_assignment.required_roles"
                          control={form.control}
                          render={({ field }) => (
                            <SelectMenuMultiple
                              options={selectOptions}
                              value={field.value}
                              onChange={field.onChange}
                              className="w-full max-w-sm"
                            />
                          )}
                        />
                      }
                    />
                  </SettingsGroup>
                </div>

                <div>
                  <h3 className="text-lg font-semibold tracking-tight text-text-primary mb-6">Special Roles</h3>
                  <SettingsGroup>
                    <SettingsRow
                      title="Mute Role"
                      description="Role applied to muted users"
                      control={
                        <Controller
                          name="mute_role"
                          control={form.control}
                          render={({ field }) => (
                            <SelectMenu
                              options={[{ value: "", label: "-- None --" }, ...selectOptions]}
                              value={field.value || ""}
                              onChange={field.onChange}
                            />
                          )}
                        />
                      }
                    />
                    <SettingsRow
                      title="Booster Role"
                      description="Role representing Nitro Boosters"
                      control={
                        <Controller
                          name="booster_role"
                          control={form.control}
                          render={({ field }) => (
                            <SelectMenu
                              options={[{ value: "", label: "-- None --" }, ...selectOptions]}
                              value={field.value || ""}
                              onChange={field.onChange}
                            />
                          )}
                        />
                      }
                    />
                  </SettingsGroup>
                </div>
              </Stack>
            )}
          </Stack>
          {!isLoading && (
            <div className="form-actions">
              <ActionTrigger variant="primary" type="submit" isLoading={isSaving}>
                Save Changes
              </ActionTrigger>
            </div>
          )}
        </form>
      </FormProvider>
    </PageContainer>
  );
}
