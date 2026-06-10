import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton, Button } from "../components/ui";
import { SelectMenuMultiple, ToggleSwitch, SettingsGroup, SettingsRow, TextInput } from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useCorePageLogic } from "./hooks/useCorePageLogic";
import { useState, useEffect, useMemo } from "react";
import { useParams } from "react-router-dom";
import { useDashboardSession } from "../context/DashboardSessionContext";

const BASE_FEATURE_OPTIONS = [
  { label: "QOTD", value: "qotd" },
  { label: "Moderation", value: "moderation", requiredPerms: 0x2000 },
  { label: "Roles", value: "roles", requiredPerms: 0x10000000 },
  { label: "Partners", value: "partners" },
  { label: "Embeds", value: "embeds" },
  { label: "Tickets", value: "tickets" },
];

export function CorePage() {
  const { guildId } = useParams<{ guildId: string }>();
  const { accessibleGuilds, manageableGuilds, baseUrl } = useDashboardSession();

  const activeGuild = accessibleGuilds.find(g => g.id === guildId) || manageableGuilds.find(g => g.id === guildId);
  const botPresent = activeGuild?.bot_present === true;

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

  // Dynamic profile list derived purely from existing tokens + currently enabled instances.
  // There are no hardcoded keys.
  const allInstances = useMemo(() => {
    // Stringify objects to use as stable dependency comparisons if needed, 
    // but the object references update from the hook so we can depend on them directly.
    return Array.from(new Set([
      ...availableInstances,
      ...Object.keys(configuredTokens),
      ...Object.keys(enabledInstances)
    ]));
  }, [availableInstances, configuredTokens, enabledInstances]);

  const [isCreatingProfile, setIsCreatingProfile] = useState(false);
  const [newProfileName, setNewProfileName] = useState("");

  const handleFeatureChange = (instanceId: string, features: string[]) => {
    const next = { ...featureRoutingState };
    for (const key of Object.keys(next)) {
      if (next[key] === instanceId) {
        delete next[key];
      }
    }
    for (const f of features) {
      next[f] = instanceId;
    }
    setFeatureRoutingState(next);
  };

  const handleAddProfileSave = () => {
    const profileName = newProfileName.trim();
    if (!profileName) return;

    const sanitized = profileName.toLowerCase().replace(/[^a-z0-9_]/g, '');
    if (!sanitized) {
      alert("Invalid name. Use only letters, numbers, and underscores.");
      return;
    }

    setEnabledInstances(prev => ({ ...prev, [sanitized]: true }));

    // Auto-select as primary if it's the very first profile created
    if (!mainBotIdState && allInstances.length === 0) {
      setMainBotIdState(sanitized);
    }

    setIsCreatingProfile(false);
    setNewProfileName("");
  };

  const handleAddProfileCancel = () => {
    setIsCreatingProfile(false);
    setNewProfileName("");
  };

  // Safe Hydration Check: Only default mainBotIdState if fully loaded and not already set
  useEffect(() => {
    if (!isLoading && settings && !mainBotIdState) {
      const persistedMain = settings.workspace.sections.main_bot_instance_id;
      if (persistedMain) {
        setMainBotIdState(persistedMain);
      }
    }
  }, [isLoading, settings, mainBotIdState, setMainBotIdState]);

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
                <p className="text-sm text-text-secondary mb-2">
                  Manage bot identities, secure tokens, and operational feature routing for this guild.
                </p>
                <div className="p-3 mb-4 rounded-md bg-[var(--status-warning-bg,rgba(245,158,11,0.1))] text-sm">
                  <strong className="text-[var(--status-warning,#f59e0b)]">Getting Started:</strong> To add a bot to your server, you must first create a profile and provide its secure token. Once saved, you will be able to authorize it.
                </div>
                {isCreatingProfile ? (
                  <div className="mt-2 flex items-center gap-2">
                    <TextInput
                      value={newProfileName}
                      onChange={e => setNewProfileName(e.target.value)}
                      placeholder="e.g., custom_qotd"
                      autoFocus
                    />
                    <Button onClick={handleAddProfileSave} variant="primary" size="sm">Save</Button>
                    <Button onClick={handleAddProfileCancel} variant="secondary" size="sm">Cancel</Button>
                  </div>
                ) : (
                  <div className="mt-2">
                    <Button onClick={() => setIsCreatingProfile(true)} variant="secondary" size="sm">
                      + Add Profile
                    </Button>
                  </div>
                )}
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
                      <div className={`p-4 flex items-center gap-4 ${isEnabled ? "border-b-[1px] border-b-[var(--border-subtle)]" : ""}`}>
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
                              {profile ? profile.username : `Instance: ${instanceId}`}
                            </span>
                            {profile?.discriminator && profile.discriminator !== "0" && (
                              <span className="text-sm text-text-muted">#{profile.discriminator}</span>
                            )}
                            {isMain && <Badge variant="neutral">Primary</Badge>}
                          </div>
                          <span className="text-sm text-text-secondary">
                            Logical ID: {instanceId}
                          </span>
                        </div>
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
                      </div>

                      {/* Config Area - Hidden if secondary and disabled */}
                      {isEnabled && (
                        <>
                          {/* Token Section */}
                          <SettingsRow
                            title={
                              <div className="flex flex-col gap-1">
                                <div className="flex items-center gap-2">
                                  <span>Secure Token</span>
                                  <Badge variant="danger">Sensitive</Badge>
                                </div>
                                {hasToken && !botPresent && profile && (
                                  <div className="text-xs text-[var(--status-warning,#f59e0b)] font-medium mt-1">
                                    ⚠️ Bot is not in server
                                  </div>
                                )}
                              </div>
                            }
                            control={
                              <div className="w-full flex-1 flex flex-col gap-2">
                                <TextInput
                                  type="password"
                                  className="w-full border-white/20 pl-6"
                                  placeholder={hasToken ? "•••••••• (Configured)" : "Enter bot token..."}
                                  value={tokensState[instanceId] !== undefined ? tokensState[instanceId] : ""}
                                  onChange={(e) => setTokensState(prev => ({ ...prev, [instanceId]: e.target.value }))}
                                />
                                {hasToken && !botPresent && profile && (
                                  <a
                                    href={`${baseUrl === "" ? "" : baseUrl}/v1/guilds/${guildId}/oauth/authorize?bot_instance_id=${instanceId}`}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="text-xs inline-flex items-center gap-1 text-[var(--status-warning,#f59e0b)] hover:underline self-start bg-[var(--status-warning-bg,rgba(245,158,11,0.1))] px-2 py-1 rounded"
                                  >
                                    Authorize {profile.username} Now →
                                  </a>
                                )}
                              </div>
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
