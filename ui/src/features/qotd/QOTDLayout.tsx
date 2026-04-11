import type { ReactNode } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { buildQOTDTabs } from "../../app/routes";
import { DashboardPageSurface, PageHeader } from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { QOTD_BUSY_LABELS, useQOTD } from "./QOTDContext";

export function QOTDLayout() {
  const location = useLocation();
  const {
    canEditSelectedGuild,
    selectedGuildID,
  } = useDashboardSession();
  const {
    busyLabel,
    notice,
    publishNow,
    workspaceState,
  } = useQOTD();
  const normalizedGuildID = selectedGuildID.trim();
  const isQuestionsRoute = location.pathname.endsWith("/questions");
  const tabs = normalizedGuildID === "" ? [] : buildQOTDTabs(normalizedGuildID);
  const publishBusy = busyLabel === QOTD_BUSY_LABELS.publishNow;

  return (
    <div className="qotd-page">
      <PageHeader
        eyebrow="Engagement"
        title="QOTD"
        description="Settings and question bank."
        actions={
          workspaceState === "ready" && isQuestionsRoute && canEditSelectedGuild ? (
            <button
              className="button-primary"
              type="button"
              disabled={publishBusy}
              onClick={() => void publishNow()}
            >
              {publishBusy ? "Publishing..." : "Publish manual QOTD"}
            </button>
          ) : undefined
        }
      />

      <DashboardPageSurface className="qotd-page-surface" notice={notice}>
        {workspaceState !== "ready" ? (
          <QOTDWorkspaceState />
        ) : (
          <div className="qotd-shell">
            {tabs.length > 0 ? (
              <nav className="subnav qotd-tabs" aria-label="QOTD sections">
                {tabs.map((tab) => (
                  <NavLink
                    key={tab.path}
                    className={({ isActive }) =>
                      isActive ? "subnav-link is-active" : "subnav-link"
                    }
                    to={tab.path}
                  >
                    {tab.label}
                  </NavLink>
                ))}
              </nav>
            ) : null}

            <Outlet />
          </div>
        )}
      </DashboardPageSurface>
    </div>
  );
}

function QOTDWorkspaceState() {
  const location = useLocation();
  const { beginLogin } = useDashboardSession();
  const { busyLabel, refreshWorkspace, workspaceState } = useQOTD();
  const refreshing = busyLabel === QOTD_BUSY_LABELS.refreshWorkspace;

  if (workspaceState === "checking") {
    return (
      <WorkspaceStateMessage
        title="Checking dashboard access"
        description="The dashboard is verifying your session before loading QOTD."
      />
    );
  }

  if (workspaceState === "auth_required") {
    return (
      <WorkspaceStateMessage
        title="Sign in with Discord"
        description="Sign in first, then choose a server from the top bar to manage QOTD."
        action={
          <button
            className="button-primary"
            type="button"
            onClick={() => void beginLogin(`${location.pathname}${location.search}`)}
          >
            Sign in with Discord
          </button>
        }
      />
    );
  }

  if (workspaceState === "server_required") {
    return (
      <WorkspaceStateMessage
        title="Choose a server"
        description="Select a server from the top bar to load its QOTD workspace."
      />
    );
  }

  if (workspaceState === "loading") {
    return (
      <WorkspaceStateMessage
        title="Loading QOTD"
        description="Fetching the current settings and question bank for this server."
      />
    );
  }

  return (
    <WorkspaceStateMessage
      title="QOTD unavailable"
      description="The dashboard could not load this server's QOTD workspace."
      action={
        <button
          className="button-primary"
          type="button"
          disabled={refreshing}
          onClick={() => void refreshWorkspace()}
        >
          {refreshing ? "Retrying..." : "Retry loading"}
        </button>
      }
    />
  );
}

function WorkspaceStateMessage({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <section className="workspace-state qotd-workspace-state">
      <div className="card-copy">
        <p className="section-label">QOTD</p>
        <h2>{title}</h2>
        <p className="section-description">{description}</p>
      </div>
      {action ? <div className="workspace-state-actions">{action}</div> : null}
    </section>
  );
}
