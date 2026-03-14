import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { appRoutes } from "../../app/routes";
import type { SettingsNavigationState } from "../../app/types";
import { buildDeliveryPayload, formsFromBoard, getDeliveryChecklist, getDeliveryGuidance, isDeliveryConfigured, postingMethodLabel, type DeliveryFormState } from "./model";
import { PartnerBoardWorkspaceState } from "./PartnerBoardWorkspaceState";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardDeliveryPage() {
  const { board, workspaceState } = usePartnerBoard();
  const [desiredMethod, setDesiredMethod] =
    useState<DeliveryFormState["type"]>("channel_message");

  useEffect(() => {
    const boardType =
      formsFromBoard(board).deliveryForm.type === "webhook_message"
        ? "webhook_message"
        : "channel_message";
    setDesiredMethod(boardType);
  }, [board]);

  if (workspaceState !== "ready") {
    return <PartnerBoardWorkspaceState />;
  }

  const currentDelivery = formsFromBoard(board).deliveryForm;
  const nextDelivery = {
    ...currentDelivery,
    type: desiredMethod,
  };
  const currentMethodLabel = postingMethodLabel(currentDelivery.type);
  const nextMethodLabel = postingMethodLabel(desiredMethod);
  const destinationReady = isDeliveryConfigured(buildDeliveryPayload(nextDelivery));
  const checklist = getDeliveryChecklist(nextDelivery);
  const settingsState: SettingsNavigationState = {
    diagnostics: {
      partnerBoardTargetType: desiredMethod,
    },
  };

  return (
    <section className="surface-card">
      <div className="card-header">
        <div className="card-copy">
          <p className="section-label">Destination</p>
          <h2>Set where the board is published</h2>
          <p className="section-description">
            Choose the publishing method here, then finish the advanced connection details in Settings.
          </p>
        </div>
        <StatusPill ready={destinationReady}>
          {destinationReady ? "Destination ready" : "Needs destination setup"}
        </StatusPill>
      </div>

      <div className="summary-list">
        <div className="summary-row">
          <span>Current publishing method</span>
          <strong>{currentMethodLabel}</strong>
        </div>
        <div className="summary-row">
          <span>Current setup</span>
          <strong>{getDeliveryGuidance(currentDelivery, isDeliveryConfigured(buildDeliveryPayload(currentDelivery)))}</strong>
        </div>
      </div>

      <label className="field-stack">
        <span className="field-label">Preferred posting method</span>
        <select
          aria-label="Preferred posting method"
          value={desiredMethod}
          onChange={(event) =>
            setDesiredMethod(event.target.value as DeliveryFormState["type"])
          }
        >
          <option value="channel_message">Channel message</option>
          <option value="webhook_message">Webhook message</option>
        </select>
      </label>

      <section className="surface-subsection">
        <div className="card-copy">
          <p className="section-label">Setup checklist</p>
          <h3>{nextMethodLabel}</h3>
          <p className="section-description">{getDeliveryGuidance(nextDelivery, destinationReady)}</p>
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

      <div className="card-actions">
        <Link
          className="button-primary"
          to={`${appRoutes.settings}#diagnostics`}
          state={settingsState}
        >
          Finish destination in Settings
        </Link>
        <span className="meta-note">
          Raw Discord identifiers stay inside Diagnostics so the feature workspace stays task-focused.
        </span>
      </div>
    </section>
  );
}

function StatusPill({
  children,
  ready,
}: {
  children: string;
  ready: boolean;
}) {
  return (
    <span className={`meta-pill ${ready ? "status-success" : "status-info"}`}>
      {children}
    </span>
  );
}
