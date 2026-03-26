import { useEffect, useState } from "react";
import type { GuildRoleOption } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { loadGuildRoleOptions, peekGuildRoleOptions } from "./guildResourceCache";

export function useGuildRoleOptions() {
  const { authState, baseUrl, client, selectedGuildID } = useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const [roles, setRoles] = useState<GuildRoleOption[]>(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      return [];
    }
    return peekGuildRoleOptions(baseUrl, normalizedGuildID);
  });
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  function resetRoleOptions() {
    setRoles([]);
    setLoading(false);
    setNotice(null);
  }

  async function refresh() {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      return;
    }

    setLoading(true);

    try {
      const nextRoles = await loadGuildRoleOptions(
        client,
        baseUrl,
        normalizedGuildID,
        {
          force: true,
        },
      );
      setRoles(nextRoles);
      setNotice(null);
    } catch (error) {
      const cachedRoles = peekGuildRoleOptions(baseUrl, normalizedGuildID);
      setRoles(cachedRoles);
      if (cachedRoles.length === 0) {
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

  useEffect(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      resetRoleOptions();
      return;
    }

    const cachedRoles = peekGuildRoleOptions(baseUrl, normalizedGuildID);
    setRoles(cachedRoles);
    setNotice(null);

    let cancelled = false;

    async function loadRoleOptions() {
      setLoading(cachedRoles.length === 0);

      try {
        const nextRoles = await loadGuildRoleOptions(
          client,
          baseUrl,
          normalizedGuildID,
        );
        if (cancelled) {
          return;
        }
        setRoles(nextRoles);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        const cachedErrorRoles = peekGuildRoleOptions(baseUrl, normalizedGuildID);
        setRoles(cachedErrorRoles);
        if (cachedErrorRoles.length === 0) {
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

    void loadRoleOptions();

    return () => {
      cancelled = true;
    };
  }, [authState, baseUrl, client, normalizedGuildID]);

  return {
    roles,
    loading,
    notice,
    clearNotice: () => setNotice(null),
    refresh,
  };
}
