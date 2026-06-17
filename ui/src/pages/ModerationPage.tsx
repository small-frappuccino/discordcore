import {
  PageHeader,
  Badge,
  PageContainer,
  Skeleton,
  SettingsGroupSkeleton,
  FormProvider,
} from "../components/ui";
import {
  SettingsGroup,
  SettingsRow,
  ToggleSwitch,
  TextInput,
  SaveActionBar
} from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useModerationPageLogic } from "./hooks/useModerationPageLogic";

export function ModerationPage() {
  const {
    selectedGuildID,
    isLoading,
    isSaving,
    automodEnabled,
    loggingEnabled,
    form,
    onSubmit,
    handleToggleAutomod,
    handleToggleLogging,
  } = useModerationPageLogic();

  if (!selectedGuildID) {
    return <div>Select a server to manage moderation.</div>;
  }

  return (
    <>
      {isLoading ? (
        <PageContainer>
          <div className="settings-form">
            <Stack spacing="xl">
              <PageHeader>
                <PageHeader.TitleRow>
                  <PageHeader.Title>Moderation</PageHeader.Title>
                  <Badge variant="neutral">Loading</Badge>
                </PageHeader.TitleRow>
                <PageHeader.Description>Configure AutoMod, Logging, and specific moderation roles.</PageHeader.Description>
              </PageHeader>
              <Stack spacing="xl">
                <Stack spacing="sm">
                  <Skeleton className="h-6 w-48" />
                  <SettingsGroupSkeleton rows={2} />
                </Stack>
                <Stack spacing="sm">
                  <Skeleton className="h-6 w-48" />
                  <SettingsGroupSkeleton rows={1} />
                </Stack>
              </Stack>
            </Stack>
          </div>
        </PageContainer>
      ) : (
      <PageContainer>
        <form className="settings-form" onSubmit={onSubmit}>
          <Stack spacing="xl">
            <PageHeader>
              <PageHeader.TitleRow>
                <PageHeader.Title>Moderation</PageHeader.Title>
                <Badge variant="success">Active</Badge>
              </PageHeader.TitleRow>
              <PageHeader.Description>Configure AutoMod, Logging, and specific moderation roles.</PageHeader.Description>
            </PageHeader>

            <Stack spacing="xl">
              <SettingsGroup>
                <SettingsRow
                  title="AutoMod Engine"
                  description="Automatically detect and block forbidden content."
                  control={<ToggleSwitch checked={automodEnabled} onChange={handleToggleAutomod} />}
                />
                <SettingsRow
                  title="Audit Logging"
                  description="Log moderation actions and deleted messages to a dedicated channel."
                  control={<ToggleSwitch checked={loggingEnabled} onChange={handleToggleLogging} />}
                />
              </SettingsGroup>
              
              <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
                <FormProvider {...form}>
                  <Stack spacing="lg">
                  <Stack spacing="sm">
                    <h3 className="text-lg font-semibold tracking-tight text-text-primary">Roles Config</h3>
                    <SettingsGroup>
                      <SettingsRow
                        title="Mute Role"
                        description="Role assigned when a user is muted."
                        control={<TextInput placeholder="Role ID..." {...form.register("mute_role")} />}
                      />
                    </SettingsGroup>
                  </Stack>
                  </Stack>
                </FormProvider>
              </fieldset>
            </Stack>
          </Stack>
        </form>
        <SaveActionBar
          isDirty={form.formState.isDirty}
          isSaving={isSaving}
          onSave={onSubmit}
          onReset={() => form.reset()}
        />
      </PageContainer>
      )}
    </>
  );
}
