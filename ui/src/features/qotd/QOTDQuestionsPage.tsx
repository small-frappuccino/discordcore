import { useEffect, useState } from "react";
import type { QOTDQuestion, QOTDQuestionStatus } from "../../api/control";
import { SurfaceCard } from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useQOTD } from "./QOTDContext";

const editableStatuses: QOTDQuestionStatus[] = ["draft", "ready", "disabled"];

export function QOTDQuestionsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const { createQuestion, deleteQuestion, questions, reorderQuestions, updateQuestion } =
    useQOTD();
  const orderedQuestions = [...questions].sort((left, right) => {
    if (left.queue_position !== right.queue_position) {
      return left.queue_position - right.queue_position;
    }
    return left.id - right.id;
  });
  const [draftBody, setDraftBody] = useState("");
  const [draftStatus, setDraftStatus] = useState<QOTDQuestionStatus>("ready");
  const [editingQuestionID, setEditingQuestionID] = useState<number | null>(null);
  const [editingBody, setEditingBody] = useState("");
  const [editingStatus, setEditingStatus] = useState<QOTDQuestionStatus>("ready");
  const [submitting, setSubmitting] = useState(false);
  const queueCounts = countQuestionsByStatus(orderedQuestions);

  useEffect(() => {
    if (editingQuestionID === null) {
      return;
    }

    const match = orderedQuestions.find((question) => question.id === editingQuestionID);
    if (!match) {
      setEditingQuestionID(null);
      setEditingBody("");
      setEditingStatus("ready");
    }
  }, [editingQuestionID, orderedQuestions]);

  async function handleCreate() {
    if (!canEditSelectedGuild || draftBody.trim() === "") {
      return;
    }

    setSubmitting(true);
    try {
      await createQuestion({
        body: draftBody,
        status: draftStatus,
      });
      setDraftBody("");
      setDraftStatus("ready");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleUpdate() {
    if (!canEditSelectedGuild || editingQuestionID === null || editingBody.trim() === "") {
      return;
    }

    setSubmitting(true);
    try {
      await updateQuestion(editingQuestionID, {
        body: editingBody,
        status: editingStatus,
      });
      setEditingQuestionID(null);
      setEditingBody("");
      setEditingStatus("ready");
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

  return (
    <div className="qotd-questions-layout">
      <SurfaceCard className="qotd-panel-card">
        <div className="qotd-card-head">
          <div className="card-copy">
            <p className="section-label">Question bank</p>
            <h3>Add a question</h3>
            <p className="section-description">
              New items join the ordered queue and stay editable while they are draft, ready, or disabled.
            </p>
          </div>
          <div className="qotd-chip-row">
            <span className="meta-pill subtle-pill">{queueCounts.ready} ready</span>
            <span className="meta-pill subtle-pill">{queueCounts.draft} draft</span>
            <span className="meta-pill subtle-pill">{queueCounts.reserved} reserved</span>
          </div>
        </div>

        <div className="qotd-composer-grid">
          <label className="field-stack">
            <span className="field-label">Question text</span>
            <textarea
              value={draftBody}
              disabled={!canEditSelectedGuild || submitting}
              onChange={(event) => setDraftBody(event.target.value)}
              placeholder="Write the next question of the day"
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

            <div className="qotd-support-card">
              <p className="section-label">Queue total</p>
              <strong>{orderedQuestions.length}</strong>
              <p className="meta-note">
                Reserved and used items stay visible so the queue keeps its full audit trail.
              </p>
            </div>

            <div className="inline-actions">
              <button
                className="button-primary"
                type="button"
                disabled={!canEditSelectedGuild || submitting || draftBody.trim() === ""}
                onClick={() => void handleCreate()}
              >
                {submitting ? "Saving..." : "Add question"}
              </button>
            </div>
          </div>
        </div>
      </SurfaceCard>

      <SurfaceCard className="qotd-panel-card">
        <div className="qotd-card-head">
          <div className="card-copy">
            <p className="section-label">Queue</p>
            <h3>Ordered questions</h3>
            <p className="section-description">
              Keep ready items near the front and preserve reserved or used history for auditability.
            </p>
          </div>
          <div className="qotd-card-meta">
            <span className="meta-pill subtle-pill">{orderedQuestions.length} total</span>
          </div>
        </div>

        {orderedQuestions.length === 0 ? (
          <div className="qotd-inline-note">
            <p className="meta-note">No questions have been added yet.</p>
          </div>
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
                        <span className="qotd-question-index">Queue #{question.queue_position}</span>
                        <span className={`qotd-status-pill ${getQuestionToneClass(question.status)}`}>
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
                    {buildQuestionMeta(question).map((item) => (
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
                          disabled={submitting || editingBody.trim() === ""}
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
      </SurfaceCard>
    </div>
  );
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

function buildQuestionMeta(question: QOTDQuestion) {
  const meta: string[] = [];
  meta.push(`Status ${formatStatusLabel(question.status)}`);
  if (question.scheduled_for_date_utc) {
    meta.push(`Scheduled ${question.scheduled_for_date_utc.slice(0, 10)}`);
  }
  if (question.used_at) {
    meta.push(`Used ${question.used_at.slice(0, 10)}`);
  }
  meta.push(`Updated ${question.updated_at.slice(0, 10)}`);
  return meta;
}

function countQuestionsByStatus(questions: QOTDQuestion[]) {
  return questions.reduce(
    (counts, question) => {
      counts[question.status] += 1;
      return counts;
    },
    {
      draft: 0,
      ready: 0,
      reserved: 0,
      used: 0,
      disabled: 0,
    },
  );
}
