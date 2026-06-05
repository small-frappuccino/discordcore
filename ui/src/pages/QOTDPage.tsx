import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, SettingsGroupSkeleton, FormControl, TransitionState } from "../components/ui";
import { Stack, Cluster } from "../components/layout";
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
    <TransitionState
      isLoading={isLoading}
      fallback={
        <PageContainer>
          <Stack spacing="xl">
            <PageHeader 
              title="Question of the Day" 
              description="Configure the automated QOTD system. When enabled, the bot will pick a question from the active deck and publish it daily."
              badge={<Badge variant="neutral">Loading</Badge>}
            />
            <Stack spacing="lg">
              <SettingsGroupSkeleton rows={3} />
              <Stack spacing="sm">
                <h2 className="text-lg">Publish Schedule (UTC)</h2>
                <SettingsGroupSkeleton rows={2} />
              </Stack>
            </Stack>
          </Stack>
        </PageContainer>
      }
    >
      <PageContainer>
        <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
          <Stack as="form" spacing="xl" onSubmit={onSubmit}>
            <PageHeader 
              title="Question of the Day" 
              description="Configure the automated QOTD system. When enabled, the bot will pick a question from the active deck and publish it daily."
              badge={<Badge variant={config ? "success" : "neutral"}>{config ? "Active" : "Disabled"}</Badge>}
            />

            {config ? (
          <Stack spacing="xl">
            <Stack spacing="sm">
              <h2 className="text-lg">Core Settings</h2>
              <SettingsGroup>
                <SettingsRow>
                  <SettingsRow.Info>
                    <SettingsRow.Title>Active Deck</SettingsRow.Title>
                    <SettingsRow.Description>{`Currently active deck for drawing questions. ${activeDeck ? `Remaining cards: ${activeDeck.name}` : ""}`}</SettingsRow.Description>
                  </SettingsRow.Info>
                  <SettingsRow.Control>
                    <FormControl asChild>
                      <select 
                        {...form.register("active_deck_id")}
                        className="form-select"
                      >
                        <option value="">-- No Active Deck --</option>
                        {config.decks?.map(d => (
                          <option key={d.id} value={d.id}>{d.name}</option>
                        ))}
                      </select>
                    </FormControl>
                  </SettingsRow.Control>
                </SettingsRow>
                <SettingsRow>
                  <SettingsRow.Info>
                    <SettingsRow.Title>Verified Role (Optional)</SettingsRow.Title>
                    <SettingsRow.Description>If set, only users with this role can answer the QOTD.</SettingsRow.Description>
                  </SettingsRow.Info>
                  <SettingsRow.Control>
                    <FormControl asChild>
                      <input
                        type="text"
                        placeholder="Role ID..."
                        {...form.register("verified_role_id")}
                        className="form-input"
                      />
                    </FormControl>
                  </SettingsRow.Control>
                </SettingsRow>
              </SettingsGroup>
            </Stack>

            <Stack spacing="sm">
              <h2 className="text-lg">Publish Schedule (UTC)</h2>
              <SettingsGroup>
                <SettingsRow>
                  <SettingsRow.Info>
                    <SettingsRow.Title>Hour & Minute</SettingsRow.Title>
                    <SettingsRow.Description>The exact UTC time when the question should be posted.</SettingsRow.Description>
                  </SettingsRow.Info>
                  <SettingsRow.Control>
                    <Cluster spacing="sm" align="center">
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
                    </Cluster>
                  </SettingsRow.Control>
                </SettingsRow>
              </SettingsGroup>
            </Stack>

            <Stack direction="horizontal" spacing="sm">
              <Button variant="primary" type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Changes"}
              </Button>
            </Stack>
          </Stack>
        ) : (
          <p className="text-muted">Failed to load QOTD settings.</p>
            )}
          </Stack>
        </fieldset>
      </PageContainer>
    </TransitionState>
  );
}
