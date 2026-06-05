import { useFieldArray, type UseFormReturn } from "react-hook-form";
import { Button } from "../../../components/ui";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function FormQuestionEditor({ formIndex, form }: { formIndex: number; form: UseFormReturn<any> }) {
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: `forms.${formIndex}.questions`,
  });

  return (
    <div className="space-y-4">
      {fields.map((field, k) => (
        <div key={field.id} className="p-4 border border-surface-border rounded-md relative bg-surface-base">
          <div className="absolute top-2 right-2 flex gap-2">
            <Button variant="danger" type="button" onClick={() => remove(k)} className="text-xs px-2 py-1">
              Remove
            </Button>
          </div>
          <h4 className="font-semibold mb-2 text-sm text-muted">Question {k + 1}</h4>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="md:col-span-2">
              <label className="block text-sm font-medium mb-1">Question Title *</label>
              <input
                {...form.register(`forms.${formIndex}.questions.${k}.title` as const)}
                className="form-input w-full"
                placeholder="What is your Minecraft username?"
              />
              {(form.formState.errors as any)?.forms?.[formIndex]?.questions?.[k]?.title && (
                <p className="text-red-500 text-xs mt-1">
                  {(form.formState.errors as any).forms[formIndex].questions[k].title.message as string}
                </p>
              )}
            </div>
            
            <div className="md:col-span-2">
              <label className="block text-sm font-medium mb-1">Placeholder</label>
              <input
                {...form.register(`forms.${formIndex}.questions.${k}.placeholder` as const)}
                className="form-input w-full"
                placeholder="e.g. Notch..."
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-1">Min Length</label>
              <input
                type="number"
                {...form.register(`forms.${formIndex}.questions.${k}.minLength` as const, { valueAsNumber: true })}
                className="form-input w-full"
                placeholder="0"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-1">Max Length</label>
              <input
                type="number"
                {...form.register(`forms.${formIndex}.questions.${k}.maxLength` as const, { valueAsNumber: true })}
                className="form-input w-full"
                placeholder="1000"
              />
            </div>

            <div className="flex items-center gap-2 mt-2">
              <input
                type="checkbox"
                {...form.register(`forms.${formIndex}.questions.${k}.required` as const)}
                className="form-checkbox w-4 h-4"
              />
              <label className="text-sm font-medium">Required field</label>
            </div>

            <div className="flex items-center gap-2 mt-2">
              <input
                type="checkbox"
                {...form.register(`forms.${formIndex}.questions.${k}.multiline` as const)}
                className="form-checkbox w-4 h-4"
              />
              <label className="text-sm font-medium">Multi-line (Paragraph)</label>
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
            title: "",
            placeholder: "",
            required: true,
            multiline: false,
          })
        }
      >
        + Add Question
      </Button>
    </div>
  );
}
