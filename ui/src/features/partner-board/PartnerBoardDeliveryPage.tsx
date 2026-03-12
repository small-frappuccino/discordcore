import { PartnerBoardWorkspaceState } from "./PartnerBoardWorkspaceState";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardDeliveryPage() {
  const {
    deliveryForm,
    loading,
    saveDelivery,
    setDeliveryFormField,
    workspaceState,
  } = usePartnerBoard();

  if (workspaceState !== "ready") {
    return <PartnerBoardWorkspaceState />;
  }

  const usesWebhook = deliveryForm.type === "webhook_message";

  return (
    <section className="surface-card">
      <div className="card-copy">
        <p className="section-label">Posting destination</p>
        <h2>Where the board is posted</h2>
        <p className="section-description">
          Phase 1 keeps the current backend contract, so posting setup still requires
          direct IDs or a webhook URL.
        </p>
      </div>

      <div className="field-grid">
        <label className="field-stack">
          <span className="field-label">Posting method</span>
          <select
            value={deliveryForm.type}
            onChange={(event) => setDeliveryFormField("type", event.target.value)}
          >
            <option value="channel_message">Channel message</option>
            <option value="webhook_message">Webhook message</option>
          </select>
        </label>

        <label className="field-stack">
          <span className="field-label">Board message ID</span>
          <input
            value={deliveryForm.messageID}
            onChange={(event) => setDeliveryFormField("messageID", event.target.value)}
            placeholder="123456789012345678"
          />
        </label>

        {usesWebhook ? (
          <label className="field-stack">
            <span className="field-label">Webhook URL</span>
            <input
              value={deliveryForm.webhookURL}
              onChange={(event) =>
                setDeliveryFormField("webhookURL", event.target.value)
              }
              placeholder="https://discord.com/api/webhooks/..."
            />
          </label>
        ) : (
          <label className="field-stack">
            <span className="field-label">Channel ID</span>
            <input
              value={deliveryForm.channelID}
              onChange={(event) =>
                setDeliveryFormField("channelID", event.target.value)
              }
              placeholder="Channel ID"
            />
          </label>
        )}
      </div>

      <div className="card-actions">
        <button
          className="button-primary"
          type="button"
          disabled={loading}
          onClick={() => void saveDelivery()}
        >
          Save posting destination
        </button>
        <span className="meta-note">Direct identifiers remain required until richer destination picking is added.</span>
      </div>
    </section>
  );
}
