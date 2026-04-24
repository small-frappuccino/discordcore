import { useEffect, useId, useRef, useState } from "react";
import type {
  QOTDCollectedQuestion,
  QOTDCollectorConfig,
  QOTDCollectorRemoveDuplicatesResult,
  QOTDCollectorRunResult,
  QOTDCollectorSummary,
  QOTDDeck,
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
import { QOTD_BUSY_LABELS, useQOTD } from "./QOTDContext";

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
  const { busyLabel, removeCollectorDeckDuplicates, saveSettings, settings } =
    useQOTD();
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
  const [dedupeDeckID, setDedupeDeckID] = useState<string>(() =>
    resolveCollectorDeckID(settings.decks, settings.active_deck_id),
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
    setDedupeDeckID((currentDeckID: string) =>
      resolveCollectorDeckID(
        settings.decks,
        currentDeckID || settings.active_deck_id,
      ),
    );
  }, [settings.active_deck_id, settings.decks]);

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
          ? "Announcement channel with historical QOTD embeds available for import."
          : "Text channel with historical QOTD embeds available for import.",
    }));
  const channelPlaceholder = channelOptions.loading
    ? "Loading message channels..."
    : messageChannelOptions.length === 0
      ? "No message channels available"
      : "Select a source channel";
  const deckOptions = (settings.decks ?? []).map((deck) => ({
    value: deck.id,
    label: deck.name,
    description: deck.enabled
      ? "Enabled deck"
      : "Disabled deck",
  }));
  const deckPlaceholder =
    deckOptions.length === 0 ? "No decks available" : "Select a target deck";
  const selectedDeckName =
    (settings.decks ?? []).find((deck) => deck.id === dedupeDeckID)?.name ??
    "the selected deck";
  const removingDuplicates =
    busyLabel === QOTD_BUSY_LABELS.removeCollectorDeckDuplicates;
  const hasUnsavedChanges = collectorDraftChanged(savedDraftRef.current, draft);
  const canRunCollector =
    canEditSelectedGuild &&
    !collecting &&
    !hasUnsavedChanges &&
    draft.source_channel_id.trim() !== "" &&
    parseTitlePatterns(draft.title_patterns_text).length > 0 &&
    selectedGuildID.trim() !== "";
  const canRemoveDuplicates =
    canEditSelectedGuild &&
    !removingDuplicates &&
    selectedGuildID.trim() !== "" &&
    summary.total_questions > 0 &&
    dedupeDeckID.trim() !== "";

  async function handleSave() {
    if (!canEditSelectedGuild || saving) {
      return;
    }

    setSaving(true);
    try {
      const updatedSettings = await saveSettings({
        ...settings,
        collector: buildCollectorConfigFromDraft(draft),
      });
      if (updatedSettings != null) {
        const nextDraft = createCollectorDraft(updatedSettings.collector);
        savedDraftRef.current = nextDraft;
        setDraft(nextDraft);
      }
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

  async function handleRemoveDuplicates() {
    if (!canRemoveDuplicates) {
      return;
    }

    try {
      const result = await removeCollectorDeckDuplicates(
        dedupeDeckID.trim(),
      );
      if (result == null) {
        return;
      }
      setPageNotice({
        tone: "success",
        message: formatCollectorRemoveDuplicatesResult(
          result,
          selectedDeckName,
        ),
      });
    } catch (error) {
      setPageNotice({
        tone: "error",
        message: formatError(error),
      });
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
                  note="Import past QOTD embeds from this channel. The live discordcore embed, answer button, and daily thread stay outside this importer."
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
                        uses title fragments from historical embeds only.
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
                      This importer reads historical embed text only: it stores
                      the first non-empty description line and ignores the live
                      answer button and daily thread flow.
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
              Import historical QOTD embeds
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
                    message="Save collector settings before importing historical embeds."
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
                        : "Source channel ready"}
                    </span>
                  </div>

                  <div className="inline-actions">
                    <button
                      className="button-primary"
                      type="button"
                      disabled={!canRunCollector}
                      onClick={() => void handleCollect()}
                    >
                      {collecting ? "Importing..." : "Import historical questions"}
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
                <SettingsSelectField
                  label="Target deck"
                  value={dedupeDeckID}
                  onChange={setDedupeDeckID}
                  options={deckOptions}
                  placeholder={deckPlaceholder}
                  disabled={
                    !canEditSelectedGuild ||
                    removingDuplicates ||
                    deckOptions.length === 0
                  }
                  note="Compare the stored collector history against this deck and remove matching mutable cards."
                />
              </GroupedSettingsSubrow>

              <GroupedSettingsSubrow>
                <div className="qotd-deck-card-footer">
                  <div className="card-copy">
                    Export uses the same `.txt` format as question import: one
                    question per line. Duplicate removal compares stored
                    collected questions against the selected deck and deletes
                    matching mutable cards.
                  </div>
                  <div className="inline-actions">
                    <button
                      className="button-danger"
                      type="button"
                      disabled={!canRemoveDuplicates}
                      onClick={() => void handleRemoveDuplicates()}
                    >
                      {removingDuplicates
                        ? "Removing duplicates..."
                        : "Remove deck duplicates"}
                    </button>
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
  return normalizeCollectorEntries(
    value
      .split(/[\s,]+/)
      .map((entry) => entry.trim())
      .filter((entry) => entry !== ""),
  );
}

function parseTitlePatterns(value: string) {
  return normalizeCollectorEntries(
    value
      .split(/\r?\n/)
      .map((entry) => entry.trim())
      .filter((entry) => entry !== ""),
    {
      caseInsensitive: true,
    },
  );
}

function formatCollectorRunResult(result: QOTDCollectorRunResult) {
  return `Scanned ${result.scanned_messages} historical messages, matched ${result.matched_messages} embeds, and stored ${result.new_questions} new questions. ${result.total_questions} total questions are ready for export.`;
}

function formatCollectorRemoveDuplicatesResult(
  result: QOTDCollectorRemoveDuplicatesResult,
  deckName: string,
) {
  if (result.duplicate_questions === 0) {
    return `Scanned ${result.scanned_messages} historical messages, matched ${result.matched_messages} embeds, and found no duplicate questions in ${deckName}.`;
  }

  const keptQuestions = Math.max(
    result.duplicate_questions - result.deleted_questions,
    0,
  );
  if (keptQuestions === 0) {
    return `Scanned ${result.scanned_messages} historical messages, matched ${result.matched_messages} embeds, and removed ${formatCollectorCount(result.deleted_questions, "duplicate question")} from ${deckName}.`;
  }

  return `Scanned ${result.scanned_messages} historical messages, matched ${result.matched_messages} embeds, found ${formatCollectorCount(result.duplicate_questions, "duplicate question")} in ${deckName}, removed ${formatCollectorCount(result.deleted_questions, "duplicate question")}, and kept ${formatCollectorCount(keptQuestions, "scheduled or used question")}.`;
}

function buildCollectedQuestionMeta(question: QOTDCollectedQuestion) {
  const postedLabel =
    typeof question.source_created_at === "string" &&
    question.source_created_at.length >= 10
      ? `Posted ${question.source_created_at.slice(0, 10)}`
      : "Posted date unavailable";
  const meta = [postedLabel];
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
          .filter(
            (question): question is QOTDCollectedQuestion =>
              question !== null && typeof question === "object",
          )
          .map((question) => ({
            id: Number(question.id ?? 0),
            source_channel_id: String(question.source_channel_id ?? "").trim(),
            source_message_id: String(question.source_message_id ?? "").trim(),
            source_author_id: String(question.source_author_id ?? "").trim(),
            source_author_name: String(
              question.source_author_name ?? "",
            ).trim(),
            source_created_at: String(question.source_created_at ?? "").trim(),
            embed_title: String(question.embed_title ?? "").trim(),
            question_text: String(question.question_text ?? "").trim(),
            created_at: String(question.created_at ?? "").trim(),
            updated_at: String(question.updated_at ?? "").trim(),
          }))
          .filter(
            (question) => question.id > 0 && question.question_text !== "",
          )
      : [],
  };
}

function normalizeCollectorEntries(
  values: readonly string[],
  options: { caseInsensitive?: boolean } = {},
) {
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const value of values) {
    const trimmed = value.trim();
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

function resolveCollectorDeckID(
  decks: readonly QOTDDeck[] | undefined,
  preferredDeckID?: string,
): string {
  const availableDecks = Array.isArray(decks) ? decks : [];
  const preferred = preferredDeckID?.trim() ?? "";
  if (
    preferred !== "" &&
    availableDecks.some((deck) => deck.id === preferred)
  ) {
    return preferred;
  }
  return availableDecks[0]?.id ?? "";
}

function formatCollectorCount(count: number, noun: string) {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}
