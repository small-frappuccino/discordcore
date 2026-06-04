import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import toast from "react-hot-toast";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { usePartnerBoardQuery, useSetPartnerBoardTemplateMutation } from "../../api/hooks/usePartners";
import { PartnersSchema, type PartnersFormData } from "../schemas/partners";

export function usePartnersPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: boardRes, isLoading } = usePartnerBoardQuery(client, selectedGuildID);
  const updateMutation = useSetPartnerBoardTemplateMutation(client, selectedGuildID);

  const form = useForm<PartnersFormData>({
    resolver: zodResolver(PartnersSchema),
    defaultValues: {
      title: "",
      continuation_title: "",
      intro: "",
      section_header_template: "",
      section_continuation_suffix: "",
      section_continuation_template: "",
      line_template: "",
      empty_state_text: "",
      footer_template: "",
      other_fandom_label: "",
      color: 0,
      disable_fandom_sorting: false,
      disable_partner_sorting: false,
    }
  });

  useEffect(() => {
    if (boardRes?.partner_board?.template) {
      form.reset(boardRes.partner_board.template);
    }
  }, [boardRes, form]);

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID) return;
    updateMutation.mutate(data, {
      onSuccess: () => toast.success("Template saved successfully."),
      onError: (e) => toast.error(`Failed to save template: ${formatError(e)}`)
    });
  });

  const isSaving = updateMutation.isPending;

  return {
    selectedGuildID,
    isLoading,
    isSaving,
    form,
    onSubmit,
  };
}
