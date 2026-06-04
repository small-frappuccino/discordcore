
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
    automodEnabled,
    loggingEnabled,
    muteRole,
    setMuteRole,
    handleToggleAutomod,
    handleToggleLogging,
    handleSaveMuteRole,
  } = useModerationPageLogic();

  if (!selectedGuildID) {
    return <div>Select a server to manage moderation.</div>;
  }

  return (
    <div className="moderation-page">
      <PageHeader
        title="Moderation"
        description="Configure automod rules, logging, and mute settings."
        badge={<Badge variant="success">Active</Badge>}
      />

      {isLoading ? (
        <div className="mt-8 text-muted">Loading settings...</div>
      ) : (
        <SurfaceCard className="mt-8">
          <SettingsGroup>
            <SettingsRow
              title="Auto-Moderation"
              description="Automatically filter and restrict malicious behavior based on custom rules."
              control={
                <Button
                  variant={automodEnabled ? "primary" : "secondary"}
                  onClick={handleToggleAutomod}
                >
                  {automodEnabled ? "Enabled" : "Disabled"}
                </Button>
              }
            />
            <SettingsRow
              title="Moderation Logging"
              description="Keep a record of kicks, bans, and automod actions."
              control={
                <Button
                  variant={loggingEnabled ? "primary" : "secondary"}
                  onClick={handleToggleLogging}
                >
                  {loggingEnabled ? "Enabled" : "Disabled"}
                </Button>
              }
            />
            <SettingsRow
              title="Mute Role"
              description="The role assigned to muted users."
              isLast={true}
              control={
                <div className="flex-row">
                  <input
                    type="text"
                    value={muteRole}
                    onChange={(e) => setMuteRole(e.target.value)}
                    placeholder="Role ID"
                    style={{
                      padding: "8px",
                      borderRadius: "4px",
                      border: "1px solid var(--border-subtle)",
                      background: "var(--bg-base)",
                      color: "var(--text-primary)",
                      width: "150px"
                    }}
                  />
                  <Button variant="primary" onClick={handleSaveMuteRole}>
                    Save
                  </Button>
                </div>
              }
            />
          </SettingsGroup>
        </SurfaceCard>
      )}
    </div>
  );
}
