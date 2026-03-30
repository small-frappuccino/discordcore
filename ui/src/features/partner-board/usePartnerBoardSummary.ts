import { useEffect, useState } from "react";
import type { PartnerBoardConfig } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import {
  peekPartnerBoard,
  readPartnerBoardCache,
  writePartnerBoardCache,
} from "./cache";
import {
  getPartnerBoardShellStatus,
  isDeliveryConfigured,
  isLayoutConfigured,
  postingMethodLabel,
  summarizePostingDestination,
} from "./model";

export function usePartnerBoardSummary() {
  const { authState, baseUrl, canReadSelectedGuild, client, selectedGuildID } =
    useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const [board, setBoard] = useState<PartnerBoardConfig | null>(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      return null;
    }
    return peekPartnerBoard(baseUrl, normalizedGuildID);
  });
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(board !== null);
  const [lastLoadedAt, setLastLoadedAt] = useState<number | null>(() =>
    readPartnerBoardCache(baseUrl, normalizedGuildID)?.fetchedAt ?? null,
  );
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

    const cachedEntry = readPartnerBoardCache(baseUrl, normalizedGuildID);
    const cachedBoard = cachedEntry?.board ?? null;
    setBoard(cachedBoard);
    setHasLoadedAttempt(cachedBoard !== null);
    setLastLoadedAt(cachedEntry?.fetchedAt ?? null);
    setLoading(cachedBoard === null);

    try {
      const response = await client.getPartnerBoard(normalizedGuildID);
      writePartnerBoardCache(baseUrl, normalizedGuildID, response.partner_board);
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

    const cachedEntry = readPartnerBoardCache(baseUrl, normalizedGuildID);
    const cachedBoard = cachedEntry?.board ?? null;
    setBoard(cachedBoard);
    setHasLoadedAttempt(cachedBoard !== null);
    setLastLoadedAt(cachedEntry?.fetchedAt ?? null);
    setNotice(null);

    let cancelled = false;

    async function autoLoadSummary() {
      setLoading(cachedBoard === null);

      try {
        const response = await client.getPartnerBoard(normalizedGuildID);
        if (cancelled) {
          return;
        }

        writePartnerBoardCache(baseUrl, normalizedGuildID, response.partner_board);
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
