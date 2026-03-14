import { Link, useLocation } from "react-router-dom";
import { appRoutes } from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatTimestamp,
} from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { PageHeader, StatusBadge } from "../components/ui";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";

export function OverviewPage() {
  const location = useLocation();
  const {
    authState,
    beginLogin,
    manageableGuilds,
    selectedGuild,
  } = useDashboardSession();
  const {
    deliveryConfigured,
    lastLoadedAt,
    layoutConfigured,
    loading,
    partnerCount,
    postingMethodLabel,
    refreshBoardSummary,
    shellStatus,
    summarizePostingDestination,
  } = usePartnerBoardSummary();

  const nextPath = `${location.pathname}${location.search}${location.hash}`;

  function renderPrimaryAction() {
    if (authState !== "signed_in") {
      return (
        <button
          className="button-primary"
          type="button"
          onClick={() => void beginLogin(nextPath)}
        >
          Sign in with Discord
        </button>
      );
    }

    if (selectedGuild === null) {
      return <span className="meta-note">Choose a server from the sidebar to continue.</span>;
    }

    if (!deliveryConfigured) {
      return (
        <Link className="button-primary" to={appRoutes.partnerBoardDelivery}>
          Finish destination
        </Link>
      );
    }

    if (partnerCount === 0) {
      return (
        <Link className="button-primary" to={appRoutes.partnerBoardEntries}>
          Add first partner
        </Link>
      );
    }

    if (!layoutConfigured) {
      return (
        <Link className="button-primary" to={appRoutes.partnerBoardLayout}>
          Edit layout
        </Link>
      );
    }

    return (
      <Link className="button-primary" to={appRoutes.partnerBoardEntries}>
        Open Partner Board
      </Link>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Overview"
        title="Overview"
        description="Check access, server readiness, and Partner Board setup for the selected server."
        status={
          <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
            {formatAuthStateLabel(authState)}
          </StatusBadge>
        }
      />

      <div className="content-grid content-grid-single">
        <section className="overview-status-grid" aria-label="Server status">
          <article className="surface-card overview-status-card">
            <p className="section-label">Access</p>
            <h2>{formatAuthStateLabel(authState)}</h2>
            <p className="section-description">
              {formatAuthSupportText(authState, manageableGuilds.length)}
            </p>
          </article>

          <article className="surface-card overview-status-card">
            <p className="section-label">Server</p>
            <h2>{selectedGuild?.name ?? "No server selected"}</h2>
            <p className="section-description">
              {selectedGuild === null
                ? "Select a server in the sidebar to load its feature workspace."
                : "Every page in the dashboard now uses this server scope."}
            </p>
          </article>

          <article className="surface-card overview-status-card">
            <p className="section-label">Partner Board</p>
            <h2>{shellStatus.label}</h2>
            <p className="section-description">{shellStatus.description}</p>
          </article>
        </section>

        <section className="surface-card feature-status-card">
          <div className="card-header">
            <div className="card-copy">
              <p className="section-label">Feature status</p>
              <h2>Partner Board</h2>
              <p className="section-description">
                Manage entries, layout, and publishing for the selected server.
              </p>
            </div>

            <div className="card-actions">
              {selectedGuild !== null && authState === "signed_in" ? (
                <button
                  className="button-secondary"
                  type="button"
                  disabled={loading}
                  onClick={() => void refreshBoardSummary()}
                >
                  Refresh status
                </button>
              ) : null}
              {renderPrimaryAction()}
            </div>
          </div>

          <div className="summary-list">
            <div className="summary-row">
              <span>Setup state</span>
              <strong>{shellStatus.label}</strong>
            </div>
            <div className="summary-row">
              <span>Posting destination</span>
              <strong>{summarizePostingDestination}</strong>
            </div>
            <div className="summary-row">
              <span>Posting method</span>
              <strong>{postingMethodLabel}</strong>
            </div>
            <div className="summary-row">
              <span>Partner entries</span>
              <strong>{partnerCount}</strong>
            </div>
            <div className="summary-row">
              <span>Last checked</span>
              <strong>{formatTimestamp(lastLoadedAt, "Not checked yet")}</strong>
            </div>
          </div>
        </section>

        <section className="surface-card roadmap-card" id="roadmap">
          <div className="card-copy">
            <p className="section-label">Roadmap</p>
            <h2>Planned areas</h2>
            <p className="section-description">
              Future features stay off the main navigation until they become actionable.
            </p>
          </div>

          <div className="roadmap-list" aria-label="Planned feature roadmap">
            <article className="roadmap-item">
              <strong>Moderation</strong>
              <p>Rules, reports, and enforcement workflows will arrive once the first real tools are ready.</p>
            </article>
            <article className="roadmap-item">
              <strong>Automations</strong>
              <p>Scheduled workflows stay hidden until the dashboard can run and inspect real automations.</p>
            </article>
            <article className="roadmap-item">
              <strong>Global activity</strong>
              <p>Cross-feature history returns only after feature-level events exist and can be filtered meaningfully.</p>
            </article>
          </div>
        </section>
      </div>
    </section>
  );
}
