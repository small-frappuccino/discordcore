import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, SettingsGroupSkeleton, FormControl, FormProvider, FormSelect, FormInput } from "../components/ui";
import { Stack, Box } from "../components/layout";
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
      <Box as="fieldset" p="none" m="none" className="border-none min-w-0">
        <FormProvider {...form}>
          <Stack as="form" spacing="xl" onSubmit={onSubmit}>
            <PageHeader>
              <PageHeader.TitleRow>
                <PageHeader.Title>Question of the Day</PageHeader.Title>
                <Badge variant={isLoading ? "neutral" : (config ? "success" : "neutral")}>{isLoading ? "Loading" : (config ? "Active" : "Disabled")}</Badge>
              </PageHeader.TitleRow>
              <PageHeader.Description>Configure the automated QOTD system. When enabled, the bot will pick a question from the active deck and publish it daily.</PageHeader.Description>
            </PageHeader>

            {isLoading ? (
              <Stack spacing="lg">
                <SettingsGroupSkeleton rows={3} />
                <Stack spacing="sm">
                  <h2 className="text-lg">Publish Schedule (UTC)</h2>
                  <SettingsGroupSkeleton rows={2} />
                </Stack>
              </Stack>
            ) : config ? (
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
                          <FormSelect name="active_deck_id">
                            <option value="">-- No Active Deck --</option>
                            {config.decks?.map(d => (
                              <option key={d.id} value={d.id}>{d.name}</option>
                            ))}
                          </FormSelect>
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
                          <FormInput
                            name="verified_role_id"
                            placeholder="Role ID..."
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
                        <div style={{ display: 'flex', flexDirection: 'row', gap: '8px', alignItems: 'center' }}>
                          <FormInput
                            type="number"
                            min="0"
                            max="23"
                            name="schedule.hour_utc"
                            rules={{ valueAsNumber: true }}
                            className="w-16"
                          />
                          <span className="text-muted">:</span>
                          <FormInput
                            type="number"
                            min="0"
                            max="59"
                            name="schedule.minute_utc"
                            rules={{ valueAsNumber: true }}
                            className="w-16"
                          />
                        </div>
                      </SettingsRow.Control>
                    </SettingsRow>
                  </SettingsGroup>
                </Stack>

                <div className="form-actions">
                  <Button variant="primary" type="submit" isLoading={isSaving}>
                    Save Changes
                  </Button>
                </div>
              </Stack>
            ) : (
              <p className="text-muted">Failed to load QOTD settings.</p>
            )}
          </Stack>
        </FormProvider>
      </Box>
    </PageContainer>
  );
}
