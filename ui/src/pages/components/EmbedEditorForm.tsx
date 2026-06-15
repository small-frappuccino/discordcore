import React, { useState } from "react";
import type { UseFormReturn } from "react-hook-form";
import { ActionTrigger, SettingsGroup, SettingsRow, TextInput, TextArea, SaveActionBar } from "../../components/ui/tahoe";
import { ConfirmationModal } from "../../components/ui";
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
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);

  if (!selectedEmbedKey && !activeEmbedDataKey) {
    return null;
  }

  return (
    <form onSubmit={onSubmit} className="settings-form">
      <SettingsGroup>
        <SettingsRow
          title="Embed Key"
          description="Unique identifier for this embed."
          control={<TextInput type="text" {...form.register("key")} disabled={!!selectedEmbedKey} />}
        />
        <SettingsRow
          title="Color"
          description="Hex color code (as an integer number)."
          control={<TextInput type="number" {...form.register("color", { valueAsNumber: true })} className="w-40" />}
        />
      </SettingsGroup>

      <SettingsGroup>
        <SettingsRow
          title="Title"
          control={<TextInput type="text" {...form.register("title")} />}
        />
        <SettingsRow
          isMultiline
          title="Description"
          control={<TextArea {...form.register("description")} className="w-full input-expansive" />}
        />
      </SettingsGroup>

      <SettingsGroup>
        <SettingsRow
          title="Author Name"
          control={<TextInput type="text" {...form.register("author_name")} />}
        />
        <SettingsRow
          title="Author Icon URL"
          control={<TextInput type="text" {...form.register("author_icon_url")} />}
        />
        <SettingsRow
          title="Footer Text"
          control={<TextInput type="text" {...form.register("footer_text")} />}
        />
        <SettingsRow
          title="Footer Icon URL"
          control={<TextInput type="text" {...form.register("footer_icon_url")} />}
        />
      </SettingsGroup>

      <SettingsGroup>
        <SettingsRow
          title="Image URL"
          control={<TextInput type="text" {...form.register("image_url")} />}
        />
        <SettingsRow
          title="Thumbnail URL"
          control={<TextInput type="text" {...form.register("thumbnail_url")} />}
        />
      </SettingsGroup>

        <div>
          {customFields.map((field, idx) => (
            <SettingsRow
              isMultiline
              key={field.id}
              title={`Custom Field ${idx + 1}`}
              description={
                <div className="mt-4 flex flex-col gap-4">
                  <label className="flex items-center gap-2 text-sm text-foreground">
                    <input type="checkbox" {...form.register(`fields.${idx}.inline` as const)} className="form-checkbox" />
                    Inline
                  </label>
                  <ActionTrigger variant="danger" onClick={() => removeField(idx)} className="px-2 py-1 text-xs self-start w-auto">
                    Remove Field
                  </ActionTrigger>
                </div>
              }
              control={
                <div className="flex flex-col gap-4 w-full">
                  <div className="flex flex-col gap-1">
                    <label className="text-xs text-muted font-medium">Name</label>
                    <TextInput type="text" {...form.register(`fields.${idx}.name` as const)} className="text-sm w-full" />
                  </div>
                  <div className="flex flex-col gap-1">
                    <label className="text-xs text-muted font-medium">Value</label>
                    <TextArea {...form.register(`fields.${idx}.value` as const)} className="text-sm w-full input-expansive min-h-[80px]" />
                  </div>
                </div>
              }
            />
          ))}
          <SettingsRow
            title=""
            control={
              <ActionTrigger variant="secondary" onClick={appendField} className="self-start">
                + Add Field
              </ActionTrigger>
            }
          />
        </div>

      <div className="form-actions mt-4">
        {selectedEmbedKey && (
          <ActionTrigger variant="danger" type="button" onClick={() => setIsDeleteModalOpen(true)} isLoading={isDeleting}>
            Delete Embed
          </ActionTrigger>
        )}
      </div>

      <ConfirmationModal
        isOpen={isDeleteModalOpen}
        onClose={() => setIsDeleteModalOpen(false)}
        title="Delete Embed"
        description={`Are you sure you want to delete the embed "${selectedEmbedKey}"? This action cannot be undone.`}
        confirmText="Delete"
        onConfirm={() => {
          setIsDeleteModalOpen(false);
          deleteEmbed();
        }}
      />

      <SaveActionBar
        isDirty={form.formState.isDirty}
        isSaving={isSaving}
        onSave={onSubmit}
        onReset={() => form.reset()}
      />
    </form>
  );
}
