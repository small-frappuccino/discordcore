import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton, Button } from "../components/ui";
import { SelectMenuMultiple, SettingsGroup, SettingsRow, TextInput } from "../components/ui/tahoe";
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
    featureRoutingState,
    setFeatureRoutingState,
    handleUpdateTokens,
    isSaving,
    saveError,
    clearSaveError,
    isDirty
  } = useCorePageLogic();

  const availableInstances = useMemo(() => settings?.workspace?.available_bot_instance_ids || [], [settings?.workspace?.available_bot_instance_ids]);
  const configuredTokens = useMemo(() => settings?.workspace?.sections?.bot_instance_tokens_configured || {}, [settings?.workspace?.sections?.bot_instance_tokens_configured]);

  const [addedProfiles, setAddedProfiles] = useState<string[]>([]);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  useEffect(() => {
    const handleClickOutside = () => setOpenMenuId(null);
    document.addEventListener("click", handleClickOutside);
    return () => document.removeEventListener("click", handleClickOutside);
  }, []);

  const allInstances = useMemo(() => {
    return Array.from(new Set([
      ...availableInstances,
      ...Object.keys(configuredTokens),
      ...addedProfiles
    ]));
  }, [availableInstances, configuredTokens, addedProfiles]);

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

    setAddedProfiles(prev => Array.from(new Set([...prev, sanitized])));

    setIsCreatingProfile(false);
    setNewProfileName("");
  };

  const handleAddProfileCancel = () => {
    setIsCreatingProfile(false);
    setNewProfileName("");
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
                <p className="text-sm text-text-secondary mb-2">
                  Manage bot identities, secure tokens, and operational feature routing for this guild.
                </p>
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
                  const profile = botProfiles?.find(p => p.logical_key === instanceId);

                  // Collect features routed to this instance
                  const routedFeatures = Object.entries(featureRoutingState)
                    .filter(([, mappedId]) => mappedId === instanceId)
                    .map(([f]) => f);

                  // Don't show instances that were marked for deletion but haven't saved yet
                  if (tokensState[instanceId] === "") return null;

                  return (
                    <SettingsGroup key={instanceId}>
                      {/* Identity Header */}
                      <div className="p-4 flex items-center gap-4 border-b-[1px] border-b-[var(--border-subtle)]">
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
                          </div>
                          <span className="text-sm text-text-secondary">
                            Logical ID: {instanceId}
                          </span>
                        </div>
                        <div className="ml-auto relative">
                          <button 
                            className="p-1 rounded hover:bg-bg-surface-active text-text-muted hover:text-text-primary transition-colors"
                            onClick={(e) => {
                              e.stopPropagation();
                              setOpenMenuId(openMenuId === instanceId ? null : instanceId);
                            }}
                          >
                            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                              <circle cx="12" cy="12" r="1"></circle>
                              <circle cx="12" cy="5" r="1"></circle>
                              <circle cx="12" cy="19" r="1"></circle>
                            </svg>
                          </button>
                          {openMenuId === instanceId && (
                            <div className="shell-dropdown">
                              <button 
                                className="shell-dropdown-item danger"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setOpenMenuId(null);
                                  if (confirm(`Are you sure you want to delete profile ${instanceId}?`)) {
                                    setTokensState(prev => ({ ...prev, [instanceId]: "" }));
                                    setAddedProfiles(prev => prev.filter(id => id !== instanceId));
                                    const nextRouting = { ...featureRoutingState };
                                    for (const key of Object.keys(nextRouting)) {
                                      if (nextRouting[key] === instanceId) {
                                        delete nextRouting[key];
                                      }
                                    }
                                    setFeatureRoutingState(nextRouting);
                                  }
                                }}
                              >
                                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                  <path d="M3 6h18"></path>
                                  <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                                </svg>
                                Delete Profile
                              </button>
                            </div>
                          )}
                        </div>
                      </div>

                      {/* Config Area - Hidden if secondary and disabled */}
                          {/* Token Section */}
                          <SettingsRow
                            title={
                              <div className="flex flex-col gap-1">
                                <div className="flex items-center gap-2">
                                  <span>Token</span>
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
