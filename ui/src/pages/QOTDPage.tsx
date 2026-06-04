import { PageHeader, SettingsGroup, SettingsRow, Button, Badge } from "../components/ui";
import { useQOTDPageLogic } from "./hooks/useQOTDPageLogic";

export function QOTDPage() {
  const {
    config,
    form,
    activeDeck,
    isLoading,
    isSaving,
    onSubmit,
  } = useQOTDPageLogic();

  return (
    <form onSubmit={onSubmit}>
      <PageHeader 
        title="Question of the Day" 
        description="Configure the automated QOTD system. When enabled, the bot will pick a question from the active deck and publish it daily."
        badge={<Badge variant={config ? "success" : "neutral"}>{config ? "Active" : "Disabled"}</Badge>}
      />

      <div className="mt-8">
        {isLoading ? (
          <p className="text-muted">Loading QOTD settings...</p>
        ) : config ? (
          <div>
            <h2 className="text-lg mb-4">Core Settings</h2>
            
            <SettingsGroup className="mb-8">
              <SettingsRow 
                title="Active Deck"
                description={`Currently active deck for drawing questions. ${activeDeck ? `Remaining cards: ${activeDeck.name}` : ""}`}
                control={
                  <select 
                    {...form.register("active_deck_id")}
                    style={{ padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none", minWidth: "200px" }}
                  >
                    <option value="">-- No Active Deck --</option>
                    {config.decks?.map(d => (
                      <option key={d.id} value={d.id}>{d.name}</option>
                    ))}
                  </select>
                }
              />
              <SettingsRow 
                title="Verified Role (Optional)"
                description="If set, only users with this role can answer the QOTD."
                isLast
                control={
                  <input
                    type="text"
                    placeholder="Role ID..."
                    {...form.register("verified_role_id")}
                    style={{ width: "200px", padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                  />
                }
              />
            </SettingsGroup>

            <h2 className="text-lg mb-4">Publish Schedule (UTC)</h2>
            <SettingsGroup className="mb-8">
              <SettingsRow 
                title="Hour & Minute"
                description="The exact UTC time when the question should be posted."
                isLast
                control={
                  <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
                    <input
                      type="number"
                      min="0"
                      max="23"
                      {...form.register("schedule.hour_utc", { valueAsNumber: true })}
                      style={{ width: "60px", padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                    />
                    <span className="text-muted">:</span>
                    <input
                      type="number"
                      min="0"
                      max="59"
                      {...form.register("schedule.minute_utc", { valueAsNumber: true })}
                      style={{ width: "60px", padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                    />
                  </div>
                }
              />
            </SettingsGroup>

            <div className="mt-4">
              <Button variant="primary" type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Changes"}
              </Button>
            </div>
          </div>
        ) : (
          <p className="text-muted">Failed to load QOTD settings.</p>
        )}
      </div>
    </form>
  );
}
