import { motion } from "framer-motion";
import { Button, SettingsGroupSkeleton, SurfaceCard } from "../../components/ui";
import { useTicketsFormsLogic } from "./hooks/useTicketsFormsLogic";
import { FormQuestionEditor } from "./components/FormQuestionEditor";

export function TicketsFormsPage() {
  const { isLoading, isSaving, form, intakeForms, addForm, removeForm, onSubmit } = useTicketsFormsLogic();

  return (
    <div>
      <div className="mb-6 flex justify-between items-center">
        <div>
          <h2 className="text-xl font-semibold">Intake Forms</h2>
          <p className="text-muted">Create dynamic pre-ticket questionnaires to collect information.</p>
        </div>
        <Button
          type="button"
          variant="primary"
          onClick={() =>
            addForm({
              id: crypto.randomUUID(),
              name: "New Form",
              questions: [],
            })
          }
        >
          + Add Form
        </Button>
      </div>

      {isLoading ? (
        <SettingsGroupSkeleton rows={4} />
      ) : (
        <form onSubmit={onSubmit} className="space-y-8">
          {intakeForms.map((intakeForm, index) => (
            <motion.div
              key={intakeForm.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95 }}
              transition={{ duration: 0.2 }}
            >
              <SurfaceCard className="p-6">
                <div className="flex justify-between items-center mb-6">
                  <h3 className="text-lg font-semibold tracking-tight">Form Configuration</h3>
                  <Button type="button" variant="danger" onClick={() => removeForm(index)}>
                    Delete Form
                  </Button>
                </div>

                <div className="mb-8">
                  <label className="block text-sm font-medium mb-1">Form Name</label>
                  <input
                    {...form.register(`forms.${index}.name` as const)}
                    className="form-input w-full md:w-1/2"
                    placeholder="e.g. Partnership Request Form"
                  />
                  {form.formState.errors?.forms?.[index]?.name && (
                    <p className="text-red-500 text-xs mt-1">
                      {form.formState.errors.forms[index].name.message as string}
                    </p>
                  )}
                  <p className="text-sm text-muted mt-2">
                    Copy the ID below to bind this form to a panel category:
                  </p>
                  <code className="text-xs bg-surface-base px-2 py-1 rounded mt-1 inline-block border border-surface-border select-all">
                    {intakeForm.id}
                  </code>
                </div>

                <div className="border-t border-surface-border pt-6">
                  <h4 className="text-md font-semibold mb-2">Questions</h4>
                  <p className="text-sm text-muted mb-4">
                    Define the questions users must answer before the ticket channel opens.
                  </p>
                  <FormQuestionEditor formIndex={index} form={form} />
                  {form.formState.errors?.forms?.[index]?.questions?.message && (
                    <p className="text-red-500 text-sm mt-2">
                      {form.formState.errors.forms[index].questions.message as string}
                    </p>
                  )}
                </div>
              </SurfaceCard>
            </motion.div>
          ))}

          {intakeForms.length > 0 && (
            <div className="sticky bottom-4 z-10 p-4 bg-surface-card border border-surface-border rounded-lg shadow-lg flex justify-end">
              <Button type="submit" variant="primary" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Forms"}
              </Button>
            </div>
          )}

          {intakeForms.length === 0 && (
            <div className="text-center p-12 border-2 border-dashed border-surface-border rounded-xl">
              <h3 className="text-lg font-medium text-foreground mb-2">No forms configured</h3>
              <p className="text-muted mb-4">Get started by creating your first intake form.</p>
            </div>
          )}
        </form>
      )}
    </div>
  );
}
