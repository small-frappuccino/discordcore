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
    <div className="control-panel-grid">
      <SurfaceCard className="control-panel-card control-panel-card-wide">
        <div className="card-copy">
          <p className="section-label">Question bank</p>
          <h2>Add the next QOTD</h2>
          <p className="section-description">
            Questions are published in queue order through the backend reservation flow. Reserved and used items are kept visible for auditability, but only draft, ready, and disabled items remain editable.
          </p>
        </div>

        <div className="surface-subsection">
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
      </SurfaceCard>

      <SurfaceCard className="control-panel-card control-panel-card-wide">
        <div className="card-copy">
          <p className="section-label">Ordered queue</p>
          <h2>Review and reorder the question bank</h2>
          <p className="section-description">
            Use move actions to reorder the full question list. The backend still persists order through a single ordered-id mutation, not direct queue position writes.
          </p>
        </div>

        {orderedQuestions.length === 0 ? (
          <p className="meta-note">No questions have been added yet.</p>
        ) : (
          <div className="qotd-question-list">
            {orderedQuestions.map((question, index) => {
              const mutable = canMutateQuestion(question);
              const editing = editingQuestionID === question.id;

              return (
                <article className="surface-subsection" key={question.id}>
                  <div className="card-copy">
                    <p className="section-label">Queue #{question.queue_position}</p>
                    <h3>{question.body}</h3>
                    <p className="meta-note">
                      Status: {formatStatusLabel(question.status)}
                      {question.scheduled_for_date_utc
                        ? ` · Scheduled ${question.scheduled_for_date_utc.slice(0, 10)}`
                        : ""}
                      {question.used_at
                        ? ` · Used ${question.used_at.slice(0, 10)}`
                        : ""}
                    </p>
                  </div>

                  <div className="inline-actions">
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

                  {editing ? (
                    <div className="surface-subsection">
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
