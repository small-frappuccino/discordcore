import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton, Button } from "../components/ui";
import { SettingsGroup, SettingsRow, ToggleSwitch } from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useCorePageLogic } from "./hooks/useCorePageLogic";
import { useState, Fragment } from "react";

export function CorePage() {
  const { settings, isLoading, tokensState, setTokensState, handleUpdateTokens, isSaving, saveError, clearSaveError } = useCorePageLogic();
  
  const availableInstances = settings?.workspace?.available_bot_instance_ids || [];
  const configuredTokens = settings?.workspace?.sections?.bot_instance_tokens_configured || {};
  
  const [enabledInstances, setEnabledInstances] = useState<Record<string, boolean>>({});

  const isDirty = Object.keys(tokensState).length > 0;

  // Ensure main is always present, filter it out from secondary instances
  // We explicitly add 'companion' to the set so it always renders, even if not currently connected
  const secondaryInstances = Array.from(new Set([
    "companion",
    ...availableInstances, 
    ...Object.keys(configuredTokens)
  ])).filter(id => id !== "main");

  return (
    <PageContainer>
      <div className="settings-form">
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
              <div className="mb-6">
                <div className="flex items-center justify-between mb-1">
                  <h3 className="text-base font-semibold text-text-primary">Bot Instance Tokens</h3>
                  {isDirty && (
                    <Button onClick={handleUpdateTokens} variant="primary" size="sm" isLoading={isSaving} disabled={isSaving}>
                      Save Changes
                    </Button>
                  )}
                </div>
                <p className="text-sm text-text-secondary">
                  Assign a specific bot developer token to each instance for this guild. Tokens are stored securely and are write-only.
                </p>
                {saveError && (
                  <div className="mt-2 p-2 rounded bg-[var(--status-error-bg,rgba(239,68,68,0.1))] text-[var(--status-error,#ef4444)] text-sm flex items-center justify-between">
                    <span>{saveError}</span>
                    <button onClick={clearSaveError} className="ml-2 text-xs opacity-70 hover:opacity-100">&times;</button>
                  </div>
                )}
              </div>
              <SettingsGroup>
                  {/* Main Instance (Always Visible) */}
                  <SettingsRow
                    title="Instance: main"
                    description={configuredTokens["main"] ? "A token is currently configured for this instance." : "No token configured for this instance."}
                    control={
                      <input 
                        type="password" 
                        className="tahoe-text-input w-full min-w-[320px]"
                        placeholder={configuredTokens["main"] ? "••••••••" : "Enter bot token..."}
                        value={tokensState["main"] !== undefined ? tokensState["main"] : ""}
                        onChange={(e) => setTokensState(prev => ({ ...prev, main: e.target.value }))}
                      />
                    }
                  />

                  {/* Secondary Instances (Toggleable) */}
                  {secondaryInstances.map((instanceId) => {
                    const hasToken = !!configuredTokens[instanceId];
                    const isEnabled = enabledInstances[instanceId] || hasToken;
                    const label = instanceId === "companion" ? "discordqotd (companion)" : instanceId;
                    
                    return (
                      <Fragment key={instanceId}>
                        <SettingsRow
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
                          <div style={{ borderTop: "1px solid var(--border-subtle)" }}>
                            <SettingsRow
                              title=""
                              control={
                                <input 
                                  type="password" 
                                  className="tahoe-text-input w-full min-w-[320px]"
                                  placeholder={hasToken ? "••••••••" : "Enter bot token..."}
                                  value={tokensState[instanceId] !== undefined ? tokensState[instanceId] : ""}
                                  onChange={(e) => setTokensState(prev => ({ ...prev, [instanceId]: e.target.value }))}
                                />
                              }
                            />
                          </div>
                        )}
                      </Fragment>
                    );
                  })}
                </SettingsGroup>
            </Stack>
          )}
        </Stack>
      </div>
    </PageContainer>
  );
}
