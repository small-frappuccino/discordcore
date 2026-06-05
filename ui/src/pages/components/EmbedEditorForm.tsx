import * as React from "react";
import type { UseFormReturn } from "react-hook-form";
import { SettingsGroup, SettingsRow, Button, FormControl } from "../../components/ui";
import type { EmbedsFormData } from "../schemas/embeds";

type EmbedEditorFormProps = {
  form: UseFormReturn<EmbedsFormData>;
  customFields: Record<"id", string>[];
  appendField: () => void;
  removeField: (idx: number) => void;
  onSubmit: (e?: React.BaseSyntheticEvent) => Promise<void>;
  isSaving: boolean;
  isDeleting: boolean;
  deleteEmbed: () => void;
  selectedEmbedKey: string | null;
  activeEmbedDataKey: string | undefined;
};

export function EmbedEditorForm({
  form,
  customFields,
  appendField,
  removeField,
  onSubmit,
  isSaving,
  isDeleting,
  deleteEmbed,
  selectedEmbedKey,
  activeEmbedDataKey,
}: EmbedEditorFormProps) {
  if (!selectedEmbedKey && !activeEmbedDataKey) {
    return null;
  }

  return (
    <form onSubmit={onSubmit} className="settings-form">
      <SettingsGroup>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Embed Key</SettingsRow.Title>
            <SettingsRow.Description>Unique identifier for this embed.</SettingsRow.Description>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input
                type="text"
                {...form.register("key")}
                className="form-input"
                disabled={!!selectedEmbedKey} // Cannot edit key after creation
              />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Color</SettingsRow.Title>
            <SettingsRow.Description>Hex color code (as an integer number).</SettingsRow.Description>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <input
              type="number"
              {...form.register("color", { valueAsNumber: true })}
              className="form-input w-40"
            />
          </SettingsRow.Control>
        </SettingsRow>
      </SettingsGroup>

      <SettingsGroup>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Title</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input type="text" {...form.register("title")} className="form-input" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Description</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <textarea {...form.register("description")} className="form-input w-full resize-y input-expansive" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
      </SettingsGroup>

      <SettingsGroup>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Author Name</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input type="text" {...form.register("author_name")} className="form-input" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Author Icon URL</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input type="text" {...form.register("author_icon_url")} className="form-input" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Footer Text</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input type="text" {...form.register("footer_text")} className="form-input" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Footer Icon URL</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input type="text" {...form.register("footer_icon_url")} className="form-input" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
      </SettingsGroup>

      <SettingsGroup>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Image URL</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input type="text" {...form.register("image_url")} className="form-input" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
        <SettingsRow>
          <SettingsRow.Info>
            <SettingsRow.Title>Thumbnail URL</SettingsRow.Title>
          </SettingsRow.Info>
          <SettingsRow.Control>
            <FormControl asChild>
              <input type="text" {...form.register("thumbnail_url")} className="form-input" />
            </FormControl>
          </SettingsRow.Control>
        </SettingsRow>
      </SettingsGroup>

        <div>
          {customFields.map((field, idx) => (
            <SettingsRow key={field.id}>
              <SettingsRow.Info>
                <SettingsRow.Title>Custom Field {idx + 1}</SettingsRow.Title>
                <div className="mt-4 flex flex-col gap-4">
                  <label className="flex items-center gap-2 text-sm text-foreground">
                    <input type="checkbox" {...form.register(`fields.${idx}.inline` as const)} className="form-checkbox" />
                    Inline
                  </label>
                  <Button type="button" variant="danger" onClick={() => removeField(idx)} className="px-2 py-1 text-xs self-start">
                    Remove Field
                  </Button>
                </div>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <div className="flex flex-col gap-4 w-full">
                  <div className="flex flex-col gap-1">
                    <label className="text-xs text-muted font-medium">Name</label>
                    <input type="text" {...form.register(`fields.${idx}.name` as const)} className="form-input text-sm w-full" />
                  </div>
                  <div className="flex flex-col gap-1">
                    <label className="text-xs text-muted font-medium">Value</label>
                    <textarea {...form.register(`fields.${idx}.value` as const)} className="form-input text-sm w-full input-expansive min-h-[80px]" />
                  </div>
                </div>
              </SettingsRow.Control>
            </SettingsRow>
          ))}
          <SettingsRow>
            <SettingsRow.Info></SettingsRow.Info>
            <SettingsRow.Control>
              <Button type="button" variant="secondary" onClick={appendField} className="self-start">
                + Add Field
              </Button>
            </SettingsRow.Control>
          </SettingsRow>
        </div>

      <div className="form-actions">
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
  );
}
