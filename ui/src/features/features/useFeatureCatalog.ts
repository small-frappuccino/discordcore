import { useEffect, useState } from "react";
import type { FeatureCatalogEntry } from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";

export function useFeatureCatalog() {
  const { authState, client } = useDashboardSession();
  const [catalog, setCatalog] = useState<FeatureCatalogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  function resetCatalog() {
    setCatalog([]);
    setLoading(false);
    setNotice(null);
  }

  async function refresh() {
    if (authState !== "signed_in") {
      return;
    }

    setLoading(true);

    try {
      const response = await client.getFeatureCatalog();
      setCatalog(response.catalog);
      setNotice(null);
    } catch (error) {
      setCatalog([]);
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
      resetCatalog();
      return;
    }

    let cancelled = false;

    async function loadCatalog() {
      setLoading(true);

      try {
        const response = await client.getFeatureCatalog();
        if (cancelled) {
          return;
        }
        setCatalog(response.catalog);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setCatalog([]);
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

    void loadCatalog();

    return () => {
      cancelled = true;
    };
  }, [authState, client]);

  return {
    catalog,
    loading,
    notice,
    clearNotice: () => setNotice(null),
    refresh,
  };
}
