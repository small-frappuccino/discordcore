import { PageHeader, StatusBadge } from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
} from "../app/utils";

export function SettingsPage() {
  const {
    applyBaseUrl,
    authState,
    baseUrlDirty,
    baseUrlDraft,
    currentOriginLabel,
    manageableGuilds,
    selectedGuild,
    session,
    sessionLoading,
    setBaseUrlDraft,
  } = useDashboardSession();

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Settings"
        title="Dashboard settings"
        description="Keep technical connection details and support context here instead of mixing them into feature workspaces."
        status={
          <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
            {formatAuthStateLabel(authState)}
          </StatusBadge>
        }
      />

      <div className="content-grid content-grid-single">
        <section className="surface-card">
          <div className="card-copy">
            <p className="section-label">Account</p>
            <h2>Access summary</h2>
            <p className="section-description">
              Session state is available globally, but technical details stay out
              of the default feature view.
            </p>
          </div>

          <div className="settings-list">
            <div className="settings-row">
              <span>Session</span>
              <strong>
                {session !== null
                  ? formatSessionTitle(session)
                  : formatAuthStateLabel(authState)}
              </strong>
            </div>
            <div className="settings-row">
              <span>Manageable servers</span>
              <strong>{manageableGuilds.length}</strong>
            </div>
            <div className="settings-row">
              <span>Selected server</span>
              <strong>{selectedGuild?.name ?? "None selected"}</strong>
            </div>
          </div>
        </section>

        <section className="surface-card">
          <div className="card-copy">
            <p className="section-label">Advanced</p>
            <h2>Control connection</h2>
            <p className="section-description">
              The dashboard defaults to the current origin. Override the connection
              only when you need to point the UI at another control server.
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

        <details className="details-panel surface-card">
          <summary>Troubleshooting</summary>
          <div className="details-content">
            <div className="settings-list">
              <div className="settings-row">
                <span>Permissions granted</span>
                <strong>
                  {session !== null && session.scopes.length > 0
                    ? session.scopes.join(", ")
                    : "Unavailable until sign-in"}
                </strong>
              </div>
              <div className="settings-row">
                <span>Selected server ID</span>
                <strong>{selectedGuild?.id ?? "No server selected"}</strong>
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
