import { useEffect, useMemo, useState } from "react";
import type { FeatureRecord, FeatureWorkspace } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { loadFeatureWorkspace, peekFeatureWorkspace } from "./guildResourceCache";
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

const EMPTY_FEATURES: FeatureRecord[] = [];

export function useFeatureWorkspace({
  scope,
}: UseFeatureWorkspaceOptions) {
  const { authState, baseUrl, client, selectedGuildID } = useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const [workspace, setWorkspace] = useState<FeatureWorkspace | null>(() => {
    if (authState !== "signed_in") {
      return null;
    }
    if (scope === "guild" && normalizedGuildID === "") {
      return null;
    }
    return peekFeatureWorkspace(baseUrl, scope, normalizedGuildID);
  });
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(
    workspace !== null,
  );
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  const features = workspace?.features ?? EMPTY_FEATURES;
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

  function updateFeature(nextFeature: FeatureRecord) {
    setWorkspace((currentWorkspace) => {
      if (currentWorkspace === null) {
        return currentWorkspace;
      }

      let featureFound = false;
      const nextFeatures = currentWorkspace.features.map((currentFeature) => {
        if (currentFeature.id !== nextFeature.id) {
          return currentFeature;
        }
        featureFound = true;
        return nextFeature;
      });

      if (!featureFound) {
        return currentWorkspace;
      }

      return {
        ...currentWorkspace,
        features: nextFeatures,
      };
    });
  }

  async function refresh() {
    if (authState !== "signed_in") {
      return;
    }
    if (scope === "guild" && normalizedGuildID === "") {
      return;
    }

    setLoading(true);

    try {
      const nextWorkspace = await loadFeatureWorkspace(
        client,
        baseUrl,
        scope,
        normalizedGuildID,
        {
          force: true,
        },
      );
      setWorkspace(nextWorkspace);
      setHasLoadedAttempt(true);
      setNotice(null);
    } catch (error) {
      const cachedWorkspace = peekFeatureWorkspace(baseUrl, scope, normalizedGuildID);
      setWorkspace(cachedWorkspace);
      setHasLoadedAttempt(true);
      if (cachedWorkspace === null) {
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
    if (authState !== "signed_in") {
      resetWorkspace();
      return;
    }

    if (scope === "guild" && normalizedGuildID === "") {
      resetWorkspace();
      return;
    }

    const cachedWorkspace = peekFeatureWorkspace(baseUrl, scope, normalizedGuildID);
    setWorkspace(cachedWorkspace);
    setHasLoadedAttempt(cachedWorkspace !== null);
    setNotice(null);

    let cancelled = false;

    async function loadWorkspace() {
      setLoading(cachedWorkspace === null);

      try {
        const nextWorkspace = await loadFeatureWorkspace(
          client,
          baseUrl,
          scope,
          normalizedGuildID,
        );
        if (cancelled) {
          return;
        }
        setWorkspace(nextWorkspace);
        setHasLoadedAttempt(true);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        const cachedErrorWorkspace = peekFeatureWorkspace(
          baseUrl,
          scope,
          normalizedGuildID,
        );
        setWorkspace(cachedErrorWorkspace);
        setHasLoadedAttempt(true);
        if (cachedErrorWorkspace === null) {
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

    void loadWorkspace();

    return () => {
      cancelled = true;
    };
  }, [authState, baseUrl, client, normalizedGuildID, scope]);

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
    updateFeature,
  };
}

export function findFeatureRecord(
  workspace: FeatureWorkspace | null,
  featureId: string,
): FeatureRecord | null {
  return workspace?.features.find((feature) => feature.id === featureId) ?? null;
}
