
import { PageHeader, SettingsGroup, SettingsRow, Badge, PageContainer, SettingsGroupSkeleton } from "../components/ui";
import { Stack } from "../components/layout";
import { useCorePageLogic } from "./hooks/useCorePageLogic";

export function CorePage() {
  const { settings, isLoading } = useCorePageLogic();
  return (
    <PageContainer>
      <Stack spacing="lg">
        <PageHeader>
          <PageHeader.TitleRow>
            <PageHeader.Title>Core Settings</PageHeader.Title>
            <Badge variant="success">Online</Badge>
          </PageHeader.TitleRow>
          <PageHeader.Description>Global operational parameters and domain routing overrides.</PageHeader.Description>
        </PageHeader>

        {isLoading ? (
          <SettingsGroupSkeleton rows={2} />
        ) : (
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
        )}
      </Stack>
    </PageContainer>
  );
}
