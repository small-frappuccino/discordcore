import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import toast from "react-hot-toast";
import { useCurrentGuild } from "../../../context/GuildContext";
import { useTicketsConfig, useUpdateTicketsConfig } from "../../../api/hooks/useTickets";
import { TicketAutomationSettingsSchema } from "../../../api/domains/tickets";

const SettingsPageSchema = z.object({
  automation: TicketAutomationSettingsSchema,
  enabled: z.boolean(),
});

type SettingsPageValues = z.infer<typeof SettingsPageSchema>;

export function useTicketsSettingsLogic() {
  const { guildId: selectedGuildID } = useCurrentGuild();
  const { data: configResp, isLoading } = useTicketsConfig(selectedGuildID || "");
  const { mutate: updateConfig, isPending: isSaving } = useUpdateTicketsConfig(selectedGuildID || "");

  const form = useForm<SettingsPageValues>({
    resolver: zodResolver(SettingsPageSchema),
    defaultValues: {
      enabled: false,
      automation: {},
    },
  });

  useEffect(() => {
    if (configResp?.settings) {
      form.reset({
        enabled: configResp.settings.enabled ?? false,
        automation: configResp.settings.automation || {},
      });
    }
  }, [configResp, form]);

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID || !configResp?.settings) return;
    
    updateConfig(
      { ...configResp.settings, enabled: data.enabled, automation: data.automation },
      {
        onSuccess: () => toast.success("Automation settings saved successfully"),
        onError: (err) => toast.error(err.message || "Failed to save settings"),
      }
    );
  });

  return {
    selectedGuildID,
    isLoading,
    isSaving,
    form,
    onSubmit,
  };
}
