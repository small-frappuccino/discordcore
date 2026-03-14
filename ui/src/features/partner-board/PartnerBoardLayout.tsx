import { NavLink, Outlet } from "react-router-dom";
import { partnerBoardTabs } from "../../app/routes";
import { formatTimestamp } from "../../app/utils";
import { AlertBanner, PageHeader, StatusBadge } from "../../components/ui";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardLayout() {
  const {
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
        eyebrow="Feature"
        title="Partner Board"
        description="Configure the board, keep its layout ready, and publish updates to Discord."
        status={<StatusBadge tone={shellStatus.tone}>{shellStatus.label}</StatusBadge>}
        meta={
          <span className="meta-pill subtle-pill">
            {lastSyncedAt !== null
              ? `Synced ${formatTimestamp(lastSyncedAt)}`
              : `Last checked ${formatTimestamp(lastLoadedAt, "Not yet")}`}
          </span>
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

      <div className="content-grid content-grid-single">
        <section className="surface-card status-strip" aria-label="Partner Board setup summary">
          <div className="status-strip-item">
            <span className="section-label">Setup</span>
            <strong>{shellStatus.label}</strong>
          </div>
          <div className="status-strip-item">
            <span className="section-label">Destination</span>
            <strong>{summarizePostingDestination}</strong>
          </div>
          <div className="status-strip-item">
            <span className="section-label">Partners</span>
            <strong>{partners.length}</strong>
          </div>
          <div className="status-strip-item">
            <span className="section-label">Method</span>
            <strong>
              {deliveryForm.type === "webhook_message"
                ? "Webhook message"
                : "Channel message"}
            </strong>
          </div>
        </section>

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

        <div className="page-main">
          <Outlet />
        </div>
      </div>
    </section>
  );
}
