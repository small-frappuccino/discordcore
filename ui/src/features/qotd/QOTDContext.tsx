/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type {
  QOTDConfig,
  QOTDCollectorConfig,
  QOTDDeck,
  QOTDDeckSummary,
  QOTDQuestion,
  QOTDQuestionMutation,
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
  saveSettings: "Saving QOTD settings...",
  createQuestion: "Creating question...",
  createQuestions: "Importing questions...",
  updateQuestion: "Updating question...",
  deleteQuestion: "Deleting question...",
  reorderQuestions: "Reordering question bank...",
  publishNow: "Publishing manual QOTD...",
} as const;

interface QOTDContextValue {
  busyLabel: string;
  deckSummaries: QOTDDeckSummary[];
  hasLoadedAttempt: boolean;
  loading: boolean;
  notice: Notice | null;
  questions: QOTDQuestion[];
  selectedDeckID: string;
  settings: QOTDConfig;
  summary: QOTDSummary | null;
  workspaceState: WorkspaceState;
  clearNotice: () => void;
  createQuestion: (payload: QOTDQuestionMutation) => Promise<void>;
  createQuestions: (payloads: QOTDQuestionMutation[]) => Promise<boolean>;
  deleteQuestion: (questionId: number) => Promise<void>;
  publishNow: () => Promise<void>;
  refreshWorkspace: () => Promise<void>;
  reorderQuestions: (orderedIDs: number[]) => Promise<void>;
  saveSettings: (settings: QOTDConfig) => Promise<QOTDConfig | null>;
  selectDeck: (deckId: string) => Promise<void>;
  updateQuestion: (
    questionId: number,
    payload: QOTDQuestionMutation,
  ) => Promise<void>;
}

