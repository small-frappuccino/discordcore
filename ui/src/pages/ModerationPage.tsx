import {
  PageHeader,
  SurfaceCard,
  SettingsGroup,
  SettingsRow,
  Button,
  Badge,
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
    <div>
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
            <SettingsRow
              title="AutoMod Engine"
              description="Automatically detect and block forbidden content."
              control={
                <Button
                  variant={automodEnabled ? "danger" : "primary"}
                  onClick={handleToggleAutomod}
                >
                  {automodEnabled ? "Disable" : "Enable"}
                </Button>
              }
            />
            <SettingsRow
              title="Audit Logging"
              description="Log moderation actions and deleted messages to a dedicated channel."
              control={
                <Button
                  variant={loggingEnabled ? "danger" : "primary"}
                  onClick={handleToggleLogging}
                >
                  {loggingEnabled ? "Disable" : "Enable"}
                </Button>
              }
            />
          </SettingsGroup>
          
          <form onSubmit={onSubmit} className="mt-8">
            <h3 className="mb-4 text-lg">Roles Config</h3>
            <SettingsGroup>
              <SettingsRow
                title="Mute Role"
                description="Role assigned when a user is muted."
                control={
                  <input
                    type="text"
                    {...form.register("mute_role")}
                    placeholder="Role ID..."
                    className="form-input w-[200px]"
                  />
                }
                isLast
              />
            </SettingsGroup>
            <div className="mt-4">
              <Button variant="primary" type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Mute Role"}
              </Button>
            </div>
          </form>
        </SurfaceCard>
      )}
    </div>
  );
}
