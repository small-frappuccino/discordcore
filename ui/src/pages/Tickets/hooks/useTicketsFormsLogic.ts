import { useEffect } from "react";
import { useForm, useFieldArray } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import toast from "react-hot-toast";
import { useCurrentGuild } from "../../../context/GuildContext";
import { useTicketsConfig, useUpdateTicketsConfig } from "../../../api/hooks/useTickets";
import { TicketIntakeFormSchema } from "../../../api/domains/tickets";

const FormsPageSchema = z.object({
  forms: z.array(TicketIntakeFormSchema),
});

type FormsPageValues = z.infer<typeof FormsPageSchema>;

export function useTicketsFormsLogic() {
  const { guildId: selectedGuildID } = useCurrentGuild();
  const { data: configResp, isLoading } = useTicketsConfig(selectedGuildID || "");
  const { mutate: updateConfig, isPending: isSaving } = useUpdateTicketsConfig(selectedGuildID || "");

  const form = useForm<FormsPageValues>({
    // @ts-expect-error - Zod resolver types mismatch with RHF defaults
    resolver: zodResolver(FormsPageSchema),
    defaultValues: { forms: [] },
  });

  const { fields: intakeForms, append: addForm, remove: removeForm } = useFieldArray({
    control: form.control,
    name: "forms",
  });

  useEffect(() => {
    if (configResp?.settings) {
      form.reset({ forms: configResp.settings.forms || [] });
    }
  }, [configResp, form]);

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID || !configResp?.settings) return;
    
    updateConfig(
      { ...configResp.settings, forms: data.forms },
      {
        onSuccess: () => toast.success("Intake forms saved successfully"),
        onError: (err) => toast.error(err.message || "Failed to save forms"),
      }
    );
  });

  return {
    selectedGuildID,
    isLoading,
    isSaving,
    form,
    intakeForms,
    addForm,
    removeForm,
    onSubmit,
  };
}
