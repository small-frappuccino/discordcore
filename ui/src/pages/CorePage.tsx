
import { PageHeader, SurfaceCard, SettingsGroup, SettingsRow, Badge, PageContainer, SettingsGroupSkeleton } from "../components/ui";
import { Stack } from "../components/layout";
import { useCorePageLogic } from "./hooks/useCorePageLogic";

export function CorePage() {
  const { settings, isLoading } = useCorePageLogic();
  return (
    <PageContainer>
      <Stack spacing="lg">
        <PageHeader
          title="Core Settings"
          description="Global operational parameters and domain routing overrides."
          badge={<Badge variant="success">Online</Badge>}
        />

        {isLoading ? (
          <SettingsGroupSkeleton rows={2} />
        ) : (
          <SurfaceCard interactive>
            <Stack spacing="sm">
              <h3 className="text-lg font-semibold tracking-tight text-text-primary">Domain Routing</h3>
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
            </Stack>
          </SurfaceCard>
        )}
      </Stack>
    </PageContainer>
  );
}
