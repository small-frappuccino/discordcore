
import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton, Button } from "../components/ui";
import { SettingsGroup, SettingsRow } from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useCorePageLogic } from "./hooks/useCorePageLogic";

export function CorePage() {
  const { settings, isLoading, tokensState, setTokensState, handleUpdateTokens } = useCorePageLogic();
  
  const availableInstances = settings?.workspace?.available_bot_instance_ids || [];
  const configuredTokens = settings?.workspace?.sections?.bot_instance_tokens || {};
  
  const isDirty = Object.keys(tokensState).length > 0;
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
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-lg font-semibold tracking-tight text-text-primary">Bot Instance Tokens</h3>
                {isDirty && (
                  <Button onClick={handleUpdateTokens} variant="primary" size="sm">
                    Save Changes
                  </Button>
                )}
              </div>
              <p className="text-sm text-text-secondary mb-4">
                Assign a specific bot developer token to each instance for this guild. Tokens are stored securely and are write-only.
              </p>
              <SettingsGroup>
                {availableInstances.map(instanceId => {
                  const hasToken = !!configuredTokens[instanceId];
                  return (
                    <SettingsRow
                      key={instanceId}
                      title={`Instance: ${instanceId}`}
                      description={hasToken ? "A token is currently configured for this instance." : "No token configured for this instance."}
                      control={
                        <input 
                          type="password" 
                          className="w-full max-w-[240px] px-3 py-2 bg-surface-base border border-border-default rounded-md text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 transition-shadow"
                          placeholder={hasToken ? "••••••••" : "Enter bot token..."}
                          value={tokensState[instanceId] || ""}
                          onChange={(e) => setTokensState(prev => ({ ...prev, [instanceId]: e.target.value }))}
                        />
                      }
                    />
                  );
                })}
              </SettingsGroup>
            </div>
          </Stack>
        )}
      </Stack>
    </PageContainer>
  );
}
