import { useEffect, useId, useRef, useState } from "react";
import type {
  QOTDCollectedQuestion,
  QOTDCollectorConfig,
  QOTDCollectorRunResult,
  QOTDCollectorSummary,
} from "../../api/control";
import type { Notice } from "../../app/types";
import { formatError } from "../../app/utils";
import {
  GroupedSettingsCopy,
  GroupedSettingsGroup,
  GroupedSettingsHeading,
  GroupedSettingsInlineMessage,
  GroupedSettingsItem,
  GroupedSettingsSection,
  GroupedSettingsStack,
  GroupedSettingsSubrow,
  SettingsSelectField,
  UnsavedChangesBar,
} from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useGuildChannelOptions } from "../features/useGuildChannelOptions";
import { useQOTD } from "./QOTDContext";

interface CollectorDraft {
  source_channel_id: string;
  author_ids_text: string;
  title_patterns_text: string;
  start_date: string;
}

const emptyCollectorSummary: QOTDCollectorSummary = {
  total_questions: 0,
  recent_questions: [],
};

export function QOTDCollectorPage() {
  const { authState, canEditSelectedGuild, client, selectedGuildID } =
    useDashboardSession();
  const { saveSettings, settings } = useQOTD();
  const channelOptions = useGuildChannelOptions();
  const collectorHeadingId = useId();
  const actionHeadingId = useId();
  const exportHeadingId = useId();
  const savedDraftRef = useRef<CollectorDraft>(
    createCollectorDraft(settings.collector),
  );
  const [draft, setDraft] = useState<CollectorDraft>(
    () => savedDraftRef.current,
  );
  const [summary, setSummary] = useState<QOTDCollectorSummary>(
    emptyCollectorSummary,
  );
  const [pageNotice, setPageNotice] = useState<Notice | null>(null);
  const [loadingSummary, setLoadingSummary] = useState(false);
  const [saving, setSaving] = useState(false);
  const [collecting, setCollecting] = useState(false);
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    const nextSavedDraft = createCollectorDraft(settings.collector);
    const previousSavedDraft = savedDraftRef.current;
    savedDraftRef.current = nextSavedDraft;
    setDraft((currentDraft) =>
      collectorDraftChanged(previousSavedDraft, currentDraft)
        ? currentDraft
        : nextSavedDraft,
    );
  }, [settings]);

  useEffect(() => {
    if (authState !== "signed_in" || selectedGuildID.trim() === "") {
      setSummary(emptyCollectorSummary);
      setPageNotice(null);
      setLoadingSummary(false);
      return;
    }

    let cancelled = false;
    setLoadingSummary(true);

    void (async () => {
      try {
        const response = await client.getQOTDCollectorSummary(
          selectedGuildID.trim(),
        );
        if (cancelled) {
          return;
        }
        setSummary(normalizeCollectorSummary(response.summary));
        setPageNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSummary(emptyCollectorSummary);
        setPageNotice({
          tone: "error",
          message: formatError(error),
        });
      } finally {
        if (!cancelled) {
          setLoadingSummary(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [authState, client, selectedGuildID]);

  const messageChannelOptions = channelOptions.channels
    .filter((channel) => channel.supports_message_route)
    .map((channel) => ({
      value: channel.id,
      label: channel.display_name,
      description:
        channel.kind === "announcement"
          ? "Announcement channel that contains previous QOTD embeds."
          : "Text channel that contains previous QOTD embeds.",
    }));
  const channelPlaceholder = channelOptions.loading
    ? "Loading message channels..."
    : messageChannelOptions.length === 0
      ? "No message channels available"
      : "Select a source channel";
  const hasUnsavedChanges = collectorDraftChanged(savedDraftRef.current, draft);
  const canRunCollector =
    canEditSelectedGuild &&
    !collecting &&
    !hasUnsavedChanges &&
    draft.source_channel_id.trim() !== "" &&
    parseTitlePatterns(draft.title_patterns_text).length > 0 &&
    selectedGuildID.trim() !== "";

  async function handleSave() {
    if (!canEditSelectedGuild || saving) {
      return;
    }

    setSaving(true);
    try {
      await saveSettings({
        ...settings,
        collector: buildCollectorConfigFromDraft(draft),
      });
    } finally {
      setSaving(false);
    }
  }

  function handleReset() {
    setDraft(savedDraftRef.current);
  }

  async function handleCollect() {
    if (!canRunCollector) {
      return;
    }

    setCollecting(true);
    try {
      const response = await client.runQOTDCollector(selectedGuildID.trim());
      setSummary(normalizeCollectorSummary(response.summary));
      setPageNotice({
        tone: "success",
        message: formatCollectorRunResult(response.result),
      });
    } catch (error) {
      setPageNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setCollecting(false);
    }
  }

  async function handleExport() {
    if (
      selectedGuildID.trim() === "" ||
      summary.total_questions === 0 ||
      exporting
    ) {
      return;
    }

    setExporting(true);
    try {
      const exportFile = await client.downloadQOTDCollectorExport(
        selectedGuildID.trim(),
      );
      const objectURL = window.URL.createObjectURL(
        new Blob([exportFile.text], {
          type: "text/plain;charset=utf-8",
        }),
      );
      const link = document.createElement("a");
      link.href = objectURL;
      link.download = exportFile.filename;
      document.body.append(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(objectURL);
      setPageNotice({
        tone: "success",
        message:
          summary.total_questions === 1
            ? "Downloaded 1 collected question as .txt."
            : `Downloaded ${summary.total_questions} collected questions as .txt.`,
      });
    } catch (error) {
      setPageNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setExporting(false);
    }
  }

  return (
    <div className="workspace-view qotd-workspace">
      <GroupedSettingsStack className="qotd-grouped-stack">
        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Collector</p>
            <GroupedSettingsHeading
              as="h2"
              variant="section"
              id={collectorHeadingId}
            >
              Source settings
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          {channelOptions.notice ? (
            <GroupedSettingsInlineMessage
              message={channelOptions.notice.message}
              tone="error"
              action={
                <button
                  className="button-secondary"
                  type="button"
                  disabled={channelOptions.loading}
                  onClick={() => void channelOptions.refresh()}
                >
                  Retry channel lookup
                </button>
              }
            />
          ) : null}

          <GroupedSettingsGroup>
            <GroupedSettingsItem
              stacked
              role="group"
              aria-labelledby={collectorHeadingId}
            >
              <GroupedSettingsSubrow>
                <SettingsSelectField
                  label="History channel"
                  value={draft.source_channel_id}
                  onChange={(value) =>
                    setDraft((current) => ({
                      ...current,
                      source_channel_id: value,
                    }))
                  }
                  options={messageChannelOptions}
                  placeholder={channelPlaceholder}
                  disabled={
                    !canEditSelectedGuild || saving || channelOptions.loading
                  }
                  note="The bot scans this channel for past QOTD embeds from other bots."
                />
              </GroupedSettingsSubrow>

              <GroupedSettingsSubrow>
                <div className="qotd-composer-grid">
                  <label className="field-stack">
                    <span className="field-label">Allowed author IDs</span>
                    <textarea
                      aria-label="Allowed author IDs"
                      value={draft.author_ids_text}
                      disabled={!canEditSelectedGuild || saving}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          author_ids_text: event.target.value,
                        }))
                      }
                      rows={4}
                      placeholder={"111111111111111111\n222222222222222222"}
                    />
                    <span className="meta-note">
                      One Discord user ID per line. Leave blank to scan all
                      authors in the selected channel.
                    </span>
                  </label>

                  <div className="qotd-composer-side">
                    <label className="field-stack">
                      <span className="field-label">Embed title patterns</span>
                      <textarea
                        aria-label="Embed title patterns"
                        value={draft.title_patterns_text}
                        disabled={!canEditSelectedGuild || saving}
                        onChange={(event) =>
                          setDraft((current) => ({
                            ...current,
                            title_patterns_text: event.target.value,
                          }))
                        }
                        rows={4}
                        placeholder={"Question Of The Day\nquestion!!"}
                      />
                      <span className="meta-note">
                        One pattern per line. Matching is case-insensitive and
                        uses title fragments.
                      </span>
                    </label>

                    <label className="field-stack">
                      <span className="field-label">Earliest message date</span>
                      <input
                        aria-label="Earliest message date"
                        type="date"
                        value={draft.start_date}
                        disabled={!canEditSelectedGuild || saving}
                        onChange={(event) =>
                          setDraft((current) => ({
                            ...current,
                            start_date: event.target.value,
                          }))
                        }
                      />
                      <span className="meta-note">
                        Optional. When set, the collector stops once it reaches
                        older messages.
                      </span>
                    </label>

                    <div className="card-copy">
                      Matching embeds store only the first non-empty description
                      line as the exported question text.
                    </div>
                  </div>
                </div>
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>

        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Collect</p>
            <GroupedSettingsHeading
              as="h2"
              variant="section"
              id={actionHeadingId}
            >
              Scan previous QOTDs
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          <GroupedSettingsGroup>
            <GroupedSettingsItem role="group" aria-labelledby={actionHeadingId}>
              <GroupedSettingsSubrow>
                {pageNotice ? (
                  <GroupedSettingsInlineMessage
                    message={pageNotice.message}
                    tone={pageNotice.tone === "error" ? "error" : "info"}
                  />
                ) : null}

                {hasUnsavedChanges ? (
                  <GroupedSettingsInlineMessage
                    message="Save collector settings before scanning channel history."
                    tone="info"
                  />
                ) : null}

                <div className="qotd-deck-card-footer">
                  <div className="qotd-deck-summary">
                    <span>
                      {loadingSummary
                        ? "Loading collected history..."
                        : summary.total_questions === 1
                          ? "1 collected question stored"
                          : `${summary.total_questions} collected questions stored`}
                    </span>
                    <span>
                      {draft.source_channel_id.trim() === ""
                        ? "No source channel selected"
                        : "Ready to scan saved source channel"}
                    </span>
                  </div>

                  <div className="inline-actions">
                    <button
                      className="button-primary"
                      type="button"
                      disabled={!canRunCollector}
                      onClick={() => void handleCollect()}
                    >
                      {collecting ? "Collecting..." : "Collect questions now"}
                    </button>
                  </div>
                </div>
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>

        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Export</p>
            <GroupedSettingsHeading
              as="h2"
              variant="section"
              id={exportHeadingId}
            >
              Download collected questions
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          <GroupedSettingsGroup>
            <GroupedSettingsItem
              stacked
              role="group"
              aria-labelledby={exportHeadingId}
            >
              <GroupedSettingsSubrow>
                <div className="qotd-deck-card-footer">
                  <div className="card-copy">
                    Export uses the same `.txt` format as question import: one
                    question per line.
                  </div>
                  <div className="inline-actions">
                    <button
                      className="button-secondary"
                      type="button"
                      disabled={summary.total_questions === 0 || exporting}
                      onClick={() => void handleExport()}
                    >
                      {exporting ? "Preparing..." : "Download .txt"}
                    </button>
                  </div>
                </div>
              </GroupedSettingsSubrow>

              <GroupedSettingsSubrow>
                {summary.recent_questions.length === 0 ? (
                  <GroupedSettingsInlineMessage
                    message="No collected questions are stored yet."
                    tone="info"
                  />
                ) : (
                  <div className="qotd-question-list">
                    {summary.recent_questions.map((question) => (
                      <article className="qotd-question-card" key={question.id}>
                        <div className="qotd-question-heading">
                          <div className="qotd-question-order-row">
                            <span className="qotd-question-index">
                              {question.embed_title}
                            </span>
                          </div>
                          <p className="qotd-question-body">
                            {question.question_text}
                          </p>
                        </div>
                        <div className="qotd-question-meta">
                          {buildCollectedQuestionMeta(question).map((item) => (
                            <span key={item}>{item}</span>
                          ))}
                        </div>
                      </article>
                    ))}
                  </div>
                )}
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>
      </GroupedSettingsStack>

      <UnsavedChangesBar
        hasUnsavedChanges={hasUnsavedChanges}
        saveLabel={saving ? "Saving..." : "Save changes"}
        saving={saving}
        disabled={!canEditSelectedGuild || saving}
        onReset={handleReset}
        onSave={handleSave}
      />
    </div>
  );
}

function createCollectorDraft(collector?: QOTDCollectorConfig): CollectorDraft {
  return {
    source_channel_id: String(collector?.source_channel_id ?? "").trim(),
    author_ids_text: Array.isArray(collector?.author_ids)
      ? collector.author_ids
          .map((value) => String(value ?? "").trim())
          .filter((value) => value !== "")
          .join("\n")
      : "",
    title_patterns_text: Array.isArray(collector?.title_patterns)
      ? collector.title_patterns
          .map((value) => String(value ?? "").trim())
          .filter((value) => value !== "")
          .join("\n")
      : "",
    start_date: String(collector?.start_date ?? "").trim(),
  };
}

function collectorDraftChanged(previous: CollectorDraft, next: CollectorDraft) {
  return JSON.stringify(previous) !== JSON.stringify(next);
}

function buildCollectorConfigFromDraft(
  draft: CollectorDraft,
): QOTDCollectorConfig {
  return {
    source_channel_id: draft.source_channel_id.trim(),
    author_ids: parseAuthorIDs(draft.author_ids_text),
    title_patterns: parseTitlePatterns(draft.title_patterns_text),
    start_date: draft.start_date.trim(),
  };
}

function parseAuthorIDs(value: string) {
  return value
    .split(/[\s,]+/)
    .map((entry) => entry.trim())
    .filter((entry) => entry !== "");
}

function parseTitlePatterns(value: string) {
  return value
    .split(/\r?\n/)
    .map((entry) => entry.trim())
    .filter((entry) => entry !== "");
}

function formatCollectorRunResult(result: QOTDCollectorRunResult) {
  return `Scanned ${result.scanned_messages} messages, matched ${result.matched_messages} embeds, and stored ${result.new_questions} new questions. ${result.total_questions} total questions are ready for export.`;
}

function buildCollectedQuestionMeta(question: QOTDCollectedQuestion) {
  const meta = [`Posted ${question.source_created_at.slice(0, 10)}`];
  if (question.source_author_name) {
    meta.push(question.source_author_name);
  } else if (question.source_author_id) {
    meta.push(`Author ${question.source_author_id}`);
  }
  meta.push(`Message ${question.source_message_id}`);
  return meta;
}

function normalizeCollectorSummary(
  summary?: QOTDCollectorSummary | null,
): QOTDCollectorSummary {
  return {
    total_questions: Number(summary?.total_questions ?? 0),
    recent_questions: Array.isArray(summary?.recent_questions)
      ? summary.recent_questions
      : [],
  };
}
