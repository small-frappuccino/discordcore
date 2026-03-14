import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import { PageHeader, StatusBadge, AlertBanner } from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
  formatTimestamp,
} from "../app/utils";
import type { Notice, SettingsNavigationState } from "../app/types";
import {
  buildDeliveryPayload,
  formsFromBoard,
  postingMethodLabel,
  validateDeliveryForm,
  type DeliveryFormState,
} from "../features/partner-board/model";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

const initialDiagnosticsDeliveryForm: DeliveryFormState = {
  type: "channel_message",
  messageID: "",
  webhookURL: "",
  channelID: "",
};

export function SettingsPage() {
  const location = useLocation();
  const navigationState = (location.state ?? null) as SettingsNavigationState | null;
  const requestedTargetType = navigationState?.diagnostics?.partnerBoardTargetType;
  const diagnosticsRequested =
    location.hash === "#diagnostics" || requestedTargetType !== undefined;
  const [deliveryForm, setDeliveryForm] = useState<DeliveryFormState>(
    initialDiagnosticsDeliveryForm,
  );
  const [diagnosticsNotice, setDiagnosticsNotice] = useState<Notice | null>(null);
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(diagnosticsRequested);
  const [savingDelivery, setSavingDelivery] = useState(false);
  const {
    applyBaseUrl,
    authState,
    baseUrlDirty,
    baseUrlDraft,
    client,
    currentOriginLabel,
    manageableGuilds,
    selectedGuild,
    selectedGuildID,
    session,
    sessionLoading,
    setBaseUrlDraft,
  } = useDashboardSession();
  const {
    board,
    deliveryConfigured,
    lastLoadedAt,
    loading: boardSummaryLoading,
    notice: boardSummaryNotice,
    postingMethodLabel: boardPostingMethodLabel,
    refreshBoardSummary,
    shellStatus,
  } = usePartnerBoardSummary();

  useEffect(() => {
    if (!diagnosticsRequested) {
      return;
    }

    setDiagnosticsOpen(true);

    requestAnimationFrame(() => {
      const diagnosticsSection = document.getElementById("diagnostics");
      if (
        diagnosticsSection &&
        typeof diagnosticsSection.scrollIntoView === "function"
      ) {
        diagnosticsSection.scrollIntoView({
          block: "start",
        });
      }
    });
  }, [diagnosticsRequested]);

  useEffect(() => {
    const nextDeliveryForm = formsFromBoard(board).deliveryForm;
    if (requestedTargetType === "channel_message" || requestedTargetType === "webhook_message") {
      setDeliveryForm({
        ...nextDeliveryForm,
        type: requestedTargetType,
      });
      return;
    }

    setDeliveryForm(nextDeliveryForm);
  }, [board, requestedTargetType]);

  async function handleSaveDelivery() {
    if (authState !== "signed_in" || selectedGuild === null) {
      setDiagnosticsNotice({
        tone: "info",
        message: "Sign in and select a server before editing Partner Board diagnostics.",
      });
      return;
    }

    const validationError = validateDeliveryForm(deliveryForm);
    if (validationError !== null) {
      setDiagnosticsNotice({
        tone: "error",
        message: validationError,
      });
      return;
    }

    setSavingDelivery(true);

    try {
      await client.setPartnerBoardTarget(
        selectedGuild.id,
        buildDeliveryPayload(deliveryForm),
      );
      await refreshBoardSummary();
      setDiagnosticsNotice({
        tone: "success",
        message: "Partner Board destination updated.",
      });
    } catch (error) {
      setDiagnosticsNotice({
        tone: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setSavingDelivery(false);
    }
  }

  const diagnosticsNoticeToRender = diagnosticsNotice ?? boardSummaryNotice;
  const boardReadyLabel = selectedGuild === null ? "No server selected" : shellStatus.label;

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Settings"
        title="Settings"
        description="Keep technical controls and diagnostics separate from the day-to-day feature workspace."
        status={
          <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
            {formatAuthStateLabel(authState)}
          </StatusBadge>
        }
      />

      <nav className="section-nav" aria-label="Settings sections">
        <a className="section-link" href="#server">
          Server
        </a>
        <a className="section-link" href="#connection">
          Connection
        </a>
        <a className="section-link" href="#permissions">
          Permissions
        </a>
        <a className="section-link" href="#diagnostics">
          Diagnostics
        </a>
      </nav>

      <div className="content-grid content-grid-single">
        <section className="surface-card" id="server">
          <div className="card-copy">
            <p className="section-label">Server</p>
            <h2>Current server scope</h2>
            <p className="section-description">
              Server selection is global. Feature workspaces inherit this selection automatically.
            </p>
          </div>

          <div className="settings-list">
            <div className="settings-row">
              <span>Selected server</span>
              <strong>{selectedGuild?.name ?? "No server selected"}</strong>
            </div>
            <div className="settings-row">
              <span>Feature readiness</span>
              <strong>{boardReadyLabel}</strong>
            </div>
            <div className="settings-row">
              <span>Manageable servers</span>
              <strong>{manageableGuilds.length}</strong>
            </div>
          </div>
        </section>

        <section className="surface-card" id="connection">
          <div className="card-copy">
            <p className="section-label">Connection</p>
            <h2>Control connection</h2>
            <p className="section-description">
              Use the current origin by default. Override it only when the dashboard needs another control server.
            </p>
          </div>

          <label className="field-stack">
            <span className="field-label">Connection URL</span>
            <input
              value={baseUrlDraft}
              onChange={(event) => setBaseUrlDraft(event.target.value)}
              placeholder="Leave blank to use the current origin"
            />
          </label>

          <div className="card-actions">
            <button
              className="button-primary"
              type="button"
              disabled={sessionLoading || !baseUrlDirty}
              onClick={applyBaseUrl}
            >
              Save connection
            </button>
            <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
          </div>
        </section>

        <section className="surface-card" id="permissions">
          <div className="card-copy">
            <p className="section-label">Permissions</p>
            <h2>Access summary</h2>
            <p className="section-description">
              Show user-facing access status here and keep literal OAuth scopes behind Diagnostics.
            </p>
          </div>

          <div className="settings-list">
            <div className="settings-row">
              <span>Account</span>
              <strong>
                {session !== null
                  ? formatSessionTitle(session)
                  : formatAuthStateLabel(authState)}
              </strong>
            </div>
            <div className="settings-row">
              <span>Server management access</span>
              <strong>{formatAuthSupportText(authState, manageableGuilds.length)}</strong>
            </div>
            <div className="settings-row">
              <span>Partner Board access</span>
              <strong>
                {selectedGuild === null
                  ? "Select a server to load feature access."
                  : shellStatus.description}
              </strong>
            </div>
          </div>
        </section>

        <details
          className="details-panel surface-card diagnostics-panel"
          id="diagnostics"
          onToggle={(event) =>
            setDiagnosticsOpen((event.currentTarget as HTMLDetailsElement).open)
          }
          open={diagnosticsOpen}
        >
          <summary>Diagnostics</summary>

          {diagnosticsNoticeToRender ? (
            <div className="diagnostics-alert">
              <AlertBanner notice={diagnosticsNoticeToRender} />
            </div>
          ) : null}

          <div className="details-content diagnostics-content">
            <section className="surface-subsection diagnostics-editor">
              <div className="card-copy">
                <p className="section-label">Partner Board destination</p>
                <h3>Advanced destination editor</h3>
                <p className="section-description">
                  This is the only place raw Discord identifiers and webhook details are edited.
                </p>
              </div>

              <div className="field-grid">
                <label className="field-stack">
                  <span className="field-label">Posting method</span>
                  <select
                    value={deliveryForm.type}
                    onChange={(event) =>
                      setDeliveryForm((currentValue) => ({
                        ...currentValue,
                        type: event.target.value as DeliveryFormState["type"],
                      }))
                    }
                  >
                    <option value="channel_message">Channel message</option>
                    <option value="webhook_message">Webhook message</option>
                  </select>
                </label>

                <label className="field-stack">
                  <span className="field-label">Board message ID</span>
                  <input
                    value={deliveryForm.messageID}
                    onChange={(event) =>
                      setDeliveryForm((currentValue) => ({
                        ...currentValue,
                        messageID: event.target.value,
                      }))
                    }
                    placeholder="123456789012345678"
                  />
                </label>

                {deliveryForm.type === "webhook_message" ? (
                  <label className="field-stack">
                    <span className="field-label">Webhook URL</span>
                    <input
                      value={deliveryForm.webhookURL}
                      onChange={(event) =>
                        setDeliveryForm((currentValue) => ({
                          ...currentValue,
                          webhookURL: event.target.value,
                        }))
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
                        setDeliveryForm((currentValue) => ({
                          ...currentValue,
                          channelID: event.target.value,
                        }))
                      }
                      placeholder="123456789012345678"
                    />
                  </label>
                )}
              </div>

              <div className="card-actions">
                <button
                  className="button-primary"
                  type="button"
                  disabled={savingDelivery || boardSummaryLoading}
                  onClick={() => void handleSaveDelivery()}
                >
                  Save destination
                </button>
                <span className="meta-pill subtle-pill">
                  {boardReadyLabel}
                </span>
                <span className="meta-pill subtle-pill">
                  {postingMethodLabel(deliveryForm.type)}
                </span>
              </div>
            </section>

            <div className="settings-list">
              <div className="settings-row">
                <span>Selected server ID</span>
                <strong>{selectedGuildID || "No server selected"}</strong>
              </div>
              <div className="settings-row">
                <span>Granted OAuth scopes</span>
                <strong>
                  {session !== null && session.scopes.length > 0
                    ? session.scopes.join(", ")
                    : "Unavailable until sign-in"}
                </strong>
              </div>
              <div className="settings-row">
                <span>Current Partner Board method</span>
                <strong>{boardPostingMethodLabel}</strong>
              </div>
              <div className="settings-row">
                <span>Destination completeness</span>
                <strong>{deliveryConfigured ? "Configured" : "Incomplete"}</strong>
              </div>
              <div className="settings-row">
                <span>Last board check</span>
                <strong>{formatTimestamp(lastLoadedAt, "Not checked yet")}</strong>
              </div>
              <div className="settings-row">
                <span>Session guidance</span>
                <strong>{formatAuthSupportText(authState, manageableGuilds.length)}</strong>
              </div>
            </div>
          </div>
        </details>
      </div>
    </section>
  );
}
