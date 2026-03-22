import { useEffect, useState } from "react";
import type { GuildChannelOption } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";

export function useGuildChannelOptions() {
  const { authState, client, selectedGuildID } = useDashboardSession();
  const [channels, setChannels] = useState<GuildChannelOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  function resetChannelOptions() {
    setChannels([]);
    setLoading(false);
    setNotice(null);
  }

  async function refresh() {
    if (authState !== "signed_in" || selectedGuildID.trim() === "") {
      return;
    }

    setLoading(true);

    try {
      const response = await client.listGuildChannelOptions(selectedGuildID.trim());
      setChannels(response.channels);
      setNotice(null);
    } catch (error) {
      setChannels([]);
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
      resetChannelOptions();
      return;
    }

    let cancelled = false;

    async function loadChannelOptions() {
      setLoading(true);

      try {
        const response = await client.listGuildChannelOptions(selectedGuildID.trim());
        if (cancelled) {
          return;
        }
        setChannels(response.channels);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setChannels([]);
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

    void loadChannelOptions();

    return () => {
      cancelled = true;
    };
  }, [authState, client, selectedGuildID]);

  return {
    channels,
    loading,
    notice,
    clearNotice: () => setNotice(null),
    refresh,
  };
}
