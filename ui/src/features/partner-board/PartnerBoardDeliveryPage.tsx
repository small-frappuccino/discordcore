import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { appRoutes } from "../../app/routes";
import type { SettingsNavigationState } from "../../app/types";
import { KeyValueList, StatusBadge } from "../../components/ui";
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
  const currentDestinationReady = isDeliveryConfigured(buildDeliveryPayload(currentDelivery));
  const nextMethodLabel = postingMethodLabel(desiredMethod);
  const destinationReady = isDeliveryConfigured(buildDeliveryPayload(nextDelivery));
  const checklist = getDeliveryChecklist(nextDelivery);
  const settingsState: SettingsNavigationState = {
    diagnostics: {
      partnerBoardTargetType: desiredMethod,
    },
  };

  return (
    <section className="workspace-view">
      <div className="workspace-view-header">
        <div className="card-copy">
          <p className="section-label">Destination</p>
          <h2>Set where the board is published</h2>
          <p className="section-description">
            Choose the publishing method here, then finish the advanced connection details in Settings.
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
            label: "Current publishing method",
            value: currentMethodLabel,
          },
          {
            label: "Current setup",
            value: getDeliveryGuidance(currentDelivery, currentDestinationReady),
          },
          {
            label: "Selected workflow",
            value:
              desiredMethod === currentDelivery.type
                ? "Using the saved method"
                : `Switch to ${nextMethodLabel} in Diagnostics`,
          },
        ]}
      />

      <section className="workspace-callout">
        <div className="card-copy">
          <p className="section-label">Delivery workflow</p>
          <h3>{nextMethodLabel}</h3>
          <p className="section-description">
            Pick the delivery style here, then finish the raw destination identifiers in Settings Diagnostics.
          </p>
        </div>
        <div className="workspace-callout-copy">
          <label className="field-stack field-stack-compact">
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
          <div className="workspace-toolbar-actions">
            <Link
              className="button-primary"
              to={`${appRoutes.settings}#diagnostics`}
              state={settingsState}
            >
              Finish destination in Settings
            </Link>
          </div>
          <p className="meta-note">
            Raw Discord identifiers stay inside Diagnostics so the feature workspace stays task-focused.
          </p>
        </div>
      </section>

      <section className="workspace-checklist-panel">
        <div className="card-copy">
          <p className="section-label">Setup checklist</p>
          <h3>{nextMethodLabel}</h3>
          <p className="section-description">
            {getDeliveryGuidance(nextDelivery, destinationReady)}
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
