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

export function usePartnerBoardSummary() {
  const { authState, canManageGuild, client, selectedGuildID } =
    useDashboardSession();
  const [board, setBoard] = useState<PartnerBoardConfig | null>(null);
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
    if (!canManageGuild) {
      return;
    }

    setLoading(true);

    try {
      const response = await client.getPartnerBoard(selectedGuildID.trim());
      setBoard(response.partner_board);
      setHasLoadedAttempt(true);
      setLastLoadedAt(Date.now());
      setNotice(null);
    } catch (error) {
      setBoard(null);
      setHasLoadedAttempt(true);
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
      resetSummary();
      return;
    }

    let cancelled = false;

    async function autoLoadSummary() {
      setLoading(true);

      try {
        const response = await client.getPartnerBoard(selectedGuildID.trim());
        if (cancelled) {
          return;
        }

        setBoard(response.partner_board);
        setHasLoadedAttempt(true);
        setLastLoadedAt(Date.now());
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }

        setBoard(null);
        setHasLoadedAttempt(true);
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

    void autoLoadSummary();

    return () => {
      cancelled = true;
    };
  }, [authState, client, selectedGuildID]);

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
