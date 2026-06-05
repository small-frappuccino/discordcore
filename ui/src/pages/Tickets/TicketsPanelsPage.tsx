import { motion } from "framer-motion";
import { Button, SettingsGroupSkeleton, SurfaceCard } from "../../components/ui";
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
            <motion.div
              key={panel.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95 }}
              transition={{ duration: 0.2 }}
            >
              <SurfaceCard className="p-6">
                <div className="flex justify-between items-center mb-6">
                  <h3 className="text-lg font-semibold tracking-tight">Panel Configuration</h3>
                  <Button type="button" variant="danger" onClick={() => removePanel(index)}>
                    Delete Panel
                  </Button>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
                  <div>
                    <label className="block text-sm font-medium mb-1">Panel Name</label>
                    <input
                      {...form.register(`panels.${index}.name` as const)}
                      className="form-input w-full"
                      placeholder="e.g. Main Support Panel"
                    />
                    {form.formState.errors?.panels?.[index]?.name && (
                      <p className="text-red-500 text-xs mt-1">
                        {form.formState.errors.panels[index].name.message as string}
                      </p>
                    )}
                  </div>
                  <div>
                    <label className="block text-sm font-medium mb-1">Target Channel ID</label>
                    <input
                      {...form.register(`panels.${index}.channelId` as const)}
                      className="form-input w-full"
                      placeholder="Where should this panel be posted?"
                    />
                    {form.formState.errors?.panels?.[index]?.channelId && (
                      <p className="text-red-500 text-xs mt-1">
                        {form.formState.errors.panels[index].channelId.message as string}
                      </p>
                    )}
                  </div>
                </div>

                <h4 className="text-md font-semibold mb-4 text-muted">Embed Appearance</h4>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8 p-4 bg-surface-base border border-surface-border rounded-lg">
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium mb-1">Embed Title</label>
                    <input
                      {...form.register(`panels.${index}.embedTitle` as const)}
                      className="form-input w-full"
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium mb-1">Embed Description</label>
                    <textarea
                      {...form.register(`panels.${index}.embedDescription` as const)}
                      className="form-input w-full min-h-[100px]"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium mb-1">Embed Color (Hex)</label>
                    <div className="flex items-center gap-3">
                      <input
                        type="color"
                        {...form.register(`panels.${index}.embedColor` as const)}
                        className="h-9 w-12 rounded border border-surface-border bg-transparent p-1 cursor-pointer"
                      />
                      <input
                        type="text"
                        {...form.register(`panels.${index}.embedColor` as const)}
                        className="form-input flex-1"
                        placeholder="#FFFFFF"
                      />
                    </div>
                  </div>
                </div>

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
            </motion.div>
          ))}

          {panels.length > 0 && (
            <div className="sticky bottom-4 z-10 p-4 bg-surface-card border border-surface-border rounded-lg shadow-lg flex justify-end">
              <Button type="submit" variant="primary" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Configuration"}
              </Button>
            </div>
          )}

          {panels.length === 0 && (
            <div className="text-center p-12 border-2 border-dashed border-surface-border rounded-xl">
              <h3 className="text-lg font-medium text-foreground mb-2">No panels configured</h3>
              <p className="text-muted mb-4">Get started by creating your first ticket trigger panel.</p>
            </div>
          )}
        </form>
      )}
    </div>
  );
}
