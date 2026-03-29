import { useEffect, useState } from "react";
import type { GuildRolesSettingsSection } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";

interface CachedGuildRolesSettings {
  roles: GuildRolesSettingsSnapshot;
  fetchedAt: number;
}

export interface GuildRolesSettingsSnapshot {
  allowedRoleIds: string[];
  dashboardReadRoleIds: string[];
  dashboardWriteRoleIds: string[];
}

const guildRolesSettingsCache = new Map<string, CachedGuildRolesSettings>();

const emptySnapshot: GuildRolesSettingsSnapshot = {
  allowedRoleIds: [],
  dashboardReadRoleIds: [],
  dashboardWriteRoleIds: [],
};

export function useGuildRolesSettings() {
  const { authState, baseUrl, client, selectedGuildID } = useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const cacheKey =
    normalizedGuildID === "" ? "" : `${baseUrl}::${normalizedGuildID}`;
  const [roles, setRoles] = useState<GuildRolesSettingsSnapshot>(() => {
    if (cacheKey === "") {
      return emptySnapshot;
    }
    return peekGuildRolesSettings(baseUrl, normalizedGuildID);
  });
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  useEffect(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      setRoles(emptySnapshot);
      setLoading(false);
      setNotice(null);
      return;
    }

    const cachedRoles = peekGuildRolesSettings(baseUrl, normalizedGuildID);
    setRoles(cachedRoles);
    setNotice(null);

    let cancelled = false;

    async function loadRoles() {
      setLoading(isRolesSettingsEmpty(cachedRoles));

      try {
        const nextRoles = await loadGuildRolesSettings(client, baseUrl, normalizedGuildID);
        if (cancelled) {
          return;
        }
        setRoles(nextRoles);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        const fallbackRoles = peekGuildRolesSettings(baseUrl, normalizedGuildID);
        setRoles(fallbackRoles);
        if (isRolesSettingsEmpty(fallbackRoles)) {
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
      const nextRoles = await loadGuildRolesSettings(client, baseUrl, normalizedGuildID, {
        force: true,
      });
      setRoles(nextRoles);
      setNotice(null);
    } catch (error) {
      const fallbackRoles = peekGuildRolesSettings(baseUrl, normalizedGuildID);
      setRoles(fallbackRoles);
      if (isRolesSettingsEmpty(fallbackRoles)) {
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

  function updateCachedRoles(nextRoles: GuildRolesSettingsSnapshot) {
    if (normalizedGuildID === "") {
      return;
    }
    writeGuildRolesSettingsCache(baseUrl, normalizedGuildID, nextRoles);
    setRoles(nextRoles);
  }

  return {
    roles,
    loading,
    notice,
    refresh,
    updateCachedRoles,
    clearNotice: () => setNotice(null),
  };
}

export async function loadGuildRolesSettings(
  client: { getGuildSettings: (guildId: string) => Promise<{ workspace: { sections: { roles: GuildRolesSettingsSection } } }> },
  baseUrl: string,
  guildID: string,
  options: { force?: boolean } = {},
) {
  const normalizedGuildID = guildID.trim();
  if (normalizedGuildID === "") {
    return emptySnapshot;
  }

  if (!options.force) {
    const cached = peekGuildRolesSettings(baseUrl, normalizedGuildID);
    if (!isRolesSettingsEmpty(cached)) {
      return cached;
    }
  }

  const response = await client.getGuildSettings(normalizedGuildID);
  const snapshot = mapGuildRolesSettings(response.workspace.sections.roles);
  writeGuildRolesSettingsCache(baseUrl, normalizedGuildID, snapshot);
  return snapshot;
}

export function peekGuildRolesSettings(baseUrl: string, guildID: string) {
  const entry = guildRolesSettingsCache.get(buildCacheKey(baseUrl, guildID));
  if (entry === undefined) {
    return emptySnapshot;
  }
  return entry.roles;
}

export function writeGuildRolesSettingsCache(
  baseUrl: string,
  guildID: string,
  roles: GuildRolesSettingsSnapshot,
) {
  guildRolesSettingsCache.set(buildCacheKey(baseUrl, guildID), {
    roles,
    fetchedAt: Date.now(),
  });
}

function buildCacheKey(baseUrl: string, guildID: string) {
  return `${baseUrl}::${guildID.trim()}`;
}

function mapGuildRolesSettings(roles: GuildRolesSettingsSection) {
  return {
    allowedRoleIds: normalizeRoleIds(roles.allowed),
    dashboardReadRoleIds: normalizeRoleIds(roles.dashboard_read),
    dashboardWriteRoleIds: normalizeRoleIds(roles.dashboard_write),
  };
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
