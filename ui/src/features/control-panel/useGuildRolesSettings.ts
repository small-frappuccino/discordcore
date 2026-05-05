import { useEffect, useState } from "react";
import type {
  GuildBotRoutingSettingsSection,
  GuildRolesSettingsSection,
} from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";

interface CachedGuildRolesSettings {
  settings: GuildControlPanelSettingsSnapshot;
  fetchedAt: number;
}

export interface GuildRolesSettingsSnapshot {
  allowedRoleIds: string[];
  dashboardReadRoleIds: string[];
  dashboardWriteRoleIds: string[];
}

export interface GuildBotRoutingSnapshot {
  botInstanceID: string;
  availableBotInstanceIDs: string[];
  domainOverrideBotInstanceIDs: string[];
  domainBotInstanceIDs: Record<string, string>;
  editableDomains: string[];
}

export interface GuildControlPanelSettingsSnapshot {
  roles: GuildRolesSettingsSnapshot;
  botRouting: GuildBotRoutingSnapshot;
}

const guildRolesSettingsCache = new Map<string, CachedGuildRolesSettings>();

const emptySnapshot: GuildRolesSettingsSnapshot = {
  allowedRoleIds: [],
  dashboardReadRoleIds: [],
  dashboardWriteRoleIds: [],
};

function createEmptyBotRoutingSnapshot(): GuildBotRoutingSnapshot {
  return {
    botInstanceID: "",
    availableBotInstanceIDs: [],
    domainOverrideBotInstanceIDs: [],
    domainBotInstanceIDs: {},
    editableDomains: [],
  };
}

function createEmptyControlPanelSettingsSnapshot(): GuildControlPanelSettingsSnapshot {
  return {
    roles: emptySnapshot,
    botRouting: createEmptyBotRoutingSnapshot(),
  };
}

export function useGuildRolesSettings() {
  const { authState, baseUrl, client, selectedGuildID } = useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const cacheKey =
    normalizedGuildID === "" ? "" : `${baseUrl}::${normalizedGuildID}`;
  const [settings, setSettings] = useState<GuildControlPanelSettingsSnapshot>(() => {
    if (cacheKey === "") {
      return createEmptyControlPanelSettingsSnapshot();
    }
    return peekGuildControlPanelSettings(baseUrl, normalizedGuildID);
  });
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  useEffect(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      setSettings(createEmptyControlPanelSettingsSnapshot());
      setLoading(false);
      setNotice(null);
      return;
    }

    const cachedRolesEntry = readGuildRolesSettingsCache(baseUrl, normalizedGuildID);
    const cachedSettings =
      cachedRolesEntry?.settings ?? createEmptyControlPanelSettingsSnapshot();
    setSettings(cachedSettings);
    setNotice(null);

    let cancelled = false;

    async function loadRoles() {
      if (cachedRolesEntry !== null) {
        setLoading(false);
        return;
      }

      setLoading(true);

      try {
        const nextSettings = await loadGuildRolesSettings(client, baseUrl, normalizedGuildID);
        if (cancelled) {
          return;
        }
        setSettings(nextSettings);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        const fallbackSettings = peekGuildControlPanelSettings(baseUrl, normalizedGuildID);
        setSettings(fallbackSettings);
        if (isRolesSettingsEmpty(fallbackSettings.roles)) {
          setNotice({
            tone: "error",
            message: formatError(error),
          });
        } else {
          setNotice(null);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void loadRoles();

    return () => {
      cancelled = true;
    };
  }, [authState, baseUrl, client, normalizedGuildID]);

  async function refresh() {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      return;
    }

    setLoading(true);

    try {
      const nextSettings = await loadGuildRolesSettings(client, baseUrl, normalizedGuildID, {
        force: true,
      });
      setSettings(nextSettings);
      setNotice(null);
    } catch (error) {
      const fallbackSettings = peekGuildControlPanelSettings(baseUrl, normalizedGuildID);
      setSettings(fallbackSettings);
      if (isRolesSettingsEmpty(fallbackSettings.roles)) {
        setNotice({
          tone: "error",
          message: formatError(error),
        });
      } else {
        setNotice(null);
      }
    } finally {
      setLoading(false);
    }
  }

  function updateCachedSettings(nextSettings: GuildControlPanelSettingsSnapshot) {
    if (normalizedGuildID === "") {
      return;
    }
    writeGuildRolesSettingsCache(baseUrl, normalizedGuildID, nextSettings);
    setSettings(nextSettings);
  }

  function updateCachedRoles(nextRoles: GuildRolesSettingsSnapshot) {
    updateCachedSettings({
      roles: nextRoles,
      botRouting: settings.botRouting,
    });
  }

  function updateCachedBotRouting(nextBotRouting: GuildBotRoutingSnapshot) {
    updateCachedSettings({
      roles: settings.roles,
      botRouting: nextBotRouting,
    });
  }

  return {
    roles: settings.roles,
    botRouting: settings.botRouting,
    loading,
    notice,
    refresh,
    updateCachedSettings,
    updateCachedRoles,
    updateCachedBotRouting,
    clearNotice: () => setNotice(null),
  };
}

