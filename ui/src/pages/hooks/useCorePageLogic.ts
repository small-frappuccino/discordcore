import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useGuildSettingsQuery } from "../../api/hooks/useGuildSettings";

export function useCorePageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: settings, isLoading } = useGuildSettingsQuery(client, selectedGuildID);

  return {
    settings,
    isLoading,
  };
}
