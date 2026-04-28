/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useEffectEvent,
  useState,
  type ReactNode,
} from "react";
import type {
  QOTDConfig,
  QOTDCollectorConfig,
  QOTDDeck,
  QOTDDeckSummary,
  QOTDSummary,
} from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";

type WorkspaceState =
  | "auth_required"
  | "checking"
  | "loading"
  | "ready"
  | "server_required"
  | "unavailable";

export const QOTD_BUSY_LABELS = {
  refreshWorkspace: "Refreshing QOTD workspace...",
} as const;

interface QOTDContextValue {
  busyLabel: string;
  deckSummaries: QOTDDeckSummary[];
  hasLoadedAttempt: boolean;
  loading: boolean;
  notice: Notice | null;
  settings: QOTDConfig;
  workspaceState: WorkspaceState;
  refreshWorkspace: () => Promise<void>;
  saveSettings: (settings: QOTDConfig) => Promise<QOTDConfig | null>;
}

const defaultDeck: QOTDDeck = {
  id: "default",
  name: "Default",
  enabled: false,
  channel_id: "",
};

const emptySettings: QOTDConfig = {
  active_deck_id: defaultDeck.id,
  decks: [defaultDeck],
};

const emptySummary: QOTDSummary | null = null;

const QOTDContext = createContext<QOTDContextValue | null>(null);

export function QOTDProvider({ children }: { children: ReactNode }) {
  const {
    authState,
    canEditSelectedGuild,
    canReadSelectedGuild,
    client,
    selectedGuildID,
  } = useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const [settings, setSettings] = useState<QOTDConfig>(emptySettings);
  const [summary, setSummary] = useState<QOTDSummary | null>(emptySummary);
  const [loading, setLoading] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(false);

  const deckSummaries = summary?.decks ?? [];

  let workspaceState: WorkspaceState = "ready";
  if (authState === "checking") {
    workspaceState = "checking";
  } else if (authState !== "signed_in") {
    workspaceState = "auth_required";
  } else if (normalizedGuildID === "") {
    workspaceState = "server_required";
  } else if (loading && !hasLoadedAttempt) {
    workspaceState = "loading";
  } else if (summary === null && hasLoadedAttempt) {
    workspaceState = "unavailable";
  }

  function resetWorkspace() {
    setSettings(emptySettings);
    setSummary(emptySummary);
    setLoading(false);
    setBusyLabel("");
    setNotice(null);
    setHasLoadedAttempt(false);
  }

  const loadWorkspace = useEffectEvent(async (background = false) => {
    if (!canReadSelectedGuild || normalizedGuildID === "") {
      return;
    }

    if (!background) {
      setLoading(true);
    }

    try {
      const [settingsResponse, summaryResponse] = await Promise.all([
        client.getQOTDSettings(normalizedGuildID),
        client.getQOTDSummary(normalizedGuildID),
      ]);
      const nextSettings = normalizeQOTDSettings(settingsResponse.settings);
      setSettings(nextSettings);
      setSummary(normalizeQOTDSummary(summaryResponse.summary, nextSettings));
      setHasLoadedAttempt(true);
      if (!background) {
        setNotice(null);
      }
    } catch (error) {
      setHasLoadedAttempt(true);
      setSummary(null);
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      if (!background) {
        setLoading(false);
        setBusyLabel("");
      }
    }
  });

  useEffect(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      resetWorkspace();
      return;
    }

    let cancelled = false;
    setLoading(true);
    setBusyLabel("");
    setNotice(null);

    async function autoLoadWorkspace() {
      try {
        const [settingsResponse, summaryResponse] = await Promise.all([
          client.getQOTDSettings(normalizedGuildID),
          client.getQOTDSummary(normalizedGuildID),
        ]);
        const nextSettings = normalizeQOTDSettings(settingsResponse.settings);
        if (cancelled) {
          return;
        }
        setSettings(nextSettings);
        setSummary(normalizeQOTDSummary(summaryResponse.summary, nextSettings));
        setHasLoadedAttempt(true);
        setNotice(null);
      } catch (error) {
        if (!cancelled) {
          setSummary(null);
          setHasLoadedAttempt(true);
          setNotice({
            tone: "error",
            message: formatError(error),
          });
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
          setBusyLabel("");
        }
      }
    }

    void autoLoadWorkspace();

    return () => {
      cancelled = true;
    };
  }, [authState, client, normalizedGuildID, selectedGuildID]);

  async function refreshWorkspace() {
    setBusyLabel(QOTD_BUSY_LABELS.refreshWorkspace);
    await loadWorkspace();
  }

  async function saveSettings(nextSettings: QOTDConfig) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return null;
    }

    try {
      const response = await client.updateQOTDSettings(
        normalizedGuildID,
        normalizeQOTDSettings(nextSettings),
      );
      const updatedSettings = normalizeQOTDSettings(response.settings);
      setSettings(updatedSettings);
      await loadWorkspace();
      setNotice(null);
      return updatedSettings;
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
      return null;
    } finally {
      setBusyLabel("");
    }
  }

  return (
    <QOTDContext.Provider
      value={{
        busyLabel,
        deckSummaries,
        hasLoadedAttempt,
        loading,
        notice,
        settings,
        workspaceState,
        refreshWorkspace,
        saveSettings,
      }}
    >
      {children}
    </QOTDContext.Provider>
  );
}

