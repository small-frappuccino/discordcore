import {
  PageHeader,
  SurfaceCard,
  SettingsGroup,
  SettingsRow,
  Button,
  Badge,
  PageContainer,
  SettingsGroupSkeleton,
  FormControl,
  TransitionState,
  FormProvider,
  FormInput,
} from "../components/ui";
import { Stack, Box } from "../components/layout";
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
    <TransitionState
      isLoading={isLoading}
      fallback={
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
      }
    >
      <PageContainer>
        <Stack spacing="xl">
          <PageHeader>
            <PageHeader.TitleRow>
              <PageHeader.Title>Moderation</PageHeader.Title>
              <Badge variant="success">Active</Badge>
            </PageHeader.TitleRow>
            <PageHeader.Description>Configure AutoMod, Logging, and specific moderation roles.</PageHeader.Description>
          </PageHeader>

          <SurfaceCard>
            <Stack spacing="xl">
              <SettingsGroup>
                <SettingsRow>
                  <SettingsRow.Info>
                    <SettingsRow.Title>AutoMod Engine</SettingsRow.Title>
                    <SettingsRow.Description>Automatically detect and block forbidden content.</SettingsRow.Description>
                  </SettingsRow.Info>
                  <SettingsRow.Control>
                    <Button
                      variant={automodEnabled ? "danger" : "primary"}
                      onClick={handleToggleAutomod}
                    >
                      {automodEnabled ? "Disable" : "Enable"}
                    </Button>
                  </SettingsRow.Control>
                </SettingsRow>
                <SettingsRow>
                  <SettingsRow.Info>
                    <SettingsRow.Title>Audit Logging</SettingsRow.Title>
                    <SettingsRow.Description>Log moderation actions and deleted messages to a dedicated channel.</SettingsRow.Description>
                  </SettingsRow.Info>
                  <SettingsRow.Control>
                    <Button
                      variant={loggingEnabled ? "danger" : "primary"}
                      onClick={handleToggleLogging}
                    >
                      {loggingEnabled ? "Disable" : "Enable"}
                    </Button>
                  </SettingsRow.Control>
                </SettingsRow>
              </SettingsGroup>
              
              <Box as="fieldset" disabled={isSaving} p="none" m="none" className="border-none min-w-0">
                <FormProvider {...form}>
                  <Stack as="form" onSubmit={onSubmit} spacing="lg">
                  <Stack spacing="sm">
                    <h3 className="text-lg">Roles Config</h3>
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
                  <Stack direction="horizontal">
                    <Button variant="primary" type="submit" disabled={isSaving}>
                      {isSaving ? "Saving..." : "Save Mute Role"}
                    </Button>
                  </Stack>
                  </Stack>
                </FormProvider>
              </Box>
            </Stack>
          </SurfaceCard>
        </Stack>
      </PageContainer>
    </TransitionState>
  );
}
