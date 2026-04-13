import { useEffect, useId, useRef, useState, type ChangeEvent } from "react";
import type {
  QOTDDeck,
  QOTDQuestion,
  QOTDQuestionStatus,
} from "../../api/control";
import {
  GroupedSettingsCopy,
  GroupedSettingsGroup,
  GroupedSettingsHeading,
  GroupedSettingsInlineMessage,
  GroupedSettingsItem,
  GroupedSettingsSection,
  GroupedSettingsStack,
  GroupedSettingsSubrow,
} from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useQOTD } from "./QOTDContext";

const editableStatuses: QOTDQuestionStatus[] = ["draft", "ready", "disabled"];

export function QOTDQuestionsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const {
    createQuestion,
    createQuestions,
    deckSummaries,
    deleteQuestion,
    questions,
    reorderQuestions,
    selectDeck,
    selectedDeckID,
    settings,
    updateQuestion,
  } = useQOTD();
  const composerHeadingId = useId();
  const importInputId = useId();
  const queueHeadingId = useId();
  const availableDecks = settings.decks ?? [];
  const selectedDeck = availableDecks.find((deck) => deck.id === selectedDeckID) ?? null;
  const selectedDeckSummary =
    deckSummaries.find((deck) => deck.id === selectedDeckID) ?? null;
  const orderedQuestions = [...questions].sort((left, right) => {
    if (left.queue_position !== right.queue_position) {
      return left.queue_position - right.queue_position;
    }
    return left.id - right.id;
  });
  const [draftBody, setDraftBody] = useState("");
  const [draftStatus, setDraftStatus] = useState<QOTDQuestionStatus>("ready");
  const [importError, setImportError] = useState("");
  const [importFileName, setImportFileName] = useState("");
  const [importedQuestions, setImportedQuestions] = useState<string[]>([]);
  const [editingQuestionID, setEditingQuestionID] = useState<number | null>(null);
  const [editingBody, setEditingBody] = useState("");
  const [editingStatus, setEditingStatus] = useState<QOTDQuestionStatus>("ready");
  const [editingDeckID, setEditingDeckID] = useState("");
  const importInputRef = useRef<HTMLInputElement | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (editingQuestionID === null) {
      return;
    }

    const match = orderedQuestions.find((question) => question.id === editingQuestionID);
    if (!match) {
      setEditingQuestionID(null);
      setEditingBody("");
      setEditingStatus("ready");
      setEditingDeckID("");
    }
  }, [editingQuestionID, orderedQuestions]);

  async function handleCreate() {
    if (!canEditSelectedGuild || draftBody.trim() === "" || selectedDeckID.trim() === "") {
      return;
    }

    setSubmitting(true);
    try {
      await createQuestion({
        deck_id: selectedDeckID,
        body: draftBody,
        status: draftStatus,
      });
      setDraftBody("");
      setDraftStatus("ready");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleImportFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    if (file === null) {
      resetImportedQuestions();
      return;
    }

    try {
      const text = await file.text();
      const questionsFromFile = parseImportedQuestions(text);
      setImportFileName(file.name);
      setImportedQuestions(questionsFromFile);
      setImportError(
        questionsFromFile.length === 0
          ? "This text file does not contain any questions yet."
          : "",
      );
    } catch {
      setImportFileName(file.name);
      setImportedQuestions([]);
      setImportError("Couldn't read this text file. Upload a plain .txt document.");
    }
  }

  async function handleImport() {
    if (
      !canEditSelectedGuild ||
      importedQuestions.length === 0 ||
      selectedDeckID.trim() === ""
    ) {
      return;
    }

    setSubmitting(true);
    try {
      const imported = await createQuestions(
        importedQuestions.map((body) => ({
          deck_id: selectedDeckID,
          body,
          status: draftStatus,
        })),
      );
      if (imported) {
        resetImportedQuestions();
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function handleUpdate() {
    if (
      !canEditSelectedGuild ||
      editingQuestionID === null ||
      editingBody.trim() === "" ||
      editingDeckID.trim() === ""
    ) {
      return;
    }

    setSubmitting(true);
    try {
      await updateQuestion(editingQuestionID, {
        deck_id: editingDeckID,
        body: editingBody,
        status: editingStatus,
      });
      setEditingQuestionID(null);
      setEditingBody("");
      setEditingStatus("ready");
      setEditingDeckID("");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(questionID: number) {
    if (!canEditSelectedGuild) {
      return;
    }
    setSubmitting(true);
    try {
      await deleteQuestion(questionID);
      if (editingQuestionID === questionID) {
        setEditingQuestionID(null);
        setEditingBody("");
        setEditingStatus("ready");
        setEditingDeckID("");
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function moveQuestion(questionID: number, direction: -1 | 1) {
    if (!canEditSelectedGuild) {
      return;
    }

    const currentOrder = orderedQuestions.map((question) => question.id);
    const index = currentOrder.indexOf(questionID);
    const targetIndex = index + direction;
    if (index < 0 || targetIndex < 0 || targetIndex >= currentOrder.length) {
      return;
    }

    const nextOrder = [...currentOrder];
    [nextOrder[index], nextOrder[targetIndex]] = [
      nextOrder[targetIndex],
      nextOrder[index],
    ];

    setSubmitting(true);
    try {
      await reorderQuestions(nextOrder);
    } finally {
      setSubmitting(false);
    }
  }

  function resetImportedQuestions() {
    setImportError("");
    setImportFileName("");
    setImportedQuestions([]);
    if (importInputRef.current) {
      importInputRef.current.value = "";
    }
  }

  return (
    <div className="workspace-view qotd-workspace">
      <GroupedSettingsStack className="qotd-grouped-stack">
        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Deck</p>
            <GroupedSettingsHeading as="h2" variant="section" id={composerHeadingId}>
              Question source
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          <GroupedSettingsGroup>
            <GroupedSettingsItem role="group" aria-labelledby={composerHeadingId}>
              <GroupedSettingsSubrow>
                <div className="qotd-deck-toolbar">
                  <label className="field-stack">
                    <span className="field-label">Selected deck</span>
                    <select
                      value={selectedDeckID}
                      disabled={submitting || availableDecks.length === 0}
                      onChange={(event) => void selectDeck(event.target.value)}
                    >
                      {availableDecks.map((deck) => (
                        <option key={deck.id} value={deck.id}>
                          {deck.name}
                        </option>
                      ))}
                    </select>
                  </label>

                  {selectedDeckSummary ? (
                    <div className="qotd-deck-summary">
                      <span>{selectedDeckSummary.cards_remaining} cards remaining</span>
                      <span>{selectedDeckSummary.counts.used} used</span>
                      <span>
                        {selectedDeckSummary.enabled ? "Enabled deck" : "Disabled deck"}
                      </span>
                    </div>
                  ) : null}
                </div>
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>

        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Question bank</p>
            <GroupedSettingsHeading as="h2" variant="section" id={queueHeadingId}>
              Add a question
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          <GroupedSettingsGroup>
            <GroupedSettingsItem role="group" aria-labelledby={queueHeadingId}>
              <GroupedSettingsSubrow>
                {selectedDeck === null ? (
                  <GroupedSettingsInlineMessage
                    message="Create a deck in Settings before adding questions."
                    tone="info"
                  />
                ) : (
                  <div className="qotd-composer-grid">
                    <label className="field-stack">
                      <span className="field-label">Question text</span>
                      <textarea
                        value={draftBody}
                        disabled={!canEditSelectedGuild || submitting}
                        onChange={(event) => setDraftBody(event.target.value)}
                        placeholder={`Write the next question for ${selectedDeck.name}`}
                        rows={4}
                      />
                    </label>

                    <div className="qotd-composer-side">
                      <label className="field-stack">
                        <span className="field-label">Initial status</span>
                        <select
                          value={draftStatus}
                          disabled={!canEditSelectedGuild || submitting}
                          onChange={(event) =>
                            setDraftStatus(event.target.value as QOTDQuestionStatus)
                          }
                        >
                          {editableStatuses.map((status) => (
                            <option key={status} value={status}>
                              {formatStatusLabel(status)}
                            </option>
                          ))}
                        </select>
                      </label>

                      <label className="field-stack" htmlFor={importInputId}>
                        <span className="field-label">Import from .txt</span>
                        <input
                          id={importInputId}
                          ref={importInputRef}
                          type="file"
                          accept=".txt,text/plain"
                          disabled={!canEditSelectedGuild || submitting}
                          onChange={(event) => void handleImportFileChange(event)}
                        />
                      </label>

                      <div className="card-copy">
                        Each non-empty line becomes one question card in the selected
                        deck.
                      </div>

                      {importFileName !== "" ? (
                        <div className="card-copy">
                          <strong>{importFileName}</strong>
                          <br />
                          {importedQuestions.length === 1
                            ? "1 question ready to import."
                            : `${importedQuestions.length} questions ready to import.`}
                        </div>
                      ) : null}

                      {importError !== "" ? (
                        <GroupedSettingsInlineMessage message={importError} tone="error" />
                      ) : null}

                      <div className="inline-actions">
                        <button
                          className="button-primary"
                          type="button"
                          disabled={
                            !canEditSelectedGuild ||
                            submitting ||
                            draftBody.trim() === "" ||
                            selectedDeckID.trim() === ""
                          }
                          onClick={() => void handleCreate()}
                        >
                          {submitting ? "Saving..." : "Add question"}
                        </button>

                        <button
                          className="button-secondary"
                          type="button"
                          disabled={
                            !canEditSelectedGuild ||
                            submitting ||
                            importedQuestions.length === 0 ||
                            selectedDeckID.trim() === ""
                          }
                          onClick={() => void handleImport()}
                        >
                          {submitting ? "Importing..." : "Import .txt"}
                        </button>
                      </div>
                    </div>
                  </div>
                )}
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>

        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Question</p>
            <GroupedSettingsHeading as="h2" variant="section">
              Question order
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          <GroupedSettingsGroup>
            <GroupedSettingsItem role="group" aria-labelledby={queueHeadingId}>
              <GroupedSettingsSubrow>
                {orderedQuestions.length === 0 ? (
                  <GroupedSettingsInlineMessage
                    message="No questions have been added for this deck yet."
                    tone="info"
                  />
                ) : (
                  <div className="qotd-question-list">
                    {orderedQuestions.map((question, index) => {
                      const mutable = canMutateQuestion(question);
                      const editing = editingQuestionID === question.id;

                      return (
                        <article
                          className={`qotd-question-card${editing ? " is-editing" : ""}`}
                          key={question.id}
                        >
                          <div className="qotd-question-top">
                            <div className="qotd-question-heading">
                              <div className="qotd-question-order-row">
                                <span className="qotd-question-index">
                                  Question #{question.queue_position}
                                </span>
                                <span
                                  className={`qotd-status-pill ${getQuestionToneClass(question.status)}`}
                                >
                                  {formatStatusLabel(question.status)}
                                </span>
                              </div>
                              <p className="qotd-question-body">{question.body}</p>
                            </div>

                            <div className="inline-actions qotd-question-actions">
                              <button
                                className="button-secondary"
                                type="button"
                                disabled={!canEditSelectedGuild || submitting || index === 0}
                                onClick={() => void moveQuestion(question.id, -1)}
                              >
                                Move up
                              </button>
                              <button
                                className="button-secondary"
                                type="button"
                                disabled={
                                  !canEditSelectedGuild ||
                                  submitting ||
                                  index === orderedQuestions.length - 1
                                }
                                onClick={() => void moveQuestion(question.id, 1)}
                              >
                                Move down
                              </button>
                              <button
                                className="button-secondary"
                                type="button"
                                disabled={!canEditSelectedGuild || submitting || !mutable}
                                onClick={() => {
                                  setEditingQuestionID(question.id);
                                  setEditingBody(question.body);
                                  setEditingStatus(normalizeEditableStatus(question.status));
                                  setEditingDeckID(question.deck_id);
                                }}
                              >
                                Edit
                              </button>
                              <button
                                className="button-secondary"
                                type="button"
                                disabled={!canEditSelectedGuild || submitting || !mutable}
                                onClick={() => void handleDelete(question.id)}
                              >
                                Delete
                              </button>
                            </div>
                          </div>

                          <div className="qotd-question-meta">
                            {buildQuestionMeta(question, availableDecks).map((item) => (
                              <span key={item}>{item}</span>
                            ))}
                          </div>

                          {editing ? (
                            <div className="qotd-question-editor">
                              <label className="field-stack">
                                <span className="field-label">Question text</span>
                                <textarea
                                  value={editingBody}
                                  disabled={submitting}
                                  onChange={(event) => setEditingBody(event.target.value)}
                                  rows={4}
                                />
                              </label>

                              <label className="field-stack">
                                <span className="field-label">Deck</span>
                                <select
                                  value={editingDeckID}
                                  disabled={submitting}
                                  onChange={(event) => setEditingDeckID(event.target.value)}
                                >
                                  {availableDecks.map((deck) => (
                                    <option key={deck.id} value={deck.id}>
                                      {deck.name}
                                    </option>
                                  ))}
                                </select>
                              </label>

                              <label className="field-stack">
                                <span className="field-label">Status</span>
                                <select
                                  value={editingStatus}
                                  disabled={submitting}
                                  onChange={(event) =>
                                    setEditingStatus(event.target.value as QOTDQuestionStatus)
                                  }
                                >
                                  {editableStatuses.map((status) => (
                                    <option key={status} value={status}>
                                      {formatStatusLabel(status)}
                                    </option>
                                  ))}
                                </select>
                              </label>

                              <div className="inline-actions">
                                <button
                                  className="button-primary"
                                  type="button"
                                  disabled={
                                    submitting ||
                                    editingBody.trim() === "" ||
                                    editingDeckID.trim() === ""
                                  }
                                  onClick={() => void handleUpdate()}
                                >
                                  {submitting ? "Saving..." : "Save changes"}
                                </button>
                                <button
                                  className="button-secondary"
                                  type="button"
                                  disabled={submitting}
                                  onClick={() => {
                                    setEditingQuestionID(null);
                                    setEditingBody("");
                                    setEditingStatus("ready");
                                    setEditingDeckID("");
                                  }}
                                >
                                  Cancel
                                </button>
                              </div>
                            </div>
                          ) : null}
                        </article>
                      );
                    })}
                  </div>
                )}
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>
      </GroupedSettingsStack>
    </div>
  );
}

function parseImportedQuestions(text: string) {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

function canMutateQuestion(question: QOTDQuestion) {
  return question.status === "draft" || question.status === "ready" || question.status === "disabled";
}

function normalizeEditableStatus(status: QOTDQuestionStatus) {
  if (status === "draft" || status === "ready" || status === "disabled") {
    return status;
  }
  return "ready";
}

function formatStatusLabel(status: QOTDQuestionStatus) {
  switch (status) {
    case "draft":
      return "Draft";
    case "ready":
      return "Ready";
    case "reserved":
      return "Reserved";
    case "used":
      return "Used";
    case "disabled":
      return "Disabled";
    default:
      return status;
  }
}

function getQuestionToneClass(status: QOTDQuestionStatus) {
  switch (status) {
    case "ready":
      return "qotd-status-success";
    case "reserved":
      return "qotd-status-warning";
    case "used":
      return "qotd-status-info";
    case "disabled":
      return "qotd-status-error";
    case "draft":
    default:
      return "qotd-status-neutral";
  }
}

function buildQuestionMeta(question: QOTDQuestion, decks: QOTDDeck[]) {
  const meta: string[] = [];
  const deck = decks.find((entry) => entry.id === question.deck_id);
  if (deck) {
    meta.push(`Deck ${deck.name}`);
  }
  if (question.scheduled_for_date_utc) {
    meta.push(`Scheduled ${question.scheduled_for_date_utc.slice(0, 10)}`);
  }
  if (question.used_at) {
    meta.push(`Used ${question.used_at.slice(0, 10)}`);
  }
  meta.push(`Updated ${question.updated_at.slice(0, 10)}`);
  return meta;
}
