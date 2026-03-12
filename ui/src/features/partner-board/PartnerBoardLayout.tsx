import { NavLink, Outlet } from "react-router-dom";
import { partnerBoardTabs } from "../../app/routes";
import { formatTimestamp } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { AlertBanner, PageHeader, StatusBadge } from "../../components/ui";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardLayout() {
  const { session, selectedGuild } = useDashboardSession();
  const {
    board,
    busyLabel,
    deliveryForm,
    lastLoadedAt,
    lastSyncedAt,
    loading,
    notice,
    partners,
    refreshBoard,
    shellStatus,
    summarizePostingDestination,
    syncBoard,
  } = usePartnerBoard();

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Engagement"
        title="Partner Board"
        description="Manage the board entries, the board copy, and the posting destination without mixing in global auth or debug panels."
        status={<StatusBadge tone={shellStatus.tone}>{shellStatus.label}</StatusBadge>}
        meta={
          <>
            <span className="meta-pill">
              {selectedGuild?.name ?? "No server selected"}
            </span>
            <span className="meta-pill subtle-pill">
              {lastSyncedAt !== null
                ? `Synced ${formatTimestamp(lastSyncedAt)}`
                : `Last refresh ${formatTimestamp(lastLoadedAt, "Not yet")}`}
            </span>
          </>
        }
        actions={
          <>
            <button
              className="button-secondary"
              type="button"
              disabled={loading}
              onClick={() => void refreshBoard()}
            >
              Refresh data
            </button>
            <button
              className="button-primary"
              type="button"
              disabled={loading}
              onClick={() => void syncBoard()}
            >
              Sync to Discord
            </button>
          </>
        }
      />

      {notice ? (
        <AlertBanner notice={notice} busyLabel={loading ? busyLabel : undefined} />
      ) : loading ? (
        <AlertBanner busyLabel={busyLabel} />
      ) : null}

      <div className="content-grid content-grid-with-aside">
        <div className="page-main">
          <nav className="subnav" aria-label="Partner Board sections">
            {partnerBoardTabs.map((item) => (
              <NavLink
                key={item.path}
                className={({ isActive }) =>
                  `subnav-link${isActive ? " is-active" : ""}`
                }
                to={item.path}
              >
                {item.label}
              </NavLink>
            ))}
          </nav>

          <Outlet />
        </div>

        <aside className="page-aside">
          <section className="surface-card summary-card">
            <div className="card-copy">
              <p className="section-label">Summary</p>
              <h2>Board status</h2>
              <p className="section-description">{shellStatus.description}</p>
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
                <span>Partner entries</span>
                <strong>{partners.length}</strong>
              </div>
              <div className="summary-row">
                <span>Current method</span>
                <strong>
                  {deliveryForm.type === "webhook_message"
                    ? "Webhook message"
                    : "Channel message"}
                </strong>
              </div>
            </div>
          </section>

          <details className="details-panel surface-card">
            <summary>Troubleshooting</summary>
            <div className="details-content">
              <div className="summary-list">
                <div className="summary-row">
                  <span>Selected server ID</span>
                  <strong>{selectedGuild?.id ?? "No server selected"}</strong>
                </div>
                <div className="summary-row">
                  <span>Board message ID</span>
                  <strong>{board?.target?.message_id?.trim() || "Not set"}</strong>
                </div>
                <div className="summary-row">
                  <span>Permissions granted</span>
                  <strong>
                    {session !== null && session.scopes.length > 0
                      ? session.scopes.join(", ")
                      : "Unavailable until sign-in"}
                  </strong>
                </div>
                <div className="summary-row">
                  <span>Last data refresh</span>
                  <strong>{formatTimestamp(lastLoadedAt, "Not yet")}</strong>
                </div>
              </div>
            </div>
          </details>
        </aside>
      </div>
    </section>
  );
}
