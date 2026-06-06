import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton, Button } from "../components/ui";
import { SettingsGroup, SettingsRow, ToggleSwitch } from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useCorePageLogic } from "./hooks/useCorePageLogic";
import { useState } from "react";

export function CorePage() {
  const { settings, isLoading, tokensState, setTokensState, handleUpdateTokens } = useCorePageLogic();
  
  const availableInstances = settings?.workspace?.available_bot_instance_ids || [];
  const configuredTokens = settings?.workspace?.sections?.bot_instance_tokens_configured || {};
  
  const [enabledInstances, setEnabledInstances] = useState<Record<string, boolean>>({});

  const isDirty = Object.keys(tokensState).length > 0;

  // Ensure main is always present, filter it out from secondary instances
  const secondaryInstances = Array.from(new Set([...availableInstances, ...Object.keys(configuredTokens)]))
    .filter(id => id !== "main");

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
                {/* Main Instance (Always Visible) */}
                <SettingsRow
                  title="Instance: main"
                  description={configuredTokens["main"] ? "A token is currently configured for this instance." : "No token configured for this instance."}
                  control={
                    <div className="flex items-center gap-2">
                      <input 
                        type="password" 
                        className="w-full max-w-[240px] px-3 py-2 bg-surface-base border border-border-default rounded-md text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 transition-shadow"
                        placeholder={configuredTokens["main"] ? "••••••••" : "Enter bot token..."}
                        value={tokensState["main"] !== undefined ? tokensState["main"] : ""}
                        onChange={(e) => setTokensState(prev => ({ ...prev, main: e.target.value }))}
                      />
                      {(configuredTokens["main"] || tokensState["main"] !== undefined) && (
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setTokensState(prev => ({ ...prev, main: "" }))}
                        >
                          Clear
                        </Button>
                      )}
                    </div>
                  }
                />

                {/* Secondary Instances (Toggleable) */}
                {secondaryInstances.map(instanceId => {
                  const hasToken = !!configuredTokens[instanceId];
                  const isEnabled = enabledInstances[instanceId] || hasToken;
                  const label = instanceId === "companion" ? "discordqotd (companion)" : instanceId;
                  
                  return (
                    <div key={instanceId} className="flex flex-col border-t border-border-subtle">
                      <SettingsRow
                        className="!border-b-0"
                        title={`Instance: ${label}`}
                        description={hasToken ? "A token is currently configured for this instance." : "Enable to configure a custom token for this instance."}
                        control={
                          <ToggleSwitch 
                            checked={isEnabled} 
                            onCheckedChange={(checked) => {
                              setEnabledInstances(prev => ({ ...prev, [instanceId]: checked }));
                              if (!checked && !hasToken) {
                                setTokensState(prev => {
                                  const next = { ...prev };
                                  delete next[instanceId];
                                  return next;
                                });
                              }
                            }} 
                          />
                        }
                      />
                      {isEnabled && (
                        <div className="px-5 pb-5 pt-1 flex items-center justify-end">
                          <div className="flex items-center gap-2">
                            <input 
                              type="password" 
                              className="w-full min-w-[320px] px-3 py-2 bg-surface-base border border-border-default rounded-md text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 transition-shadow"
                              placeholder={hasToken ? "••••••••" : "Enter bot token..."}
                              value={tokensState[instanceId] !== undefined ? tokensState[instanceId] : ""}
                              onChange={(e) => setTokensState(prev => ({ ...prev, [instanceId]: e.target.value }))}
                            />
                            {(hasToken || tokensState[instanceId] !== undefined) && (
                              <Button
                                variant="secondary"
                                size="sm"
                                onClick={() => setTokensState(prev => ({ ...prev, [instanceId]: "" }))}
                              >
                                Clear
                              </Button>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
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
