import { PageHeader, SettingsGroup, SettingsRow, Button, Badge } from "../components/ui";
import { useEmbedsPageLogic } from "./hooks/useEmbedsPageLogic";

export function EmbedsPage() {
  const {
    form,
    onSubmit,
    isLoading,
    isSaving,
  } = useEmbedsPageLogic();

  return (
    <form onSubmit={onSubmit}>
      <PageHeader 
        title="Custom Embeds" 
        description="Configure webhooks to send rich embeds into your server."
        badge={<Badge variant="success">Active</Badge>}
      />

      <div className="mt-8">
        {isLoading ? (
          <p className="text-muted">Loading Embed settings...</p>
        ) : (
          <div>
            <h2 className="text-lg mb-4">Webhook Settings</h2>
            
            <SettingsGroup className="mb-8">
              <SettingsRow 
                title="Enable Custom Embeds"
                description="Toggle whether the bot should use custom webhooks for embeds."
                control={
                  <input
                    type="checkbox"
                    {...form.register("enabled")}
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
                    {...form.register("webhook_url")}
                    style={{ width: "300px", padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                  />
                }
              />
            </SettingsGroup>

            <div className="mt-4">
              <Button variant="primary" type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Changes"}
              </Button>
            </div>
          </div>
        )}
      </div>
    </form>
  );
}
