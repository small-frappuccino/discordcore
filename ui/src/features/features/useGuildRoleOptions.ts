import { useEffect, useState } from "react";
import type { GuildRoleOption } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";

export function useGuildRoleOptions() {
  const { authState, client, selectedGuildID } = useDashboardSession();
  const [roles, setRoles] = useState<GuildRoleOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  function resetRoleOptions() {
    setRoles([]);
    setLoading(false);
    setNotice(null);
  }

  async function refresh() {
    if (authState !== "signed_in" || selectedGuildID.trim() === "") {
      return;
    }

    setLoading(true);

    try {
      const response = await client.listGuildRoleOptions(selectedGuildID.trim());
      setRoles(response.roles);
      setNotice(null);
    } catch (error) {
      setRoles([]);
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (authState !== "signed_in" || selectedGuildID.trim() === "") {
      resetRoleOptions();
      return;
    }

    let cancelled = false;

    async function loadRoleOptions() {
      setLoading(true);

      try {
        const response = await client.listGuildRoleOptions(selectedGuildID.trim());
        if (cancelled) {
          return;
        }
        setRoles(response.roles);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setRoles([]);
        setNotice({
          tone: "error",
          message: formatError(error),
        });
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
  }, [authState, client, selectedGuildID]);

  return {
    roles,
    loading,
    notice,
    clearNotice: () => setNotice(null),
    refresh,
  };
}
