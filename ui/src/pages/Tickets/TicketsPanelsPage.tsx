import { Button, SettingsGroupSkeleton, SurfaceCard, SettingsGroup, SettingsRow } from "../../components/ui";
import { useTicketsPanelsLogic } from "./hooks/useTicketsPanelsLogic";
import { PanelCategoryEditor } from "./components/PanelCategoryEditor";

export function TicketsPanelsPage() {
  const { isLoading, isSaving, form, panels, addPanel, removePanel, onSubmit } = useTicketsPanelsLogic();

  return (
    <div>
      <div className="mb-6 flex justify-between items-center">
        <div>
          <h2 className="text-xl font-semibold">Ticket Panels</h2>
          <p className="text-muted">Create trigger panels where users can open tickets.</p>
        </div>
        <Button
          type="button"
          variant="primary"
          onClick={() =>
            addPanel({
              id: crypto.randomUUID(),
              name: "New Panel",
              channelId: "",
              embedTitle: "Support Ticket",
              embedDescription: "Please click the button below to open a ticket.",
              embedColor: "#5865F2",
              categories: [],
            })
          }
        >
          + Add Panel
        </Button>
      </div>

      {isLoading ? (
        <SettingsGroupSkeleton rows={4} />
      ) : (
        <form onSubmit={onSubmit} className="space-y-8">
          {panels.map((panel, index) => (
            <div
              key={panel.id}
              className="animate-in fade-in slide-in-from-top-2 duration-300 ease-out"
            >
              <SurfaceCard className="p-6">
                <div className="flex justify-between items-center mb-6">
                  <h3 className="text-lg font-semibold tracking-tight">Panel Configuration</h3>
                  <Button type="button" variant="danger" onClick={() => removePanel(index)}>
                    Delete Panel
                  </Button>
                </div>

                <SettingsGroup className="mb-8">
                  <SettingsRow>
                    <SettingsRow.Info>
                      <SettingsRow.Title>Panel Name</SettingsRow.Title>
                      {form.formState.errors?.panels?.[index]?.name && (
                        <SettingsRow.Description className="text-danger">
                          {form.formState.errors.panels[index].name.message as string}
                        </SettingsRow.Description>
                      )}
                    </SettingsRow.Info>
                    <SettingsRow.Control>
                      <input
                        {...form.register(`panels.${index}.name` as const)}
                        className="form-input w-full"
                        placeholder="e.g. Main Support Panel"
                      />
                    </SettingsRow.Control>
                  </SettingsRow>
                  <SettingsRow>
                    <SettingsRow.Info>
                      <SettingsRow.Title>Target Channel ID</SettingsRow.Title>
                      {form.formState.errors?.panels?.[index]?.channelId && (
                        <SettingsRow.Description className="text-danger">
                          {form.formState.errors.panels[index].channelId.message as string}
                        </SettingsRow.Description>
                      )}
                    </SettingsRow.Info>
                    <SettingsRow.Control>
                      <input
                        {...form.register(`panels.${index}.channelId` as const)}
                        className="form-input w-full"
                        placeholder="Where should this panel be posted?"
                      />
                    </SettingsRow.Control>
                  </SettingsRow>
                </SettingsGroup>

                <h4 className="text-md font-semibold mb-4 text-muted">Embed Appearance</h4>
                <SettingsGroup className="mb-8 p-4 bg-surface-base border border-surface-border rounded-lg">
                  <SettingsRow>
                    <SettingsRow.Info>
                      <SettingsRow.Title>Embed Title</SettingsRow.Title>
                    </SettingsRow.Info>
                    <SettingsRow.Control>
                      <input
                        {...form.register(`panels.${index}.embedTitle` as const)}
                        className="form-input w-full"
                      />
                    </SettingsRow.Control>
                  </SettingsRow>
                  <SettingsRow className="settings-row--multiline">
                    <SettingsRow.Info>
                      <SettingsRow.Title>Embed Description</SettingsRow.Title>
                    </SettingsRow.Info>
                    <SettingsRow.Control>
                      <textarea
                        {...form.register(`panels.${index}.embedDescription` as const)}
                        className="form-input w-full min-h-[100px]"
                      />
                    </SettingsRow.Control>
                  </SettingsRow>
                  <SettingsRow>
                    <SettingsRow.Info>
                      <SettingsRow.Title>Embed Color (Hex)</SettingsRow.Title>
                    </SettingsRow.Info>
                    <SettingsRow.Control>
                      <div className="flex items-center gap-3 w-full">
                        <input
                          type="color"
                          {...form.register(`panels.${index}.embedColor` as const)}
                          className="h-9 w-12 rounded border border-surface-border bg-transparent p-1 cursor-pointer shrink-0"
                        />
                        <input
                          type="text"
                          {...form.register(`panels.${index}.embedColor` as const)}
                          className="form-input flex-1"
                          placeholder="#FFFFFF"
                        />
                      </div>
                    </SettingsRow.Control>
                  </SettingsRow>
                </SettingsGroup>

                <div className="mt-8 border-t border-surface-border pt-6">
                  <h4 className="text-md font-semibold mb-2">Dropdown Categories</h4>
                  <p className="text-sm text-muted mb-4">
                    Map user selections to specific backend configurations and staff roles.
                  </p>
                  <PanelCategoryEditor nestIndex={index} form={form} />
                  {form.formState.errors?.panels?.[index]?.categories?.message && (
                    <p className="text-red-500 text-sm mt-2">
                      {form.formState.errors.panels[index].categories.message as string}
                    </p>
                  )}
                </div>
              </SurfaceCard>
            </div>
          ))}

          {panels.length > 0 && (
            <div className="form-actions sticky bottom-4 z-10 p-4 bg-surface-card border border-surface-border rounded-lg shadow-lg">
              <Button type="submit" variant="primary" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Configuration"}
              </Button>
            </div>
          )}

          {panels.length === 0 && (
            <div className="empty-state">
              <div>
                <h3 className="text-lg font-medium text-foreground mb-2">No panels configured</h3>
                <p className="text-muted mb-4">Get started by creating your first ticket trigger panel.</p>
              </div>
            </div>
          )}
        </form>
      )}
    </div>
  );
}
