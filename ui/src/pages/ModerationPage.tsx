import {
  PageHeader,
  SettingsGroup,
  SettingsRow,
  Button,
  Badge,
  PageContainer,
  SettingsGroupSkeleton,
  FormControl,
  FormProvider,
  FormInput,
  ToggleSwitch
} from "../components/ui";
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
          <Stack spacing="xl">
            <PageHeader>
              <PageHeader.TitleRow>
                <PageHeader.Title>Moderation</PageHeader.Title>
                <Badge variant="neutral">Loading</Badge>
              </PageHeader.TitleRow>
              <PageHeader.Description>Configure AutoMod, Logging, and specific moderation roles.</PageHeader.Description>
            </PageHeader>
            <Stack spacing="lg">
              <SettingsGroupSkeleton rows={2} />
              <Stack spacing="sm">
                <h3 className="text-lg">Roles Config</h3>
                <SettingsGroupSkeleton rows={1} />
              </Stack>
            </Stack>
          </Stack>
        </PageContainer>
      ) : (
      <PageContainer>
        <Stack spacing="xl">
          <PageHeader>
            <PageHeader.TitleRow>
              <PageHeader.Title>Moderation</PageHeader.Title>
              <Badge variant="success">Active</Badge>
            </PageHeader.TitleRow>
            <PageHeader.Description>Configure AutoMod, Logging, and specific moderation roles.</PageHeader.Description>
          </PageHeader>

          <form className="settings-form" onSubmit={onSubmit}>
            <Stack spacing="xl">
              <SettingsGroup>
                <SettingsRow>
                  <SettingsRow.Info>
                    <SettingsRow.Title>AutoMod Engine</SettingsRow.Title>
                    <SettingsRow.Description>Automatically detect and block forbidden content.</SettingsRow.Description>
                  </SettingsRow.Info>
                  <SettingsRow.Control>
                    <ToggleSwitch
                      checked={automodEnabled}
                      onChange={handleToggleAutomod}
                    />
                  </SettingsRow.Control>
                </SettingsRow>
                <SettingsRow>
                  <SettingsRow.Info>
                    <SettingsRow.Title>Audit Logging</SettingsRow.Title>
                    <SettingsRow.Description>Log moderation actions and deleted messages to a dedicated channel.</SettingsRow.Description>
                  </SettingsRow.Info>
                  <SettingsRow.Control>
                    <ToggleSwitch
                      checked={loggingEnabled}
                      onChange={handleToggleLogging}
                    />
                  </SettingsRow.Control>
                </SettingsRow>
              </SettingsGroup>
              
              <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
                <FormProvider {...form}>
                  <Stack spacing="lg">
                  <Stack spacing="sm">
                    <h3 className="text-lg font-semibold tracking-tight">Roles Config</h3>
                    <SettingsGroup>
                      <SettingsRow>
                        <SettingsRow.Info>
                          <SettingsRow.Title>Mute Role</SettingsRow.Title>
                          <SettingsRow.Description>Role assigned when a user is muted.</SettingsRow.Description>
                        </SettingsRow.Info>
                        <SettingsRow.Control>
                          <FormControl asChild>
                            <FormInput
                              name="mute_role"
                              placeholder="Role ID..."
                            />
                          </FormControl>
                        </SettingsRow.Control>
                      </SettingsRow>
                    </SettingsGroup>
                  </Stack>
                  <div className="form-actions">
                    <Button variant="primary" type="submit" disabled={isSaving}>
                      {isSaving ? "Saving..." : "Save Mute Role"}
                    </Button>
                  </div>
                  </Stack>
                </FormProvider>
              </fieldset>
            </Stack>
          </form>
        </Stack>
      </PageContainer>
      )}
    </>
  );
}
