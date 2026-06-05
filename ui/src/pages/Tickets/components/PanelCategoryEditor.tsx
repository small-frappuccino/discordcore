import { useFieldArray, type UseFormReturn } from "react-hook-form";
import { Button } from "../../../components/ui";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function PanelCategoryEditor({ nestIndex, form }: { nestIndex: number; form: UseFormReturn<any> }) {
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: `panels.${nestIndex}.categories`,
  });

  return (
    <div className="space-y-4">
      {fields.map((field, k) => (
        <div key={field.id} className="p-4 border border-surface-border rounded-md relative bg-surface-base">
          <div className="absolute top-2 right-2">
            <Button variant="danger" type="button" onClick={() => remove(k)} className="text-xs px-2 py-1">
              Remove
            </Button>
          </div>
          <h4 className="font-semibold mb-2">Category {k + 1}</h4>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium mb-1">Name</label>
              <input
                {...form.register(`panels.${nestIndex}.categories.${k}.name` as const)}
                className="form-input w-full"
                placeholder="e.g. Server Support"
              />
              {(form.formState.errors as any)?.panels?.[nestIndex]?.categories?.[k]?.name && (
                <p className="text-red-500 text-xs mt-1">
                  {(form.formState.errors as any).panels[nestIndex].categories[k].name.message as string}
                </p>
              )}
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Emoji (optional)</label>
              <input
                {...form.register(`panels.${nestIndex}.categories.${k}.emoji` as const)}
                className="form-input w-full"
                placeholder="e.g. 🎟️"
              />
            </div>
            <div className="md:col-span-2">
              <label className="block text-sm font-medium mb-1">Description</label>
              <input
                {...form.register(`panels.${nestIndex}.categories.${k}.description` as const)}
                className="form-input w-full"
                placeholder="Brief description in dropdown"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Discord Category ID</label>
              <input
                {...form.register(`panels.${nestIndex}.categories.${k}.discordCategoryId` as const)}
                className="form-input w-full"
                placeholder="18-digit ID"
              />
              {(form.formState.errors as any)?.panels?.[nestIndex]?.categories?.[k]?.discordCategoryId && (
                <p className="text-red-500 text-xs mt-1">
                  {(form.formState.errors as any).panels[nestIndex].categories[k].discordCategoryId.message as string}
                </p>
              )}
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Staff Role IDs (comma-separated)</label>
              <input
                {...form.register(`panels.${nestIndex}.categories.${k}.staffRoleIds` as const, {
                  setValueAs: (val: string) => (val ? val.split(",").map((s) => s.trim()).filter(Boolean) : []),
                })}
                className="form-input w-full"
                placeholder="Role ID 1, Role ID 2..."
              />
              {(form.formState.errors as any)?.panels?.[nestIndex]?.categories?.[k]?.staffRoleIds && (
                <p className="text-red-500 text-xs mt-1">
                  Invalid role ID provided.
                </p>
              )}
            </div>
          </div>
        </div>
      ))}
      <Button
        type="button"
        variant="secondary"
        onClick={() =>
          append({
            id: crypto.randomUUID(),
            name: "",
            description: "",
            discordCategoryId: "",
            staffRoleIds: [],
          })
        }
      >
        + Add Category
      </Button>
    </div>
  );
}
