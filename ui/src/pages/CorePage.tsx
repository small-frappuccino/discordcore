import { useState, useEffect } from "react";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { PageHeader, SurfaceCard, SettingsGroup, SettingsRow, Badge } from "../components/ui";
import type { GuildSettingsWorkspaceResponse } from "../api/control";

export function CorePage() {
  const { client, selectedGuildID } = useDashboardSession();
  const [loading, setLoading] = useState(true);
  const [settings, setSettings] = useState<GuildSettingsWorkspaceResponse | null>(null);

  useEffect(() => {
    if (!selectedGuildID) return;
    client.getGuildSettings(selectedGuildID).then((res) => {
      setSettings(res);
      setLoading(false);
    });
  }, [client, selectedGuildID]);

  if (loading) return null;

  return (
    <div>
      <PageHeader
        title="Core Settings"
        description="Global operational parameters and domain routing overrides."
        badge={<Badge variant="success">Online</Badge>}
      />

      <SurfaceCard className="mt-8">
        <h3 className="mb-4 text-lg">Domain Routing</h3>
        <SettingsGroup>
          <SettingsRow
            title="Default Bot Instance"
            description="The fallback worker instance for this server."
            control={<span className="text-muted">{settings?.workspace?.sections?.bot_routing?.bot_instance_id || "Main Worker"}</span>}
          />
          <SettingsRow
            title="QOTD Domain Override"
            description="Specific worker assigned to QOTD processing."
            control={<span className="text-muted">{settings?.workspace?.sections?.bot_routing?.domain_bot_instance_ids?.qotd || "Inherited"}</span>}
            isLast
          />
        </SettingsGroup>
      </SurfaceCard>
    </div>
  );
}
