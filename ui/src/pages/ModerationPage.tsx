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
    <TransitionState
      isLoading={isLoading}
      fallback={
        <PageContainer>
          <Stack spacing="xl">
            <PageHeader
              title="Moderation"
              description="Configure AutoMod, Logging, and specific moderation roles."
              badge={<Badge variant="neutral">Loading</Badge>}
            />
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
          <PageHeader
            title="Moderation"
            description="Configure AutoMod, Logging, and specific moderation roles."
            badge={<Badge variant="success">Active</Badge>}
          />

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
              
              <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
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
                            <input
                              type="text"
                              {...form.register("mute_role")}
                              placeholder="Role ID..."
                              className="form-input"
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
              </fieldset>
            </Stack>
          </SurfaceCard>
        </Stack>
      </PageContainer>
    </TransitionState>
  );
}
