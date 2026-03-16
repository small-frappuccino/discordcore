import { useEffect, useMemo, useState } from "react";
import type { FeatureRecord, FeatureWorkspace } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { groupFeaturesByCategory } from "./model";

export type FeatureWorkspaceScope = "global" | "guild";
export type FeatureWorkspaceState =
  | "auth_required"
  | "checking"
  | "loading"
  | "ready"
  | "server_required"
  | "unavailable";

interface UseFeatureWorkspaceOptions {
  scope: FeatureWorkspaceScope;
}

export function useFeatureWorkspace({
  scope,
}: UseFeatureWorkspaceOptions) {
  const { authState, client, selectedGuildID } = useDashboardSession();
  const [workspace, setWorkspace] = useState<FeatureWorkspace | null>(null);
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(false);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  const features = workspace?.features ?? [];
  const groupedFeatures = useMemo(
    () => groupFeaturesByCategory(features),
    [features],
  );

  let workspaceState: FeatureWorkspaceState = "ready";
  if (authState === "checking") {
    workspaceState = "checking";
  } else if (authState !== "signed_in") {
    workspaceState = "auth_required";
  } else if (scope === "guild" && selectedGuildID.trim() === "") {
    workspaceState = "server_required";
  } else if (workspace !== null) {
    workspaceState = "ready";
  } else if (loading || !hasLoadedAttempt) {
    workspaceState = "loading";
  } else {
    workspaceState = "unavailable";
  }

  function resetWorkspace() {
    setWorkspace(null);
    setHasLoadedAttempt(false);
    setLoading(false);
    setNotice(null);
  }

  async function refresh() {
    if (authState !== "signed_in") {
      return;
    }
    if (scope === "guild" && selectedGuildID.trim() === "") {
      return;
    }

    setLoading(true);

    try {
      const response =
        scope === "guild"
          ? await client.listGuildFeatures(selectedGuildID.trim())
          : await client.listGlobalFeatures();
      setWorkspace(response.workspace);
      setHasLoadedAttempt(true);
      setNotice(null);
    } catch (error) {
      setWorkspace(null);
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
    if (authState !== "signed_in") {
      resetWorkspace();
      return;
    }

    if (scope === "guild" && selectedGuildID.trim() === "") {
      resetWorkspace();
      return;
    }

    let cancelled = false;

    async function loadWorkspace() {
      setLoading(true);

      try {
        const response =
          scope === "guild"
            ? await client.listGuildFeatures(selectedGuildID.trim())
            : await client.listGlobalFeatures();
        if (cancelled) {
          return;
        }
        setWorkspace(response.workspace);
        setHasLoadedAttempt(true);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setWorkspace(null);
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

    void loadWorkspace();

    return () => {
      cancelled = true;
    };
  }, [authState, client, scope, selectedGuildID]);

  return {
    features,
    groupedFeatures,
    loading,
    notice,
    scope,
    workspace,
    workspaceState,
    clearNotice: () => setNotice(null),
    refresh,
  };
}

export function findFeatureRecord(
  workspace: FeatureWorkspace | null,
  featureId: string,
): FeatureRecord | null {
  return workspace?.features.find((feature) => feature.id === featureId) ?? null;
}