const defaultDeck: QOTDDeck = {
  id: "default",
  name: "Default",
  enabled: false,
  question_channel_id: "",
  response_channel_id: "",
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
  const [questions, setQuestions] = useState<QOTDQuestion[]>([]);
  const [summary, setSummary] = useState<QOTDSummary | null>(emptySummary);
  const [selectedDeckID, setSelectedDeckID] = useState("");
  const [loading, setLoading] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(false);
  const selectedDeckRef = useRef("");

  useEffect(() => {
    selectedDeckRef.current = selectedDeckID.trim();
  }, [selectedDeckID]);

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
    setQuestions([]);
    setSummary(emptySummary);
    setSelectedDeckID("");
    setLoading(false);
    setBusyLabel("");
    setNotice(null);
    setHasLoadedAttempt(false);
  }

  async function loadWorkspace(preferredDeckID = "", background = false) {
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
      const nextDeckID = chooseDeckID(
        preferredDeckID || selectedDeckRef.current,
        nextSettings,
      );
      const questionsResponse = await client.listQOTDQuestions(
        normalizedGuildID,
        nextDeckID,
      );
      setSettings(nextSettings);
      setSummary(normalizeQOTDSummary(summaryResponse.summary, nextSettings));
      setSelectedDeckID(nextDeckID);
      setQuestions(questionsResponse.questions);
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
  }

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
        const nextDeckID = chooseDeckID(selectedDeckRef.current, nextSettings);
        const questionsResponse = await client.listQOTDQuestions(
          normalizedGuildID,
          nextDeckID,
        );
        if (cancelled) {
          return;
        }
        setSettings(nextSettings);
        setSummary(normalizeQOTDSummary(summaryResponse.summary, nextSettings));
        setSelectedDeckID(nextDeckID);
        setQuestions(questionsResponse.questions);
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
    await loadWorkspace(selectedDeckRef.current);
  }

  async function saveSettings(nextSettings: QOTDConfig) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return null;
    }

    setBusyLabel(QOTD_BUSY_LABELS.saveSettings);
    try {
      const response = await client.updateQOTDSettings(
        normalizedGuildID,
        normalizeQOTDSettings(nextSettings),
      );
      const updatedSettings = normalizeQOTDSettings(response.settings);
      setSettings(updatedSettings);
      await loadWorkspace(
        chooseDeckID(selectedDeckRef.current, updatedSettings),
      );
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

  async function createQuestion(payload: QOTDQuestionMutation) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    const targetDeckID = chooseDeckID(
      payload.deck_id ?? selectedDeckRef.current,
      settings,
    );
    if (targetDeckID === "") {
      return;
    }

    setBusyLabel(QOTD_BUSY_LABELS.createQuestion);
    try {
      const response = await client.createQOTDQuestion(normalizedGuildID, {
        ...payload,
        deck_id: targetDeckID,
      });
      if (targetDeckID === selectedDeckRef.current) {
        setQuestions((prev) => [...prev, response.question]);
      }
      void loadWorkspace(targetDeckID, true);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setBusyLabel("");
    }
  }

  async function createQuestions(payloads: QOTDQuestionMutation[]) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return false;
    }

    const normalizedPayloads = payloads
      .map((payload) => {
        const targetDeckID = chooseDeckID(
          payload.deck_id ?? selectedDeckRef.current,
          settings,
        );
        const body = payload.body.trim();
        if (targetDeckID === "" || body === "") {
          return null;
        }
        return {
          ...payload,
          body,
          deck_id: targetDeckID,
        };
      })
      .filter(
        (
          payload,
        ): payload is QOTDQuestionMutation & {
          deck_id: string;
        } => payload !== null,
      );

    if (normalizedPayloads.length === 0) {
      return false;
    }

    const reloadDeckID =
      normalizedPayloads[normalizedPayloads.length - 1]?.deck_id ??
      selectedDeckRef.current;

    setBusyLabel(QOTD_BUSY_LABELS.createQuestions);
    try {
      await client.createQOTDQuestionsBatch(normalizedGuildID, {
        questions: normalizedPayloads,
      });
      await loadWorkspace(reloadDeckID);
      setNotice({
        tone: "success",
        message:
          normalizedPayloads.length === 1
            ? "Created 1 question."
            : `Created ${normalizedPayloads.length} questions.`,
      });
      return true;
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
      return false;
    } finally {
      setBusyLabel("");
    }
  }

  async function updateQuestion(
    questionId: number,
    payload: QOTDQuestionMutation,
  ) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    const targetDeckID =
      payload.deck_id && payload.deck_id.trim() !== ""
        ? payload.deck_id.trim()
        : selectedDeckRef.current;

    setBusyLabel(QOTD_BUSY_LABELS.updateQuestion);
    try {
      const response = await client.updateQOTDQuestion(normalizedGuildID, questionId, payload);
      if (targetDeckID === selectedDeckRef.current) {
        setQuestions((prev) =>
          prev.map((q) => (q.id === questionId ? response.question : q)),
        );
      } else {
        setQuestions((prev) => prev.filter((q) => q.id !== questionId));
      }
      void loadWorkspace(targetDeckID, true);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setBusyLabel("");
    }
  }

  async function deleteQuestion(questionId: number) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    setBusyLabel(QOTD_BUSY_LABELS.deleteQuestion);
    try {
      await client.deleteQOTDQuestion(normalizedGuildID, questionId);
      setQuestions((prev) => prev.filter((q) => q.id !== questionId));
      void loadWorkspace(selectedDeckRef.current, true);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setBusyLabel("");
    }
  }

  async function reorderQuestions(orderedIDs: number[]) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    const targetDeckID = chooseDeckID(selectedDeckRef.current, settings);
    if (targetDeckID === "") {
      return;
    }

    setBusyLabel(QOTD_BUSY_LABELS.reorderQuestions);
    try {
      const response = await client.reorderQOTDQuestions(
        normalizedGuildID,
        targetDeckID,
        orderedIDs,
      );
      setQuestions(response.questions);
      void loadWorkspace(targetDeckID, true);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setBusyLabel("");
    }
  }

  async function publishNow() {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    setBusyLabel(QOTD_BUSY_LABELS.publishNow);
    try {
      const response = await client.publishQOTDNow(normalizedGuildID);
      await loadWorkspace(selectedDeckRef.current);
      setNotice({
        tone: "success",
        message: response.result.post_url
          ? "Manual QOTD published to Discord. Use the post link to verify it."
          : "Manual QOTD published to Discord.",
      });
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setBusyLabel("");
    }
  }

  async function selectDeck(deckId: string) {
    if (!canReadSelectedGuild || normalizedGuildID === "") {
      return;
    }
    const nextDeckID = chooseDeckID(deckId, settings);
    if (nextDeckID === "" || nextDeckID === selectedDeckRef.current) {
      return;
    }

    setLoading(true);
    try {
      const response = await client.listQOTDQuestions(
        normalizedGuildID,
        nextDeckID,
      );
      setSelectedDeckID(nextDeckID);
      setQuestions(response.questions);
      setNotice(null);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
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
        questions,
        selectedDeckID,
        settings,
        summary,
        workspaceState,
        clearNotice: () => setNotice(null),
        createQuestion,
        createQuestions,
        deleteQuestion,
        publishNow,
        refreshWorkspace,
        reorderQuestions,
        saveSettings,
        selectDeck,
        updateQuestion,
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
  return {
    id: id === "" ? defaultDeck.id : id,
    name: name === "" ? defaultDeck.name : name,
    enabled: Boolean(deck.enabled),
    question_channel_id: String(deck.question_channel_id ?? ""),
    response_channel_id: String(deck.response_channel_id ?? ""),
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
