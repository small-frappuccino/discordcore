import { useEffect, useState } from "react";
import type { PartnerBoardConfig } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import {
  getPartnerBoardShellStatus,
  isDeliveryConfigured,
  isLayoutConfigured,
  postingMethodLabel,
  summarizePostingDestination,
} from "./model";

interface CachedPartnerBoardSummary {
  board: PartnerBoardConfig;
  fetchedAt: number;
}

const partnerBoardSummaryCache = new Map<string, CachedPartnerBoardSummary>();

export function usePartnerBoardSummary() {
  const { authState, baseUrl, canReadSelectedGuild, client, selectedGuildID } =
    useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const [board, setBoard] = useState<PartnerBoardConfig | null>(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      return null;
    }
    return peekPartnerBoardSummary(baseUrl, normalizedGuildID);
  });
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(false);
  const [lastLoadedAt, setLastLoadedAt] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  const deliveryConfigured = isDeliveryConfigured(board?.target);
  const layoutConfigured = isLayoutConfigured(board?.template);
  const partnerCount = board?.partners?.length ?? 0;
  const shellStatus = getPartnerBoardShellStatus({
    authState,
    board,
    deliveryConfigured,
    hasLoadedAttempt,
    lastSyncedAt: null,
    layoutConfigured,
    loading,
    partnerCount,
    selectedGuildID,
  });

  function resetSummary() {
    setBoard(null);
    setHasLoadedAttempt(false);
    setLastLoadedAt(null);
    setLoading(false);
    setNotice(null);
  }

  async function loadBoardSummary() {
    if (!canReadSelectedGuild || normalizedGuildID === "") {
      return;
    }

    const cachedBoard = peekPartnerBoardSummary(baseUrl, normalizedGuildID);
    setLoading(cachedBoard === null);

    try {
      const response = await client.getPartnerBoard(normalizedGuildID);
      writePartnerBoardSummaryCache(baseUrl, normalizedGuildID, response.partner_board);
      setBoard(response.partner_board);
      setHasLoadedAttempt(true);
      setLastLoadedAt(Date.now());
      setNotice(null);
    } catch (error) {
      setBoard(cachedBoard);
      setHasLoadedAttempt(true);
      if (cachedBoard === null) {
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
      resetSummary();
      return;
    }

    const cachedBoard = peekPartnerBoardSummary(baseUrl, normalizedGuildID);
    setBoard(cachedBoard);
    setHasLoadedAttempt(cachedBoard !== null);
    setNotice(null);

    let cancelled = false;

    async function autoLoadSummary() {
      setLoading(cachedBoard === null);

      try {
        const response = await client.getPartnerBoard(normalizedGuildID);
        if (cancelled) {
          return;
        }

        writePartnerBoardSummaryCache(baseUrl, normalizedGuildID, response.partner_board);
        setBoard(response.partner_board);
        setHasLoadedAttempt(true);
        setLastLoadedAt(Date.now());
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }

        setBoard(cachedBoard);
        setHasLoadedAttempt(true);
        if (cachedBoard === null) {
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

    void autoLoadSummary();

    return () => {
      cancelled = true;
    };
  }, [authState, baseUrl, client, normalizedGuildID]);

  return {
    board,
    clearNotice: () => setNotice(null),
    deliveryConfigured,
    hasLoadedAttempt,
    lastLoadedAt,
    layoutConfigured,
    loading,
    notice,
    partnerCount,
    postingMethodLabel: postingMethodLabel(board?.target?.type ?? ""),
    refreshBoardSummary: loadBoardSummary,
    shellStatus,
    summarizePostingDestination: summarizePostingDestination(board?.target),
  };
}

function peekPartnerBoardSummary(baseUrl: string, guildID: string) {
  const entry = partnerBoardSummaryCache.get(buildPartnerBoardCacheKey(baseUrl, guildID));
  return entry?.board ?? null;
}

function writePartnerBoardSummaryCache(
  baseUrl: string,
  guildID: string,
  board: PartnerBoardConfig,
) {
  partnerBoardSummaryCache.set(buildPartnerBoardCacheKey(baseUrl, guildID), {
    board,
    fetchedAt: Date.now(),
  });
}

function buildPartnerBoardCacheKey(baseUrl: string, guildID: string) {
  return `${baseUrl}::${guildID.trim()}`;
}
