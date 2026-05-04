import { useEffect, useState } from "react";
import type { GuildChannelOption } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import {
  loadGuildChannelOptions,
  peekGuildChannelOptions,
} from "./guildResourceCache";

interface UseGuildChannelOptionsOptions {
  domain?: string;
}

export function useGuildChannelOptions(
  options: UseGuildChannelOptionsOptions = {},
) {
  const { authState, baseUrl, client, selectedGuildID } = useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const normalizedDomain = options.domain?.trim().toLowerCase() ?? "";
  const [channels, setChannels] = useState<GuildChannelOption[]>(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      return [];
    }
    return peekGuildChannelOptions(baseUrl, normalizedGuildID, {
      domain: normalizedDomain,
    });
  });
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  function resetChannelOptions() {
    setChannels([]);
    setLoading(false);
    setNotice(null);
  }

  async function refresh() {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      return;
    }

    setLoading(true);

    try {
      const nextChannels = await loadGuildChannelOptions(
        client,
        baseUrl,
        normalizedGuildID,
        {
          domain: normalizedDomain,
          force: true,
        },
      );
      setChannels(nextChannels);
      setNotice(null);
    } catch (error) {
      const cachedChannels = peekGuildChannelOptions(baseUrl, normalizedGuildID, {
        domain: normalizedDomain,
      });
      setChannels(cachedChannels);
      if (cachedChannels.length === 0) {
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
      resetChannelOptions();
      return;
    }

    const cachedChannels = peekGuildChannelOptions(baseUrl, normalizedGuildID, {
      domain: normalizedDomain,
    });
    setChannels(cachedChannels);
    setNotice(null);

    let cancelled = false;

    async function loadChannelOptions() {
      setLoading(cachedChannels.length === 0);

      try {
        const nextChannels = await loadGuildChannelOptions(
          client,
          baseUrl,
          normalizedGuildID,
          { domain: normalizedDomain },
        );
        if (cancelled) {
          return;
        }
        setChannels(nextChannels);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        const cachedErrorChannels = peekGuildChannelOptions(baseUrl, normalizedGuildID, {
          domain: normalizedDomain,
        });
        setChannels(cachedErrorChannels);
        if (cachedErrorChannels.length === 0) {
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

    void loadChannelOptions();

    return () => {
      cancelled = true;
    };
    }, [authState, baseUrl, client, normalizedDomain, normalizedGuildID]);

  return {
    channels,
    loading,
    notice,
    clearNotice: () => setNotice(null),
    refresh,
  };
}
