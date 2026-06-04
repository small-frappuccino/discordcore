
import { PageHeader, SurfaceCard, SettingsGroup, SettingsRow, Badge, PageContainer, SettingsGroupSkeleton } from "../components/ui";
import { useCorePageLogic } from "./hooks/useCorePageLogic";

export function CorePage() {
  const { settings, isLoading } = useCorePageLogic();
  return (
    <PageContainer>
      <PageHeader
        title="Core Settings"
        description="Global operational parameters and domain routing overrides."
        badge={<Badge variant="success">Online</Badge>}
      />

      {isLoading ? (
        <div className="mt-8">
          <SettingsGroupSkeleton rows={2} />
        </div>
      ) : (
        <SurfaceCard className="mt-8">
        <h3 className="mb-4 text-lg">Domain Routing</h3>
        <SettingsGroup>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>Default Bot Instance</SettingsRow.Title>
              <SettingsRow.Description>The fallback worker instance for this server.</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              <span className="text-muted">{settings?.workspace?.sections?.bot_routing?.bot_instance_id || "Main Worker"}</span>
            </SettingsRow.Control>
          </SettingsRow>
          <SettingsRow>
            <SettingsRow.Info>
              <SettingsRow.Title>QOTD Domain Override</SettingsRow.Title>
              <SettingsRow.Description>Specific worker assigned to QOTD processing.</SettingsRow.Description>
            </SettingsRow.Info>
            <SettingsRow.Control>
              <span className="text-muted">{settings?.workspace?.sections?.bot_routing?.domain_bot_instance_ids?.qotd || "Inherited"}</span>
            </SettingsRow.Control>
          </SettingsRow>
        </SettingsGroup>
        </SurfaceCard>
      )}
    </PageContainer>
  );
}
