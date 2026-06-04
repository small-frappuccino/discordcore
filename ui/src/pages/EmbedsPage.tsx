import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, EmbedPreview } from "../components/ui";
import { useEmbedsPageLogic } from "./hooks/useEmbedsPageLogic";

export function EmbedsPage() {
  const {
    embeds,
    isLoading,
    selectedEmbedKey,
    selectEmbed,
    createNewEmbed,
    form,
    customFields,
    appendField,
    removeField,
    onSubmit,
    isSaving,
    isDeleting,
    deleteEmbed
  } = useEmbedsPageLogic();

  const activeEmbedData = form.watch();

  return (
    <div className="flex flex-col h-full">
      <PageHeader 
        title="Custom Embeds" 
        description="Design and manage custom embeds for your server."
        badge={<Badge variant="success">Active</Badge>}
      />

      <div className="mt-8 flex gap-8 h-full min-h-[600px] items-start">
        {/* Left Pane: List & Editor */}
        <div className="flex-1 flex flex-col gap-6">
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center">
              <h2 className="text-lg font-semibold text-white">Saved Embeds</h2>
              <Button variant="primary" onClick={createNewEmbed} className="text-sm px-3 py-1">
                + New Embed
              </Button>
            </div>
            
            {isLoading ? (
              <p className="text-muted">Loading Embeds...</p>
            ) : embeds.length === 0 ? (
              <div className="text-muted text-sm italic py-4">No custom embeds found. Create one above!</div>
            ) : (
              <div className="flex flex-wrap gap-2">
                {embeds.map(emb => (
                  <button
                    key={emb.key}
                    type="button"
                    onClick={() => selectEmbed(emb)}
                    className={`px-4 py-2 rounded-md border text-sm font-medium transition-colors ${
                      selectedEmbedKey === emb.key 
                        ? 'border-brand-500 bg-brand-500/10 text-white' 
                        : 'border-white/10 bg-white/5 text-muted hover:bg-white/10 hover:text-white'
                    }`}
                  >
                    {emb.key}
                  </button>
                ))}
              </div>
            )}
          </div>

          {(selectedEmbedKey || activeEmbedData.key) && (
            <form onSubmit={onSubmit} className="flex flex-col gap-6">
              <SettingsGroup title="General Settings">
                <SettingsRow 
                  title="Embed Key"
                  description="Unique identifier for this embed."
                  control={
                    <input
                      type="text"
                      {...form.register("key")}
                      className="form-input w-[250px]"
                      disabled={!!selectedEmbedKey} // Cannot edit key after creation
                    />
                  }
                />
                <SettingsRow 
                  title="Color"
                  description="Hex color code (as an integer number)."
                  isLast
                  control={
                    <input
                      type="number"
                      {...form.register("color", { valueAsNumber: true })}
                      className="form-input w-[150px]"
                    />
                  }
                />
              </SettingsGroup>

              <SettingsGroup title="Content">
                <SettingsRow 
                  title="Title"
                  control={<input type="text" {...form.register("title")} className="form-input w-full" />}
                />
                <SettingsRow 
                  title="Description"
                  isLast
                  control={<textarea {...form.register("description")} className="form-input w-full h-24 resize-y" />}
                />
              </SettingsGroup>

              <SettingsGroup title="Author & Footer">
                <SettingsRow 
                  title="Author Name"
                  control={<input type="text" {...form.register("author_name")} className="form-input w-full" />}
                />
                <SettingsRow 
                  title="Author Icon URL"
                  control={<input type="text" {...form.register("author_icon_url")} className="form-input w-full" />}
                />
                <SettingsRow 
                  title="Footer Text"
                  control={<input type="text" {...form.register("footer_text")} className="form-input w-full" />}
                />
                <SettingsRow 
                  title="Footer Icon URL"
                  isLast
                  control={<input type="text" {...form.register("footer_icon_url")} className="form-input w-full" />}
                />
              </SettingsGroup>

              <SettingsGroup title="Images">
                <SettingsRow 
                  title="Image URL"
                  control={<input type="text" {...form.register("image_url")} className="form-input w-full" />}
                />
                <SettingsRow 
                  title="Thumbnail URL"
                  isLast
                  control={<input type="text" {...form.register("thumbnail_url")} className="form-input w-full" />}
                />
              </SettingsGroup>

              <SettingsGroup title="Fields">
                <div className="p-4 flex flex-col gap-4">
                  {customFields.map((field, idx) => (
                    <div key={field.id} className="flex flex-col gap-2 p-3 border border-white/10 rounded bg-white/5 relative">
                      <div className="flex gap-4">
                        <div className="flex-1 flex flex-col gap-1">
                          <label className="text-xs text-muted font-medium">Name</label>
                          <input type="text" {...form.register(`fields.${idx}.name` as const)} className="form-input text-sm" />
                        </div>
                        <div className="flex-1 flex flex-col gap-1">
                          <label className="text-xs text-muted font-medium">Value</label>
                          <input type="text" {...form.register(`fields.${idx}.value` as const)} className="form-input text-sm" />
                        </div>
                      </div>
                      <div className="flex justify-between items-center mt-2">
                        <label className="flex items-center gap-2 text-sm text-white">
                          <input type="checkbox" {...form.register(`fields.${idx}.inline` as const)} className="form-checkbox" />
                          Inline
                        </label>
                        <Button type="button" variant="danger" onClick={() => removeField(idx)} className="px-2 py-1 text-xs">
                          Remove Field
                        </Button>
                      </div>
                    </div>
                  ))}
                  <Button type="button" variant="secondary" onClick={appendField} className="self-start">
                    + Add Field
                  </Button>
                </div>
              </SettingsGroup>

              <div className="flex gap-4 mt-2">
                <Button variant="primary" type="submit" disabled={isSaving}>
                  {isSaving ? "Saving..." : "Save Embed"}
                </Button>
                {selectedEmbedKey && (
                  <Button type="button" variant="danger" disabled={isDeleting} onClick={deleteEmbed}>
                    {isDeleting ? "Deleting..." : "Delete Embed"}
                  </Button>
                )}
              </div>
            </form>
          )}
        </div>

        {/* Right Pane: Live Preview */}
        <div className="w-[520px] shrink-0 sticky top-8">
          <h2 className="text-lg font-semibold text-white mb-4">Live Preview</h2>
          <div className="p-4 bg-[#36393f] rounded-lg border border-black/20 shadow-xl overflow-hidden">
            <EmbedPreview embed={activeEmbedData} />
          </div>
        </div>
      </div>
    </div>
  );
}
