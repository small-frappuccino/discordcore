
import { PageHeader, SettingsGroup, SettingsRow, Button, Badge } from "../components/ui";
import { useEmbedsPageLogic } from "./hooks/useEmbedsPageLogic";

export function EmbedsPage() {
  const {
    config,
    setConfig,
    isLoading,
    isSaving,
    handleSave,
  } = useEmbedsPageLogic();

  return (
    <div>
      <PageHeader 
        title="Custom Embeds" 
        description="Configure webhooks to send rich embeds into your server."
        badge={<Badge variant={config?.enabled ? "success" : "neutral"}>{config?.enabled ? "Active" : "Disabled"}</Badge>}
      />

      <div className="mt-8">
        {isLoading ? (
          <p className="text-muted">Loading Embed settings...</p>
        ) : config ? (
          <div>
            <h2 className="text-lg mb-4">Webhook Settings</h2>
            
            <SettingsGroup className="mb-8">
              <SettingsRow 
                title="Enable Custom Embeds"
                description="Toggle whether the bot should use custom webhooks for embeds."
                control={
                  <input
                    type="checkbox"
                    checked={config.enabled}
                    onChange={e => setConfig({ ...config, enabled: e.target.checked })}
                    style={{ width: "20px", height: "20px", accentColor: "var(--accent-primary)", cursor: "pointer" }}
                  />
                }
              />
              <SettingsRow 
                title="Webhook URL"
                description="The Discord webhook URL where messages will be sent."
                isLast
                control={
                  <input
                    type="text"
                    placeholder="https://discord.com/api/webhooks/..."
                    value={config.webhook_url}
                    onChange={e => setConfig({ ...config, webhook_url: e.target.value })}
                    style={{ width: "300px", padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                  />
                }
              />
            </SettingsGroup>

            <div className="mt-4">
              <Button variant="primary" onClick={handleSave} disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Changes"}
              </Button>
            </div>
          </div>
        ) : (
          <p className="text-muted">Failed to load Embed settings.</p>
        )}
      </div>
    </div>
  );
}
