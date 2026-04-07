import {
  KeyValueList,
  StatusBadge,
  UnsavedChangesBar,
} from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import {
  buildDeliveryPayload,
  getDeliveryChecklist,
  getDeliveryGuidance,
  isDeliveryConfigured,
  postingMethodLabel,
} from "./model";
import { PartnerBoardWorkspaceState } from "./PartnerBoardWorkspaceState";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardDeliveryPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const {
    deliveryDirty,
    deliveryForm,
    loading,
    resetDeliveryForm,
    saveDelivery,
    setDeliveryFormField,
    workspaceState,
  } = usePartnerBoard();

  if (workspaceState !== "ready") {
    return <PartnerBoardWorkspaceState />;
  }

  const currentPayload = buildDeliveryPayload(deliveryForm);
  const destinationReady = isDeliveryConfigured(currentPayload);
  const methodLabel = postingMethodLabel(deliveryForm.type);
  const checklist = getDeliveryChecklist(deliveryForm);

  return (
    <section className="workspace-view">
      <div className="workspace-view-header">
        <div className="card-copy">
          <p className="section-label">Destination</p>
          <h2>Set where the board is published</h2>
          <p className="section-description">
            Keep the posting method and raw destination identifiers in the same
            page so the board can be fully configured without leaving this workspace.
          </p>
        </div>
        <div className="workspace-view-meta">
          <StatusBadge tone={destinationReady ? "success" : "info"}>
            {destinationReady ? "Destination ready" : "Needs destination setup"}
          </StatusBadge>
        </div>
      </div>

      <KeyValueList
        className="workspace-status-list"
        items={[
          {
            label: "Posting method",
            value: methodLabel,
          },
          {
            label: "Current setup",
            value: getDeliveryGuidance(deliveryForm, destinationReady),
          },
          {
            label: "Access",
            value: canEditSelectedGuild ? "Writable" : "Read-only",
          },
        ]}
      />

      <UnsavedChangesBar
        hasUnsavedChanges={deliveryDirty}
        saveLabel={loading ? "Saving..." : "Save changes"}
        saving={loading}
        disabled={!canEditSelectedGuild}
        onReset={resetDeliveryForm}
        onSave={saveDelivery}
      />

      <section className="workspace-callout">
        <div className="card-copy">
          <p className="section-label">Posting method</p>
          <h3>{methodLabel}</h3>
          <p className="section-description">
            Choose the delivery style and fill the exact Discord identifiers used to publish the board.
          </p>
        </div>

        <div className="workspace-callout-copy partner-board-delivery-form">
          <label className="field-stack field-stack-compact">
            <span className="field-label">Posting method</span>
            <select
              aria-label="Posting method"
              value={deliveryForm.type}
              disabled={!canEditSelectedGuild || loading}
              onChange={(event) =>
                setDeliveryFormField("type", event.target.value)
              }
            >
              <option value="channel_message">Channel message</option>
              <option value="webhook_message">Webhook message</option>
            </select>
          </label>

          <label className="field-stack field-stack-compact">
            <span className="field-label">Board message ID</span>
            <input
              aria-label="Board message ID"
              value={deliveryForm.messageID}
              disabled={!canEditSelectedGuild || loading}
              onChange={(event) => setDeliveryFormField("messageID", event.target.value)}
              placeholder="Discord message ID"
            />
          </label>

          {deliveryForm.type === "webhook_message" ? (
            <label className="field-stack field-stack-compact">
              <span className="field-label">Webhook URL</span>
              <input
                aria-label="Webhook URL"
                value={deliveryForm.webhookURL}
                disabled={!canEditSelectedGuild || loading}
                onChange={(event) =>
                  setDeliveryFormField("webhookURL", event.target.value)
                }
                placeholder="https://discord.com/api/webhooks/..."
              />
            </label>
          ) : (
            <label className="field-stack field-stack-compact">
              <span className="field-label">Channel ID</span>
              <input
                aria-label="Channel ID"
                value={deliveryForm.channelID}
                disabled={!canEditSelectedGuild || loading}
                onChange={(event) =>
                  setDeliveryFormField("channelID", event.target.value)
                }
                placeholder="Discord channel ID"
              />
            </label>
          )}

          {!canEditSelectedGuild ? (
            <p className="meta-note">
              This server is open in read-only mode. Destination changes are disabled.
            </p>
          ) : null}
        </div>
      </section>

      <section className="workspace-checklist-panel">
        <div className="card-copy">
          <p className="section-label">Setup checklist</p>
          <h3>{methodLabel}</h3>
          <p className="section-description">
            {getDeliveryGuidance(deliveryForm, destinationReady)}
          </p>
        </div>
        <ul className="checklist">
          {checklist.map((item) => (
            <li key={item.label} className={item.complete ? "is-complete" : "is-pending"}>
              <span className="checklist-mark" aria-hidden="true">
                {item.complete ? "Done" : "Next"}
              </span>
              <span>{item.label}</span>
            </li>
          ))}
        </ul>
      </section>
    </section>
  );
}
