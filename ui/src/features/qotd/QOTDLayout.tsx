import { NavLink, Outlet } from "react-router-dom";
import { buildQOTDTabs } from "../../app/routes";
import { formatTimestamp } from "../../app/utils";
import {
  DashboardPageSurface,
  EmptyState,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useQOTD } from "./QOTDContext";

export function QOTDLayout() {
  const {
    authState,
    beginLogin,
    canEditSelectedGuild,
    selectedGuild,
    selectedGuildID,
  } = useDashboardSession();
  const {
    busyLabel,
    loading,
    notice,
    refreshWorkspace,
    publishNow,
    settings,
    summary,
    workspaceState,
  } = useQOTD();
  const tabs = buildQOTDTabs(selectedGuildID);
  const status = buildShellStatus(settings.enabled ?? false, summary?.published_for_current_slot ?? false);
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const currentPostLabel = summary?.current_post?.published_at
    ? `Published ${formatTimestamp(Date.parse(summary.current_post.published_at))}`
    : "No current Discord post yet";
  const previousPostLabel = summary?.previous_post?.publish_date_utc
    ? `Previous slot ${summary.previous_post.publish_date_utc.slice(0, 10)}`
    : "No previous slot active";
  const queueReadyCount = summary?.counts.ready ?? 0;
  const publishDisabled =
    authState !== "signed_in" ||
    !canEditSelectedGuild ||
    !(settings.enabled ?? false) ||
    loading;

  function renderBody() {
    if (workspaceState === "checking" || workspaceState === "loading") {
      return (
        <SurfaceCard className="empty-state-card">
          <p className="section-label">Workspace</p>
          <h2>Loading QOTD</h2>
          <p>Preparing the selected server&apos;s QOTD settings, queue, and publish state.</p>
        </SurfaceCard>
      );
    }

    if (workspaceState === "auth_required") {
      return (
        <EmptyState
          title="Sign in required"
          description="Sign in with Discord before managing the QOTD workflow."
          action={
            <button
              className="button-primary"
              type="button"
              onClick={() => void beginLogin()}
            >
              Sign in with Discord
            </button>
          }
        />
      );
    }

    if (workspaceState === "server_required") {
      return (
        <EmptyState
          title="Select a server"
          description="Choose a server from the top bar before editing QOTD settings or queue order."
        />
      );
    }

    return (
      <DashboardPageSurface notice={notice} busyLabel={loading ? busyLabel : busyLabel || undefined}>
        <section aria-label="QOTD summary" className="partner-board-summary-strip">
          <MetricCard
            label="Status"
            value={status.label}
            description={status.description}
            tone={status.tone}
          />
          <MetricCard
            label="Current slot"
            value={summary?.published_for_current_slot ? "Published" : "Pending"}
            description={currentPostLabel}
            tone={summary?.published_for_current_slot ? "success" : "info"}
          />
          <MetricCard
            label="Ready questions"
            value={String(queueReadyCount)}
            description="Questions ready to be reserved for the next due publish."
            tone={queueReadyCount > 0 ? "success" : "info"}
          />
          <MetricCard
            label="Previous slot"
            value={summary?.previous_post ? "Available" : "None"}
            description={previousPostLabel}
            tone={summary?.previous_post ? "neutral" : "info"}
          />
        </section>

        <SurfaceCard className="workspace-panel">
          <nav className="subnav workspace-tabs" aria-label="QOTD sections">
            {tabs.map((tab) => (
              <NavLink
                key={tab.path}
                className={({ isActive }) =>
                  `subnav-link${isActive ? " is-active" : ""}`
                }
                to={tab.path}
              >
                {tab.label}
              </NavLink>
            ))}
          </nav>

          <div className="workspace-panel-body">
            <Outlet />
          </div>
        </SurfaceCard>
      </DashboardPageSurface>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Engagement"
        title="QOTD"
        description="Configure the daily question forum workflow, maintain the ordered question bank, and publish extra manual QOTDs without changing the scheduled slot."
        status={<StatusBadge tone={status.tone}>{status.label}</StatusBadge>}
        meta={
          <>
            <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
            <span className="meta-pill subtle-pill">
              {summary?.current_publish_date_utc?.slice(0, 10) ?? "No active slot"}
            </span>
          </>
        }
        actions={
          <div className="inline-actions">
            <button
              className="button-secondary"
              type="button"
              disabled={loading}
              onClick={() => void refreshWorkspace()}
            >
              Refresh
            </button>
            <button
              className="button-primary"
              type="button"
              disabled={publishDisabled}
              onClick={() => void publishNow()}
            >
              Publish manual QOTD
            </button>
          </div>
        }
      />

      {renderBody()}
    </section>
  );
}

function buildShellStatus(enabled: boolean, publishedForCurrentSlot: boolean) {
  if (!enabled) {
    return {
      tone: "info" as const,
      label: "Disabled",
      description: "Enable QOTD and finish the forum/tag settings before publishing.",
    };
  }
  if (publishedForCurrentSlot) {
    return {
      tone: "success" as const,
      label: "Current slot published",
      description: "The due UTC slot already has an official Discord post.",
    };
  }
  return {
    tone: "info" as const,
    label: "Ready to publish",
    description: "The scheduled UTC slot is open. Manual publishes can still be created independently.",
  };
}
