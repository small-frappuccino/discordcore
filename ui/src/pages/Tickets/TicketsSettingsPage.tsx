import { SettingsGroupSkeleton, FormProvider } from "../../components/ui";
import {
  SettingsGroup,
  SettingsRow,
  ToggleSwitch,
  ActionTrigger,
  TextInput
} from "../../components/ui/tahoe";
import { Stack } from "../../components/layout";
import { useTicketsSettingsLogic } from "./hooks/useTicketsSettingsLogic";

export function TicketsSettingsPage() {
  const { isLoading, isSaving, form, onSubmit } = useTicketsSettingsLogic();

  return (
    <>
      {isLoading ? (
        <Stack spacing="xl">
          <div>
            <h2 className="text-xl font-semibold">Automation Settings</h2>
            <p className="text-muted">Configure auto-close timers, transcript logs, and system enablement.</p>
          </div>
          <SettingsGroupSkeleton rows={3} />
        </Stack>
      ) : (
      <Stack spacing="xl">
        <div>
          <h2 className="text-xl font-semibold">Automation Settings</h2>
          <p className="text-muted">Configure auto-close timers, transcript logs, and system enablement.</p>
        </div>

        <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
          <FormProvider {...form}>
            <form className="settings-form" onSubmit={onSubmit}>
              <Stack spacing="xl">
              <h3 className="text-lg font-semibold tracking-tight">Core System</h3>
            <SettingsGroup>
              <SettingsRow
                title="Enable Tickets System"
                description="If disabled, all ticket panels will stop working."
                control={<ToggleSwitch {...form.register("enabled")} />}
              />
            </SettingsGroup>

              <h3 className="text-lg font-semibold tracking-tight">Automation & Logging</h3>
              <SettingsGroup>
              <SettingsRow
                title="Transcript Log Channel"
                description="Channel where HTML transcripts are sent when tickets close."
                control={
                  <TextInput
                    placeholder="Discord Channel ID"
                    {...form.register("automation.transcriptChannelId")}
                  />
                }
              />

              <SettingsRow
                title="Auto-Close Timer (Hours)"
                description="Automatically close the ticket if no one sends a message for this long. Set to 0 to disable."
                control={
                  <TextInput
                    type="number"
                    min="0"
                    {...form.register("automation.autoCloseTimerHours", { valueAsNumber: true })}
                    className="w-24"
                  />
                }
              />

              <SettingsRow
                title="Inactivity Warning (Hours)"
                description="Ping the ticket creator with a warning before auto-closing. Set to 0 to disable."
                control={
                  <TextInput
                    type="number"
                    min="0"
                    {...form.register("automation.inactivityWarningHours", { valueAsNumber: true })}
                    className="w-24"
                  />
                }
              />
              </SettingsGroup>

          <div className="form-actions">
            <ActionTrigger type="submit" variant="primary" isLoading={isSaving}>
              Save Settings
            </ActionTrigger>
          </div>
              </Stack>
            </form>
          </FormProvider>
        </fieldset>
      </Stack>
      )}
    </>
  );
}
