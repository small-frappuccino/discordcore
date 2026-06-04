import { useEffect, useState } from "react";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  PageHeader,
  SurfaceCard,
  SettingsGroup,
  SettingsRow,
  Button,
  Badge,
} from "../components";

export function ModerationPage() {
  const { client, selectedGuildID } = useDashboardSession();
  const [automodEnabled, setAutomodEnabled] = useState(false);
  const [loggingEnabled, setLoggingEnabled] = useState(false);
  const [muteRole, setMuteRole] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!selectedGuildID) return;
    setLoading(true);
    Promise.all([
      client.getGuildFeature(selectedGuildID, "automod").catch(() => null),
      client.getGuildFeature(selectedGuildID, "logging").catch(() => null),
      client.getGuildSettings(selectedGuildID).catch(() => null),
    ]).then(([automodRes, loggingRes, settingsRes]) => {
      if (automodRes?.feature) {
        setAutomodEnabled(automodRes.feature.effective_enabled);
      }
      if (loggingRes?.feature) {
        setLoggingEnabled(loggingRes.feature.effective_enabled);
      }
      if (settingsRes?.workspace?.sections?.roles) {
        setMuteRole(settingsRes.workspace.sections.roles.mute_role || "");
      }
      setLoading(false);
    });
  }, [client, selectedGuildID]);

  const handleToggleAutomod = async () => {
    if (!selectedGuildID) return;
    const newVal = !automodEnabled;
    setAutomodEnabled(newVal);
    await client.patchGuildFeature(selectedGuildID, "automod", { enabled: newVal });
  };

  const handleToggleLogging = async () => {
    if (!selectedGuildID) return;
    const newVal = !loggingEnabled;
    setLoggingEnabled(newVal);
    await client.patchGuildFeature(selectedGuildID, "logging", { enabled: newVal });
  };

  const handleSaveMuteRole = async () => {
    if (!selectedGuildID) return;
    await client.updateGuildSettings(selectedGuildID, {
      roles: {
        mute_role: muteRole,
      },
    });
    alert("Mute role saved");
  };

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

      {loading ? (
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
