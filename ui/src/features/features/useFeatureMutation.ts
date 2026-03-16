import { useState } from "react";
import type { FeaturePatchPayload, FeatureRecord } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import type { FeatureWorkspaceScope } from "./useFeatureWorkspace";

interface UseFeatureMutationOptions {
  scope: FeatureWorkspaceScope;
  onSuccess?: (feature: FeatureRecord) => void;
  onError?: (message: string) => void;
}

export function useFeatureMutation({
  scope,
  onSuccess,
  onError,
}: UseFeatureMutationOptions) {
  const { authState, client, selectedGuildID } = useDashboardSession();
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  async function patchFeature(
    featureId: string,
    payload: FeaturePatchPayload,
  ): Promise<FeatureRecord | null> {
    if (authState !== "signed_in") {
      const message = "Sign in with Discord before updating feature settings.";
      setNotice({
        tone: "info",
        message,
      });
      onError?.(message);
      return null;
    }

    if (scope === "guild" && selectedGuildID.trim() === "") {
      const message = "Select a server before updating guild feature settings.";
      setNotice({
        tone: "info",
        message,
      });
      onError?.(message);
      return null;
    }

    setSaving(true);

    try {
      const response =
        scope === "guild"
          ? await client.patchGuildFeature(selectedGuildID.trim(), featureId, payload)
          : await client.patchGlobalFeature(featureId, payload);
      setNotice(null);
      onSuccess?.(response.feature);
      return response.feature;
    } catch (error) {
      const message = formatError(error);
      setNotice({
        tone: "error",
        message,
      });
      onError?.(message);
      return null;
    } finally {
      setSaving(false);
    }
  }

  return {
    saving,
    notice,
    clearNotice: () => setNotice(null),
    patchFeature,
  };
}
