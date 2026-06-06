
import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton } from "../components/ui";
import { SettingsGroup, SettingsRow } from "../components/ui/tahoe";
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
            <div className="settings-form">
              <h3 className="text-lg font-semibold tracking-tight text-text-primary mb-4">Domain Routing</h3>
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
                />
              </SettingsGroup>
            </div>
            </Stack>
        )}
      </Stack>
    </PageContainer>
  );
}