export async function loadGuildRolesSettings(
  client: { getGuildSettings: (guildId: string) => Promise<{ workspace: { sections: { roles: GuildRolesSettingsSection; bot_routing?: GuildBotRoutingSettingsSection } } }> },
  baseUrl: string,
  guildID: string,
  options: { force?: boolean } = {},
) {
  const normalizedGuildID = guildID.trim();
  if (normalizedGuildID === "") {
    return createEmptyControlPanelSettingsSnapshot();
  }

  if (!options.force) {
    const cachedEntry = readGuildRolesSettingsCache(baseUrl, normalizedGuildID);
    if (cachedEntry !== null) {
      return cachedEntry.settings;
    }
  }

  const response = await client.getGuildSettings(normalizedGuildID);
  const snapshot = mapGuildControlPanelSettings(response.workspace.sections);
  writeGuildRolesSettingsCache(baseUrl, normalizedGuildID, snapshot);
  return snapshot;
}

export function peekGuildControlPanelSettings(baseUrl: string, guildID: string) {
  const entry = readGuildRolesSettingsCache(baseUrl, guildID);
  if (entry === null) {
    return createEmptyControlPanelSettingsSnapshot();
  }
  return entry.settings;
}

export function peekGuildRolesSettings(baseUrl: string, guildID: string) {
  return peekGuildControlPanelSettings(baseUrl, guildID).roles;
}

export function writeGuildRolesSettingsCache(
  baseUrl: string,
  guildID: string,
  settings: GuildControlPanelSettingsSnapshot,
) {
  guildRolesSettingsCache.set(buildCacheKey(baseUrl, guildID), {
    settings,
    fetchedAt: Date.now(),
  });
}

  export function clearGuildRolesSettingsCache() {
    guildRolesSettingsCache.clear();
  }

function buildCacheKey(baseUrl: string, guildID: string) {
  return `${baseUrl}::${guildID.trim()}`;
}

function readGuildRolesSettingsCache(baseUrl: string, guildID: string) {
  return guildRolesSettingsCache.get(buildCacheKey(baseUrl, guildID)) ?? null;
}

function mapGuildControlPanelSettings(sections: {
  roles: GuildRolesSettingsSection;
  bot_routing?: GuildBotRoutingSettingsSection;
}): GuildControlPanelSettingsSnapshot {
  return {
    roles: mapGuildRolesSettings(sections.roles),
    botRouting: mapGuildBotRoutingSettings(sections.bot_routing),
  };
}

function mapGuildRolesSettings(roles: GuildRolesSettingsSection) {
  return {
    allowedRoleIds: normalizeRoleIds(roles.allowed),
    dashboardReadRoleIds: normalizeRoleIds(roles.dashboard_read),
    dashboardWriteRoleIds: normalizeRoleIds(roles.dashboard_write),
  };
}

function mapGuildBotRoutingSettings(
  botRouting: GuildBotRoutingSettingsSection | undefined,
): GuildBotRoutingSnapshot {
  const availableBotInstanceIDs = normalizeStringList(botRouting?.available_bot_instance_ids);
  return {
    botInstanceID: normalizeStringValue(botRouting?.bot_instance_id),
    availableBotInstanceIDs,
    domainOverrideBotInstanceIDs: normalizeStringList(
      botRouting?.domain_override_bot_instance_ids,
    ).length > 0
      ? normalizeStringList(botRouting?.domain_override_bot_instance_ids)
      : availableBotInstanceIDs,
    domainBotInstanceIDs: normalizeDomainBotInstanceIDs(botRouting?.domain_bot_instance_ids),
    editableDomains: normalizeStringList(botRouting?.editable_domains).map((domain) =>
      domain.toLowerCase(),
    ),
  };
}

function normalizeDomainBotInstanceIDs(
  domainBotInstanceIDs: Record<string, string> | undefined,
) {
  if (domainBotInstanceIDs == null || typeof domainBotInstanceIDs !== "object") {
    return {};
  }

  const normalized: Record<string, string> = {};
  for (const [domain, botInstanceID] of Object.entries(domainBotInstanceIDs)) {
    const normalizedDomain = normalizeStringValue(domain).toLowerCase();
    const normalizedBotInstanceID = normalizeStringValue(botInstanceID);
    if (normalizedDomain === "" || normalizedBotInstanceID === "") {
      continue;
    }
    normalized[normalizedDomain] = normalizedBotInstanceID;
  }
  return normalized;
}

function normalizeStringList(values: string[] | undefined) {
  if (!Array.isArray(values)) {
    return [];
  }
  return values
    .filter((value): value is string => typeof value === "string")
    .map((value) => value.trim())
    .filter((value, index, allValues) => value !== "" && allValues.indexOf(value) === index);
}

function normalizeStringValue(value: string | undefined) {
  if (typeof value !== "string") {
    return "";
  }
  return value.trim();
}

function normalizeRoleIds(roleIds: string[] | undefined) {
  if (!Array.isArray(roleIds)) {
    return [];
  }
  return roleIds
    .filter((roleId): roleId is string => typeof roleId === "string")
    .map((roleId) => roleId.trim())
    .filter((roleId) => roleId !== "");
}

function isRolesSettingsEmpty(roles: GuildRolesSettingsSnapshot) {
  return (
    roles.allowedRoleIds.length === 0 &&
    roles.dashboardReadRoleIds.length === 0 &&
    roles.dashboardWriteRoleIds.length === 0
  );
}
