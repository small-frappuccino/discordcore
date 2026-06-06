import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton, FormProvider } from "../components/ui";
import {
  SettingsGroup,
  SettingsRow,
  SelectMenu,
  ActionTrigger,
  TextInput
} from "../components/ui/tahoe";
import { Stack, Box } from "../components/layout";
import { useQOTDPageLogic } from "./hooks/useQOTDPageLogic";
import { Controller } from "react-hook-form";

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
          <Stack as="form" spacing="xl" onSubmit={onSubmit} className="settings-form">
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
                  <h3 className="text-lg font-semibold tracking-tight text-text-primary">Core Settings</h3>
                  <SettingsGroup>
                    <SettingsRow
                      title="Active Deck"
                      description={`Currently active deck for drawing questions. ${activeDeck ? `Remaining cards: ${activeDeck.name}` : ""}`}
                      control={
                        <Controller
                          name="active_deck_id"
                          control={form.control}
                          render={({ field }) => (
                            <SelectMenu
                              options={[
                                { value: "", label: "-- No Active Deck --" },
                                ...(config.decks?.map(d => ({ value: d.id, label: d.name })) || [])
                              ]}
                              value={field.value || ""}
                              onChange={field.onChange}
                            />
                          )}
                        />
                      }
                    />
                    <SettingsRow
                      title="Verified Role (Optional)"
                      description="If set, only users with this role can answer the QOTD."
                      control={
                        <TextInput
                          {...form.register("verified_role_id")}
                          placeholder="Role ID..."
                        />
                      }
                    />
                  </SettingsGroup>
                </Stack>

                <Stack spacing="sm">
                  <h3 className="text-lg font-semibold tracking-tight text-text-primary">Publish Schedule (UTC)</h3>
                  <SettingsGroup>
                    <SettingsRow
                      title="Hour & Minute"
                      description="The exact UTC time when the question should be posted."
                      control={
                        <div style={{ display: 'flex', flexDirection: 'row', gap: '8px', alignItems: 'center' }}>
                          <TextInput
                            type="number"
                            min="0"
                            max="23"
                            {...form.register("schedule.hour_utc", { valueAsNumber: true })}
                            className="w-16"
                          />
                          <span className="text-muted">:</span>
                          <TextInput
                            type="number"
                            min="0"
                            max="59"
                            {...form.register("schedule.minute_utc", { valueAsNumber: true })}
                            className="w-16"
                          />
                        </div>
                      }
                    />
                  </SettingsGroup>
                </Stack>

                <div className="form-actions">
                  <ActionTrigger variant="primary" type="submit" isLoading={isSaving}>
                    Save Changes
                  </ActionTrigger>
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
