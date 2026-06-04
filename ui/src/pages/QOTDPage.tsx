
import { PageHeader, SettingsGroup, SettingsRow, Button, Badge } from "../components/ui";
import { useQOTDPageLogic } from "./hooks/useQOTDPageLogic";

export function QOTDPage() {
  const {
    config,
    setConfig,
    activeDeck,
    isLoading,
    isSaving,
    handleSave,
  } = useQOTDPageLogic();

  return (
    <div>
      <PageHeader 
        title="Question of the Day" 
        description="Configure the QOTD decks, schedule, and channels."
        badge={config ? <Badge variant="success">Active</Badge> : <Badge variant="neutral">Loading</Badge>}
      />

      <div className="mt-8">
        {isLoading ? (
          <p className="text-muted">Loading QOTD settings...</p>
        ) : config ? (
          <div>
            <h2 className="text-lg mb-4">General Settings</h2>
            
            <SettingsGroup className="mb-8">
              <SettingsRow 
                title="Active Deck"
                description="Select which deck is currently being used for questions."
                control={
                  <select 
                    value={config.active_deck_id || ""}
                    onChange={e => setConfig({ ...config, active_deck_id: e.target.value })}
                    style={{ padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none", cursor: "pointer" }}
                  >
                    <option value="">None</option>
                    {config.decks?.map(deck => (
                      <option key={deck.id} value={deck.id}>{deck.name}</option>
                    ))}
                  </select>
                }
              />
              <SettingsRow 
                title="Publish Channel"
                description="The channel where questions will be posted for the active deck."
                control={
                  <input
                    type="text"
                    placeholder="Channel ID"
                    value={activeDeck?.channel_id || ""}
                    onChange={e => {
                      const newDecks = config.decks?.map(d => 
                        d.id === activeDeck?.id ? { ...d, channel_id: e.target.value } : d
                      );
                      setConfig({ ...config, decks: newDecks });
                    }}
                    style={{ padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                  />
                }
              />
              <SettingsRow 
                title="Publish Time (UTC)"
                description="The hour and minute (UTC) when the question is posted."
                isLast
                control={
                  <div className="flex-row">
                    <input
                      type="number"
                      min="0"
                      max="23"
                      placeholder="HH"
                      value={config.schedule?.hour_utc ?? 0}
                      onChange={e => setConfig({ 
                        ...config, 
                        schedule: { ...config.schedule, hour_utc: parseInt(e.target.value) || 0 } 
                      })}
                      style={{ width: "60px", padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                    />
                    <span>:</span>
                    <input
                      type="number"
                      min="0"
                      max="59"
                      placeholder="MM"
                      value={config.schedule?.minute_utc ?? 0}
                      onChange={e => setConfig({ 
                        ...config, 
                        schedule: { ...config.schedule, minute_utc: parseInt(e.target.value) || 0 } 
                      })}
                      style={{ width: "60px", padding: "8px", borderRadius: "6px", background: "var(--bg-surface-hover)", border: "1px solid var(--border-subtle)", color: "var(--text-primary)", outline: "none" }}
                    />
                  </div>
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
          <p className="text-muted">Failed to load QOTD settings.</p>
        )}
      </div>
    </div>
  );
}