function normalizeQOTDSummary(
  summary: QOTDSummary,
  settings: QOTDConfig,
): QOTDSummary {
  return {
    ...summary,
    settings,
    decks: Array.isArray(summary.decks) ? summary.decks : [],
  };
}

function normalizeQOTDSettings(settings?: QOTDConfig | null): QOTDConfig {
  const decks =
    Array.isArray(settings?.decks) && settings.decks.length > 0
      ? settings.decks.map(normalizeQOTDDeck)
      : [defaultDeck];
  const activeDeckID = chooseDeckID(String(settings?.active_deck_id ?? ""), {
    active_deck_id: settings?.active_deck_id,
    decks,
  });
  return {
    collector: normalizeQOTDCollectorConfig(settings?.collector),
    verified_role_id: String(settings?.verified_role_id ?? "").trim(),
    active_deck_id: activeDeckID,
    decks,
  };
}

function normalizeQOTDCollectorConfig(
  collector?: QOTDCollectorConfig | null,
): QOTDCollectorConfig {
  return {
    source_channel_id: String(collector?.source_channel_id ?? "").trim(),
    author_ids: normalizeCollectorEntries(collector?.author_ids),
    title_patterns: normalizeCollectorEntries(collector?.title_patterns, {
      caseInsensitive: true,
    }),
    start_date: String(collector?.start_date ?? "").trim(),
  };
}

function normalizeCollectorEntries(
  values: readonly unknown[] | undefined,
  options: { caseInsensitive?: boolean } = {},
) {
  if (!Array.isArray(values) || values.length === 0) {
    return [];
  }

  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const value of values) {
    const trimmed = String(value ?? "").trim();
    if (trimmed === "") {
      continue;
    }
    const key = options.caseInsensitive ? trimmed.toLowerCase() : trimmed;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    normalized.push(trimmed);
  }
  return normalized;
}

function normalizeQOTDDeck(deck: QOTDDeck): QOTDDeck {
  const id = String(deck.id ?? "").trim();
  const name = String(deck.name ?? "").trim();
  const channelID = String(deck.channel_id ?? "").trim();
  return {
    id: id === "" ? defaultDeck.id : id,
    name: name === "" ? defaultDeck.name : name,
    enabled: Boolean(deck.enabled),
    channel_id: channelID,
  };
}

function chooseDeckID(preferredDeckID: string, settings: QOTDConfig): string {
  const decks = settings.decks ?? [];
  if (decks.length === 0) {
    return "";
  }
  const preferred = preferredDeckID.trim();
  if (preferred !== "" && decks.some((deck) => deck.id === preferred)) {
    return preferred;
  }
  const active = String(settings.active_deck_id ?? "").trim();
  if (active !== "" && decks.some((deck) => deck.id === active)) {
    return active;
  }
  return decks[0]?.id ?? "";
}

export function useQOTD() {
  const value = useContext(QOTDContext);
  if (value === null) {
    throw new Error("useQOTD must be used within QOTDProvider");
  }
  return value;
}
