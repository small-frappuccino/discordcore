import { NavLink, Outlet } from "react-router-dom";
import { partnerBoardTabs } from "../../app/routes";
import { formatTimestamp } from "../../app/utils";
import {
  AlertBanner,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../../components/ui";
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
  const methodLabel =
    deliveryForm.type === "webhook_message" ? "Webhook message" : "Channel message";
  const syncLabel =
    lastSyncedAt !== null
      ? `Synced ${formatTimestamp(lastSyncedAt)}`
      : `Last checked ${formatTimestamp(lastLoadedAt, "Not yet")}`;
  const destinationValue =
    summarizePostingDestination === "Not set" ? "Not set" : "Configured";

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Feature"
        title="Partner Board"
        description="Configure the board, keep its layout ready, and publish updates to Discord."
        status={<StatusBadge tone={shellStatus.tone}>{shellStatus.label}</StatusBadge>}
        meta={
          <>
            <span className="meta-pill subtle-pill">{syncLabel}</span>
            <span className="meta-pill subtle-pill">{summarizePostingDestination}</span>
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

      <section className="partner-board-summary-strip" aria-label="Partner Board setup summary">
        <MetricCard
          label="Setup"
          value={shellStatus.label}
          description={shellStatus.description}
          tone={shellStatus.tone}
        />
        <MetricCard
          label="Destination"
          value={destinationValue}
          description={summarizePostingDestination}
          tone={destinationValue === "Configured" ? "success" : "info"}
        />
        <MetricCard
          label="Partners"
          value={String(partners.length)}
          description={
            partners.length > 0
              ? "The board has real entries to manage."
              : "Add the first entry to make the workspace actionable."
          }
          tone={partners.length > 0 ? "success" : "neutral"}
        />
        <MetricCard
          label="Method"
          value={methodLabel}
          description={syncLabel}
          tone="neutral"
        />
      </section>

      <SurfaceCard className="workspace-panel">
        <nav className="subnav workspace-tabs" aria-label="Partner Board sections">
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

        <div className="workspace-panel-body">
          <Outlet />
        </div>
      </SurfaceCard>
    </section>
  );
}
