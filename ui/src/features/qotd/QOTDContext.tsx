/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import type { QOTDConfig, QOTDQuestion, QOTDSummary } from "../../api/control";
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
  updateQuestion: "Updating question...",
  deleteQuestion: "Deleting question...",
  reorderQuestions: "Reordering question bank...",
  publishNow: "Publishing manual QOTD...",
} as const;

interface QOTDContextValue {
  busyLabel: string;
  hasLoadedAttempt: boolean;
  loading: boolean;
  notice: Notice | null;
  questions: QOTDQuestion[];
  settings: QOTDConfig;
  summary: QOTDSummary | null;
  workspaceState: WorkspaceState;
  clearNotice: () => void;
  createQuestion: (payload: Pick<QOTDQuestion, "body" | "status">) => Promise<void>;
  deleteQuestion: (questionId: number) => Promise<void>;
  publishNow: () => Promise<void>;
  refreshWorkspace: () => Promise<void>;
  reorderQuestions: (orderedIDs: number[]) => Promise<void>;
  saveSettings: (settings: QOTDConfig) => Promise<void>;
  updateQuestion: (
    questionId: number,
    payload: Pick<QOTDQuestion, "body" | "status">,
  ) => Promise<void>;
}

const emptySettings: QOTDConfig = {
  enabled: false,
  question_channel_id: "",
  response_channel_id: "",
};

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
  const [summary, setSummary] = useState<QOTDSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(false);

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
    setSummary(null);
    setLoading(false);
    setBusyLabel("");
    setNotice(null);
    setHasLoadedAttempt(false);
  }

  async function loadWorkspace() {
    if (!canReadSelectedGuild || normalizedGuildID === "") {
      return;
    }

    setLoading(true);

    try {
      const [settingsResponse, questionsResponse, summaryResponse] = await Promise.all([
        client.getQOTDSettings(normalizedGuildID),
        client.listQOTDQuestions(normalizedGuildID),
        client.getQOTDSummary(normalizedGuildID),
      ]);
      setSettings({
        ...emptySettings,
        ...settingsResponse.settings,
      });
      setQuestions(questionsResponse.questions);
      setSummary(summaryResponse.summary);
      setHasLoadedAttempt(true);
      setNotice(null);
    } catch (error) {
      setHasLoadedAttempt(true);
      setSummary(null);
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
      setBusyLabel("");
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
        const [settingsResponse, questionsResponse, summaryResponse] = await Promise.all([
          client.getQOTDSettings(normalizedGuildID),
          client.listQOTDQuestions(normalizedGuildID),
          client.getQOTDSummary(normalizedGuildID),
        ]);
        if (cancelled) {
          return;
        }
        setSettings({
          ...emptySettings,
          ...settingsResponse.settings,
        });
        setQuestions(questionsResponse.questions);
        setSummary(summaryResponse.summary);
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
  }, [authState, client, normalizedGuildID]);

  async function refreshWorkspace() {
    setBusyLabel(QOTD_BUSY_LABELS.refreshWorkspace);
    await loadWorkspace();
  }

  async function saveSettings(nextSettings: QOTDConfig) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    setBusyLabel(QOTD_BUSY_LABELS.saveSettings);
    try {
      const response = await client.updateQOTDSettings(normalizedGuildID, nextSettings);
      const updatedSettings = {
        ...emptySettings,
        ...response.settings,
      };
      setSettings(updatedSettings);
      setSummary((currentSummary) =>
        currentSummary === null
          ? currentSummary
          : {
              ...currentSummary,
              settings: updatedSettings,
            },
      );
      setNotice(null);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setBusyLabel("");
    }
  }

  async function createQuestion(payload: Pick<QOTDQuestion, "body" | "status">) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    setBusyLabel(QOTD_BUSY_LABELS.createQuestion);
    try {
      await client.createQOTDQuestion(normalizedGuildID, payload);
      await loadWorkspace();
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setBusyLabel("");
    }
  }

  async function updateQuestion(
    questionId: number,
    payload: Pick<QOTDQuestion, "body" | "status">,
  ) {
    if (!canEditSelectedGuild || normalizedGuildID === "") {
      return;
    }

    setBusyLabel(QOTD_BUSY_LABELS.updateQuestion);
    try {
      await client.updateQOTDQuestion(normalizedGuildID, questionId, payload);
      await loadWorkspace();
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
      await loadWorkspace();
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

    setBusyLabel(QOTD_BUSY_LABELS.reorderQuestions);
    try {
      const response = await client.reorderQOTDQuestions(normalizedGuildID, orderedIDs);
      setQuestions(response.questions);
      await loadWorkspace();
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
      await loadWorkspace();
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

  return (
    <QOTDContext.Provider
      value={{
        busyLabel,
        hasLoadedAttempt,
        loading,
        notice,
        questions,
        settings,
        summary,
        workspaceState,
        clearNotice: () => setNotice(null),
        createQuestion,
        deleteQuestion,
        publishNow,
        refreshWorkspace,
        reorderQuestions,
        saveSettings,
        updateQuestion,
      }}
    >
      {children}
    </QOTDContext.Provider>
  );
}

export function useQOTD() {
  const value = useContext(QOTDContext);
  if (value === null) {
    throw new Error("useQOTD must be used within QOTDProvider");
  }
  return value;
}
