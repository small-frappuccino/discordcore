import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton, Button } from "../components/ui";
import { SelectMenuMultiple, ToggleSwitch, SettingsGroup, SettingsRow, TextInput } from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useCorePageLogic } from "./hooks/useCorePageLogic";
import { useState } from "react";

const BASE_FEATURE_OPTIONS = [
  { label: "QOTD", value: "qotd" },
  { label: "Moderation", value: "moderation", requiredPerms: 0x2000 },
  { label: "Roles", value: "roles", requiredPerms: 0x10000000 },
  { label: "Partners", value: "partners" },
  { label: "Embeds", value: "embeds" },
  { label: "Tickets", value: "tickets" },
];

export function CorePage() {
  const { 
    settings, 
    botProfiles, 
    isLoading, 
    tokensState, 
    setTokensState, 
    mainBotIdState, 
    setMainBotIdState, 
    featureRoutingState, 
    setFeatureRoutingState, 
    handleUpdateTokens, 
    isSaving, 
    saveError, 
    clearSaveError, 
    isDirty 
  } = useCorePageLogic();
  
  const availableInstances = settings?.workspace?.available_bot_instance_ids || [];
  const configuredTokens = settings?.workspace?.sections?.bot_instance_tokens_configured || {};
  
  const [enabledInstances, setEnabledInstances] = useState<Record<string, boolean>>({});

  const secondaryInstances = Array.from(new Set([
    "companion",
    ...availableInstances, 
    ...Object.keys(configuredTokens)
  ])).filter(id => id !== "main");
  
  const allInstances = ["main", ...secondaryInstances];

  const handleFeatureChange = (instanceId: string, features: string[]) => {
    const next = { ...featureRoutingState };
    // Remove features currently mapped to this instance
    for (const key of Object.keys(next)) {
      if (next[key] === instanceId) {
        delete next[key];
      }
    }
    // Re-add selected features mapped to this instance
    for (const f of features) {
      next[f] = instanceId;
    }
    setFeatureRoutingState(next);
  };

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
                  <h3 className="text-base font-semibold text-text-primary">Bot Profiles</h3>
                  {isDirty && (
                    <Button onClick={handleUpdateTokens} variant="primary" size="sm" isLoading={isSaving} disabled={isSaving}>
                      Save Changes
                    </Button>
                  )}
                </div>
                <p className="text-sm text-text-secondary">
                  Manage bot identities, secure tokens, and operational feature routing for this guild.
                </p>
                {saveError && (
                  <div className="mt-2 p-2 rounded bg-[var(--status-error-bg,rgba(239,68,68,0.1))] text-[var(--status-error,#ef4444)] text-sm flex items-center justify-between">
                    <span>{saveError}</span>
                    <button onClick={clearSaveError} className="ml-2 text-xs opacity-70 hover:opacity-100">&times;</button>
                  </div>
                )}
              </div>
              
              <Stack spacing="md">
                {allInstances.map((instanceId) => {
                  const hasToken = !!configuredTokens[instanceId];
                  const isEnabled = instanceId === "main" || enabledInstances[instanceId] || hasToken;
                  const isMain = mainBotIdState === instanceId || (instanceId === "main" && !mainBotIdState);
                  const profile = botProfiles?.find(p => p.logical_key === instanceId);
                  
                  // Collect features routed to this instance
                  const routedFeatures = Object.entries(featureRoutingState)
                    .filter(([, mappedId]) => mappedId === instanceId)
                    .map(([f]) => f);

                  return (
                    <SettingsGroup key={instanceId}>
                      {/* Identity Header */}
                      <div className={`p-4 flex items-center gap-4 ${isEnabled ? "border-b border-border-subtle" : ""}`}>
                        <div className="w-12 h-12 rounded-full overflow-hidden bg-bg-surface-active flex items-center justify-center shrink-0 border border-border-subtle">
                          {profile?.avatar_url ? (
                            <img src={profile.avatar_url} alt="Avatar" className="w-full h-full object-cover" />
                          ) : (
                            <span className="text-text-secondary text-lg font-bold">
                              {(profile?.username || instanceId).charAt(0).toUpperCase()}
                            </span>
                          )}
                        </div>
                        <div className="flex flex-col">
                          <div className="flex items-center gap-2">
                            <span className="font-semibold text-text-primary">
                              {profile ? profile.username : instanceId === "main" ? "Main Instance" : `Instance: ${instanceId}`}
                            </span>
                            {profile?.discriminator && profile.discriminator !== "0" && (
                              <span className="text-sm text-text-muted">#{profile.discriminator}</span>
                            )}
                            {isMain && <Badge variant="neutral">Primary</Badge>}
                          </div>
                          <span className="text-sm text-text-secondary">
                            {instanceId === "main" ? "Default bot handler" : `Companion (${instanceId})`}
                          </span>
                        </div>
                        {instanceId !== "main" && (
                          <div className="ml-auto">
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
                          </div>
                        )}
                      </div>

                      {/* Config Area - Hidden if secondary and disabled */}
                      {isEnabled && (
                        <>
                          {/* Token Section */}
                          <SettingsRow 
                            title={
                              <div className="flex items-center gap-2">
                                <span>Secure Token</span>
                                <Badge variant="danger">Sensitive</Badge>
                              </div>
                            }
                            control={
                              <TextInput 
                                type="password" 
                                className="w-full md:w-2/3 lg:w-1/2 border-white/20 pl-6"
                                placeholder={hasToken ? "•••••••• (Configured)" : "Enter bot token..."}
                                value={tokensState[instanceId] !== undefined ? tokensState[instanceId] : ""}
                                onChange={(e) => setTokensState(prev => ({ ...prev, [instanceId]: e.target.value }))}
                              />
                            }
                          />

                          {/* Routing Section */}
                          <SettingsRow 
                            title="Primary Bot Status"
                            control={
                              <ToggleSwitch 
                                checked={isMain} 
                                onCheckedChange={(checked) => {
                                  if (checked) setMainBotIdState(instanceId);
                                }} 
                              />
                            }
                          />
                          <SettingsRow 
                            title="Feature Routing"
                            control={
                              <SelectMenuMultiple 
                                className="w-full"
                                options={BASE_FEATURE_OPTIONS.map(opt => {
                                  if (!profile || !opt.requiredPerms) return { label: opt.label, value: opt.value };
                                  const perms = Number(profile.permissions || 0);
                                  const isAdmin = (perms & 0x8) === 0x8;
                                  const hasPerms = isAdmin || (perms & opt.requiredPerms) === opt.requiredPerms;
                                  return { label: opt.label, value: opt.value, disabled: !hasPerms };
                                })}
                                value={routedFeatures}
                                onChange={(values) => handleFeatureChange(instanceId, values)}
                                placeholder="Select features..."
                              />
                            }
                          />
                        </>
                      )}
                    </SettingsGroup>
                  );
                })}
              </Stack>
            </Stack>
          )}
        </Stack>
      </div>
    </PageContainer>
  );
}
