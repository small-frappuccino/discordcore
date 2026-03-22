import { useEffect, useState } from "react";
import type { GuildMemberOption } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";

interface GuildMemberOptionsArgs {
  enabled?: boolean;
  limit?: number;
  query?: string;
  selectedMemberId?: string;
}

export function useGuildMemberOptions({
  enabled = false,
  limit = 25,
  query = "",
  selectedMemberId = "",
}: GuildMemberOptionsArgs = {}) {
  const { authState, client, selectedGuildID } = useDashboardSession();
  const [members, setMembers] = useState<GuildMemberOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  const normalizedQuery = query.trim();
  const normalizedSelectedMemberId = selectedMemberId.trim();

  function resetMemberOptions() {
    setMembers([]);
    setLoading(false);
    setNotice(null);
  }

  async function refresh() {
    if (!enabled || authState !== "signed_in" || selectedGuildID.trim() === "") {
      return;
    }

    setLoading(true);

    try {
      const response = await client.listGuildMemberOptions(selectedGuildID.trim(), {
        query: normalizedQuery,
        selectedId: normalizedSelectedMemberId,
        limit,
      });
      setMembers(response.members);
      setNotice(null);
    } catch (error) {
      setMembers([]);
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (!enabled || authState !== "signed_in" || selectedGuildID.trim() === "") {
      resetMemberOptions();
      return;
    }

    let cancelled = false;
    const timeout = window.setTimeout(() => {
      void (async () => {
        setLoading(true);

        try {
          const response = await client.listGuildMemberOptions(selectedGuildID.trim(), {
            query: normalizedQuery,
            selectedId: normalizedSelectedMemberId,
            limit,
          });
          if (cancelled) {
            return;
          }
          setMembers(response.members);
          setNotice(null);
        } catch (error) {
          if (cancelled) {
            return;
          }
          setMembers([]);
          setNotice({
            tone: "error",
            message: formatError(error),
          });
        } finally {
          if (!cancelled) {
            setLoading(false);
          }
        }
      })();
    }, normalizedQuery === "" ? 0 : 180);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [
    authState,
    client,
    enabled,
    limit,
    normalizedQuery,
    normalizedSelectedMemberId,
    selectedGuildID,
  ]);

  return {
    members,
    loading,
    notice,
    clearNotice: () => setNotice(null),
    refresh,
  };
}
