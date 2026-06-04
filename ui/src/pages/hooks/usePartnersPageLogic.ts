import { useEffect, useState } from "react";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import type { PartnerBoardTemplateConfig } from "../../api/control";
import { usePartnerBoardQuery, useSetPartnerBoardTemplateMutation } from "../../api/hooks/usePartners";

export function usePartnersPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: boardRes, isLoading } = usePartnerBoardQuery(client, selectedGuildID);
  const updateMutation = useSetPartnerBoardTemplateMutation(client, selectedGuildID);

  const [template, setTemplate] = useState<PartnerBoardTemplateConfig>({});

  useEffect(() => {
    if (boardRes?.partner_board?.template) {
      setTemplate(boardRes.partner_board.template);
    }
  }, [boardRes]);

  const handleSave = () => {
    if (!selectedGuildID) return;
    updateMutation.mutate(template, {
      onSuccess: () => alert("Template saved successfully."),
      onError: () => alert("Failed to save template.")
    });
  };

  const updateField = (field: keyof PartnerBoardTemplateConfig, value: string) => {
    setTemplate((prev) => ({ ...prev, [field]: value }));
  };

  return {
    selectedGuildID,
    isLoading,
    template,
    updateField,
    handleSave,
  };
}
