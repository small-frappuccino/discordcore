import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, SettingsGroupSkeleton } from "../components/ui";
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
    <PageContainer>
      <form onSubmit={onSubmit}>
      <PageHeader 
        title="Question of the Day" 
        description="Configure the automated QOTD system. When enabled, the bot will pick a question from the active deck and publish it daily."
        badge={<Badge variant={config ? "success" : "neutral"}>{config ? "Active" : "Disabled"}</Badge>}
      />

      <div className="mt-8">
        {isLoading ? (
          <div className="mt-8">
            <SettingsGroupSkeleton rows={3} />
            <h2 className="text-lg mb-4 mt-8">Publish Schedule (UTC)</h2>
            <SettingsGroupSkeleton rows={2} />
          </div>
        ) : config ? (
          <div>
            <h2 className="text-lg mb-4">Core Settings</h2>
            
            <SettingsGroup className="mb-8">
              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Active Deck</SettingsRow.Title>
                  <SettingsRow.Description>{`Currently active deck for drawing questions. ${activeDeck ? `Remaining cards: ${activeDeck.name}` : ""}`}</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <select 
                    {...form.register("active_deck_id")}
                    className="form-select w-full max-w-xs"
                  >
                    <option value="">-- No Active Deck --</option>
                    {config.decks?.map(d => (
                      <option key={d.id} value={d.id}>{d.name}</option>
                    ))}
                  </select>
                </SettingsRow.Control>
              </SettingsRow>
              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Verified Role (Optional)</SettingsRow.Title>
                  <SettingsRow.Description>If set, only users with this role can answer the QOTD.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <input
                    type="text"
                    placeholder="Role ID..."
                    {...form.register("verified_role_id")}
                    className="form-input w-full max-w-xs"
                  />
                </SettingsRow.Control>
              </SettingsRow>
            </SettingsGroup>

            <h2 className="text-lg mb-4">Publish Schedule (UTC)</h2>
            <SettingsGroup className="mb-8">
              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Hour & Minute</SettingsRow.Title>
                  <SettingsRow.Description>The exact UTC time when the question should be posted.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      min="0"
                      max="23"
                      {...form.register("schedule.hour_utc", { valueAsNumber: true })}
                      className="form-input w-16"
                    />
                    <span className="text-muted">:</span>
                    <input
                      type="number"
                      min="0"
                      max="59"
                      {...form.register("schedule.minute_utc", { valueAsNumber: true })}
                      className="form-input w-16"
                    />
                  </div>
                </SettingsRow.Control>
              </SettingsRow>
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
    </PageContainer>
  );
}
