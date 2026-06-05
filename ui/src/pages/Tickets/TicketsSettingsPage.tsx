import { Button, SettingsGroupSkeleton, SurfaceCard, SettingsGroup, SettingsRow, FormControl, TransitionState, FormProvider, FormInput } from "../../components/ui";
import { Stack, Cluster } from "../../components/layout";
import { useTicketsSettingsLogic } from "./hooks/useTicketsSettingsLogic";

export function TicketsSettingsPage() {
  const { isLoading, isSaving, form, onSubmit } = useTicketsSettingsLogic();

  return (
    <TransitionState
      isLoading={isLoading}
      fallback={
        <Stack spacing="xl">
          <div>
            <h2 className="text-xl font-semibold">Automation Settings</h2>
            <p className="text-muted">Configure auto-close timers, transcript logs, and system enablement.</p>
          </div>
          <SettingsGroupSkeleton rows={3} />
        </Stack>
      }
    >
      <Stack spacing="xl">
        <div>
          <h2 className="text-xl font-semibold">Automation Settings</h2>
          <p className="text-muted">Configure auto-close timers, transcript logs, and system enablement.</p>
        </div>

        <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
          <FormProvider {...form}>
            <form onSubmit={onSubmit}>
              <Stack spacing="xl">
          <SurfaceCard>
            <Stack spacing="lg">
              <h3 className="text-lg font-semibold tracking-tight">Core System</h3>
            <SettingsGroup>
              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Enable Tickets System</SettingsRow.Title>
                  <SettingsRow.Description>If disabled, all ticket panels will stop working.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      {...form.register("enabled")}
                      className="sr-only peer"
                    />
                    <div className="w-11 h-6 bg-surface-border peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
                  </label>
                </SettingsRow.Control>
              </SettingsRow>
              </SettingsGroup>
            </Stack>
          </SurfaceCard>

          <SurfaceCard>
            <Stack spacing="lg">
              <h3 className="text-lg font-semibold tracking-tight">Automation & Logging</h3>
              <SettingsGroup>
              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Transcript Log Channel</SettingsRow.Title>
                  <SettingsRow.Description>Channel where HTML transcripts are sent when tickets close.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <FormControl asChild>
                    <FormInput
                      name="automation.transcriptChannelId"
                      placeholder="Discord Channel ID"
                    />
                  </FormControl>
                </SettingsRow.Control>
              </SettingsRow>

              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Auto-Close Timer (Hours)</SettingsRow.Title>
                  <SettingsRow.Description>Automatically close the ticket if no one sends a message for this long. Set to 0 to disable.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <FormInput
                    type="number"
                    min="0"
                    name="automation.autoCloseTimerHours"
                    rules={{ valueAsNumber: true }}
                    className="w-24"
                  />
                </SettingsRow.Control>
              </SettingsRow>

              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Inactivity Warning (Hours)</SettingsRow.Title>
                  <SettingsRow.Description>Ping the ticket creator with a warning before auto-closing. Set to 0 to disable.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <FormInput
                    type="number"
                    min="0"
                    name="automation.inactivityWarningHours"
                    rules={{ valueAsNumber: true }}
                    className="w-24"
                  />
                </SettingsRow.Control>
              </SettingsRow>
              </SettingsGroup>
            </Stack>
          </SurfaceCard>

          <Cluster justify="end" className="sticky bottom-4 z-10 p-4 bg-surface border border-surface-border rounded-lg shadow-lg">
            <Button type="submit" variant="primary" disabled={isSaving}>
              {isSaving ? "Saving..." : "Save Settings"}
            </Button>
          </Cluster>
              </Stack>
            </form>
          </FormProvider>
        </fieldset>
      </Stack>
    </TransitionState>
  );
}
