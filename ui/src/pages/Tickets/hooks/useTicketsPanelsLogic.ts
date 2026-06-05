import { useEffect } from "react";
import { useForm, useFieldArray } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import toast from "react-hot-toast";
import { useCurrentGuild } from "../../../context/GuildContext";
import { useTicketsConfig, useUpdateTicketsConfig } from "../../../api/hooks/useTickets";
import { TicketPanelSchema } from "../../../api/domains/tickets";

const PanelsFormSchema = z.object({
  panels: z.array(TicketPanelSchema),
});

type PanelsFormValues = z.infer<typeof PanelsFormSchema>;

export function useTicketsPanelsLogic() {
  const { guildId: selectedGuildID } = useCurrentGuild();
  const { data: configResp, isLoading } = useTicketsConfig(selectedGuildID || "");
  const { mutate: updateConfig, isPending: isSaving } = useUpdateTicketsConfig(selectedGuildID || "");

  const form = useForm<PanelsFormValues>({
    resolver: zodResolver(PanelsFormSchema),
    defaultValues: { panels: [] },
  });

  const { fields: panels, append: addPanel, remove: removePanel } = useFieldArray({
    control: form.control,
    name: "panels",
  });

  useEffect(() => {
    if (configResp?.settings) {
      form.reset({ panels: configResp.settings.panels || [] });
    }
  }, [configResp, form]);

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID || !configResp?.settings) return;
    
    updateConfig(
      { ...configResp.settings, panels: data.panels },
      {
        onSuccess: () => toast.success("Panels saved successfully"),
        onError: (err) => toast.error(err.message || "Failed to save panels"),
      }
    );
  });

  return {
    selectedGuildID,
    isLoading,
    isSaving,
    form,
    panels,
    addPanel,
    removePanel,
    onSubmit,
  };
}
