import {
  PageHeader,
  SurfaceCard,
  SettingsGroup,
  SettingsRow,
  Button,
  Badge,
  PageContainer,
} from "../components/ui";
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
    <PageContainer>
      <PageHeader
        title="Moderation"
        description="Configure AutoMod, Logging, and specific moderation roles."
        badge={<Badge variant="success">Active</Badge>}
      />

      {isLoading ? (
        <div className="mt-8 text-muted">Loading settings...</div>
      ) : (
        <SurfaceCard className="mt-8">
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
          
          <form onSubmit={onSubmit} className="mt-8">
            <h3 className="mb-4 text-lg">Roles Config</h3>
            <SettingsGroup>
              <SettingsRow isLast>
                <SettingsRow.Info>
                  <SettingsRow.Title>Mute Role</SettingsRow.Title>
                  <SettingsRow.Description>Role assigned when a user is muted.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <input
                    type="text"
                    {...form.register("mute_role")}
                    placeholder="Role ID..."
                    className="form-input w-full max-w-xs"
                  />
                </SettingsRow.Control>
              </SettingsRow>
            </SettingsGroup>
            <div className="mt-4">
              <Button variant="primary" type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Mute Role"}
              </Button>
            </div>
          </form>
        </SurfaceCard>
      )}
    </PageContainer>
  );
}
