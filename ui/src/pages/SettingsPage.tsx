import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import {
  AlertBanner,
  KeyValueList,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";
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

type SettingsSection = "server" | "connection" | "permissions";

const settingsSections: Array<{
  id: SettingsSection;
  label: string;
}> = [
  {
    id: "server",
    label: "Server",
  },
  {
    id: "connection",
    label: "Connection",
  },
  {
    id: "permissions",
    label: "Permissions",
  },
];

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
  const activeSection = resolveSettingsSection(location.hash);
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
    summarizePostingDestination,
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
    if (
      requestedTargetType === "channel_message" ||
      requestedTargetType === "webhook_message"
    ) {
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
  const accountLabel =
    session !== null ? formatSessionTitle(session) : formatAuthStateLabel(authState);
  const authSupportText = formatAuthSupportText(authState, manageableGuilds.length);
  const diagnosticsBusyLabel = savingDelivery
    ? "Saving destination"
    : boardSummaryLoading
      ? "Refreshing diagnostics"
      : undefined;
  const draftModeLabel = baseUrlDirty ? "Unsaved changes" : "Saved";

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
        meta={
          <>
            <span className="meta-pill subtle-pill">
              {selectedGuild?.name ?? "No server selected"}
            </span>
            <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
          </>
        }
      />

      <SurfaceCard className="workspace-panel settings-workspace-panel">
        <nav className="subnav workspace-tabs settings-subnav" aria-label="Settings sections">
          {settingsSections.map((section) => (
            <a
              key={section.id}
              className={`subnav-link${activeSection === section.id ? " is-active" : ""}`}
              href={`#${section.id}`}
            >
              {section.label}
            </a>
          ))}
        </nav>

        <div className="workspace-panel-body">
          {activeSection === "server" ? (
            <section className="workspace-view" id="server">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Server</p>
                  <h2>Current server</h2>
                  <p className="section-description">
                    Server selection is shared across feature workspaces. Choose it in the sidebar, then manage the active server here.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  <StatusBadge tone={shellStatus.tone}>{boardReadyLabel}</StatusBadge>
                </div>
              </div>

              <div className="settings-summary-strip">
                <MetricCard
                  label="Server"
                  value={selectedGuild?.name ?? "Not selected"}
                  description={
                    selectedGuild === null
                      ? "Choose a server from the sidebar."
                      : "This is the active server for dashboard workspaces."
                  }
                  tone={selectedGuild === null ? "info" : "success"}
                />
                <MetricCard
                  label="Readiness"
                  value={boardReadyLabel}
                  description={shellStatus.description}
                  tone={shellStatus.tone}
                />
                <MetricCard
                  label="Servers"
                  value={String(manageableGuilds.length)}
                  description={authSupportText}
                  tone={manageableGuilds.length > 0 ? "success" : "info"}
                />
                <MetricCard
                  label="Destination"
                  value={deliveryConfigured ? "Configured" : "Incomplete"}
                  description={summarizePostingDestination}
                  tone={deliveryConfigured ? "success" : "info"}
                />
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Selected server",
                    value: selectedGuild?.name ?? "No server selected",
                  },
                  {
                    label: "Partner Board status",
                    value: shellStatus.description,
                  },
                  {
                    label: "Posting method",
                    value: boardPostingMethodLabel,
                  },
                  {
                    label: "Last board check",
                    value: formatTimestamp(lastLoadedAt, "Not checked yet"),
                  },
                ]}
              />
            </section>
          ) : null}

          {activeSection === "connection" ? (
            <section className="workspace-view" id="connection">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Connection</p>
                  <h2>Control connection</h2>
                  <p className="section-description">
                    Point the dashboard at a control server only when you intentionally need another backend endpoint.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  <span className="meta-pill subtle-pill">{draftModeLabel}</span>
                </div>
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Saved endpoint",
                    value: currentOriginLabel,
                  },
                  {
                    label: "Draft state",
                    value:
                      baseUrlDirty
                        ? "The draft differs from the saved endpoint."
                        : "The draft matches the saved endpoint.",
                  },
                  {
                    label: "Session status",
                    value: formatAuthStateLabel(authState),
                  },
                ]}
              />

              <label className="field-stack">
                <span className="field-label">Connection URL</span>
                <input
                  value={baseUrlDraft}
                  onChange={(event) => setBaseUrlDraft(event.target.value)}
                  placeholder="Leave blank to use the current origin"
                />
              </label>

              <div className="workspace-footer">
                <button
                  className="button-primary"
                  type="button"
                  disabled={sessionLoading || !baseUrlDirty}
                  onClick={applyBaseUrl}
                >
                  Save connection
                </button>
                <span className="meta-note">
                  Keep this value stable unless the dashboard should target another control server.
                </span>
              </div>
            </section>
          ) : null}

          {activeSection === "permissions" ? (
            <section className="workspace-view" id="permissions">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Permissions</p>
                  <h2>Access summary</h2>
                  <p className="section-description">
                    Show the user-facing access state here and keep raw OAuth scopes and identifiers inside Diagnostics.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
                    {formatAuthStateLabel(authState)}
                  </StatusBadge>
                </div>
              </div>

              <div className="settings-summary-strip settings-summary-strip-compact">
                <MetricCard
                  label="Session"
                  value={authState === "signed_in" ? "Connected" : formatAuthStateLabel(authState)}
                  description={accountLabel}
                  tone={authState === "signed_in" ? "success" : "info"}
                />
                <MetricCard
                  label="Servers"
                  value={String(manageableGuilds.length)}
                  description={authSupportText}
                  tone={manageableGuilds.length > 0 ? "success" : "info"}
                />
                <MetricCard
                  label="Partner Board"
                  value={boardReadyLabel}
                  description={
                    selectedGuild === null
                      ? "Select a server to load feature access."
                      : shellStatus.description
                  }
                  tone={selectedGuild === null ? "info" : shellStatus.tone}
                />
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Account",
                    value: accountLabel,
                  },
                  {
                    label: "Server management access",
                    value: authSupportText,
                  },
                  {
                    label: "Partner Board access",
                    value:
                      selectedGuild === null
                        ? "Select a server to load feature access."
                        : shellStatus.description,
                  },
                ]}
              />
            </section>
          ) : null}
        </div>
      </SurfaceCard>

      <details
        className="details-panel surface-card diagnostics-panel settings-diagnostics"
        id="diagnostics"
        onToggle={(event) =>
          setDiagnosticsOpen((event.currentTarget as HTMLDetailsElement).open)
        }
        open={diagnosticsOpen}
      >
        <summary>Diagnostics</summary>

        {(diagnosticsNoticeToRender || diagnosticsBusyLabel) ? (
          <div className="diagnostics-alert">
            <AlertBanner
              notice={diagnosticsNoticeToRender}
              busyLabel={diagnosticsBusyLabel}
            />
          </div>
        ) : null}

        <div className="details-content diagnostics-content settings-diagnostics-content">
          <div className="card-copy settings-diagnostics-copy">
            <p className="section-label">Advanced</p>
            <h2>Diagnostics</h2>
            <p className="section-description">
              Inspect raw identifiers, OAuth scopes, and transport details only when you need to troubleshoot or complete a technical setup.
            </p>
          </div>

          <div className="settings-diagnostics-grid">
            <SurfaceCard className="settings-diagnostics-editor">
              <div className="card-copy">
                <p className="section-label">Partner Board destination</p>
                <h3>Advanced destination editor</h3>
                <p className="section-description">
                  This is the only place raw Discord identifiers and webhook details are edited.
                </p>
              </div>

              <div className="workspace-form-grid">
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

              <div className="workspace-footer">
                <button
                  className="button-primary"
                  type="button"
                  disabled={savingDelivery || boardSummaryLoading}
                  onClick={() => void handleSaveDelivery()}
                >
                  Save destination
                </button>
                <div className="workspace-toolbar-actions">
                  <span className="meta-pill subtle-pill">{boardReadyLabel}</span>
                  <span className="meta-pill subtle-pill">
                    {postingMethodLabel(deliveryForm.type)}
                  </span>
                </div>
              </div>
            </SurfaceCard>

            <SurfaceCard className="settings-diagnostics-state">
              <div className="card-copy">
                <p className="section-label">Technical state</p>
                <h3>Current session and board</h3>
                <p className="section-description">
                  Use these values when troubleshooting destination setup, access, or session state.
                </p>
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Selected server ID",
                    value: selectedGuildID || "No server selected",
                  },
                  {
                    label: "Granted OAuth scopes",
                    value:
                      session !== null && session.scopes.length > 0
                        ? session.scopes.join(", ")
                        : "Unavailable until sign-in",
                  },
                  {
                    label: "Current Partner Board method",
                    value: boardPostingMethodLabel,
                  },
                  {
                    label: "Destination completeness",
                    value: deliveryConfigured ? "Configured" : "Incomplete",
                  },
                  {
                    label: "Last board check",
                    value: formatTimestamp(lastLoadedAt, "Not checked yet"),
                  },
                  {
                    label: "Session guidance",
                    value: authSupportText,
                  },
                ]}
              />
            </SurfaceCard>
          </div>
        </div>
      </details>
    </section>
  );
}

function resolveSettingsSection(hash: string): SettingsSection {
  switch (hash) {
    case "#connection":
      return "connection";
    case "#permissions":
      return "permissions";
    default:
      return "server";
  }
}
