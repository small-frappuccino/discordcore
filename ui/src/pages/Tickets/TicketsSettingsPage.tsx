import { Button, SettingsGroupSkeleton, SurfaceCard, SettingsGroup, SettingsRow } from "../../components/ui";
import { useTicketsSettingsLogic } from "./hooks/useTicketsSettingsLogic";

export function TicketsSettingsPage() {
  const { isLoading, isSaving, form, onSubmit } = useTicketsSettingsLogic();

  return (
    <div>
      <div className="mb-6 flex justify-between items-center">
        <div>
          <h2 className="text-xl font-semibold">Automation Settings</h2>
          <p className="text-muted">Configure auto-close timers, transcript logs, and system enablement.</p>
        </div>
      </div>

      {isLoading ? (
        <SettingsGroupSkeleton rows={3} />
      ) : (
        <form onSubmit={onSubmit} className="space-y-8">
          <SurfaceCard className="p-6">
            <h3 className="text-lg font-semibold tracking-tight mb-6">Core System</h3>
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
          </SurfaceCard>

          <SurfaceCard className="p-6">
            <h3 className="text-lg font-semibold tracking-tight mb-6">Automation & Logging</h3>
            <SettingsGroup>
              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Transcript Log Channel</SettingsRow.Title>
                  <SettingsRow.Description>Channel where HTML transcripts are sent when tickets close.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <input
                    type="text"
                    {...form.register("automation.transcriptChannelId" as const)}
                    className="form-input w-full max-w-xs"
                    placeholder="Discord Channel ID"
                  />
                  {form.formState.errors?.automation?.transcriptChannelId && (
                    <p className="text-red-500 text-xs mt-1">
                      {form.formState.errors.automation.transcriptChannelId.message as string}
                    </p>
                  )}
                </SettingsRow.Control>
              </SettingsRow>

              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Auto-Close Timer (Hours)</SettingsRow.Title>
                  <SettingsRow.Description>Automatically close the ticket if no one sends a message for this long. Set to 0 to disable.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <input
                    type="number"
                    min="0"
                    {...form.register("automation.autoCloseTimerHours" as const, { valueAsNumber: true })}
                    className="form-input w-24"
                  />
                  {form.formState.errors?.automation?.autoCloseTimerHours && (
                    <p className="text-red-500 text-xs mt-1">
                      {form.formState.errors.automation.autoCloseTimerHours.message as string}
                    </p>
                  )}
                </SettingsRow.Control>
              </SettingsRow>

              <SettingsRow>
                <SettingsRow.Info>
                  <SettingsRow.Title>Inactivity Warning (Hours)</SettingsRow.Title>
                  <SettingsRow.Description>Ping the ticket creator with a warning before auto-closing. Set to 0 to disable.</SettingsRow.Description>
                </SettingsRow.Info>
                <SettingsRow.Control>
                  <input
                    type="number"
                    min="0"
                    {...form.register("automation.inactivityWarningHours" as const, { valueAsNumber: true })}
                    className="form-input w-24"
                  />
                  {form.formState.errors?.automation?.inactivityWarningHours && (
                    <p className="text-red-500 text-xs mt-1">
                      {form.formState.errors.automation.inactivityWarningHours.message as string}
                    </p>
                  )}
                </SettingsRow.Control>
              </SettingsRow>
            </SettingsGroup>
          </SurfaceCard>

          <div className="sticky bottom-4 z-10 p-4 bg-surface-card border border-surface-border rounded-lg shadow-lg flex justify-end">
            <Button type="submit" variant="primary" disabled={isSaving}>
              {isSaving ? "Saving..." : "Save Settings"}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
