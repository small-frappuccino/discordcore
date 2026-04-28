import type { ReactNode } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { FlatPageLayout } from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { QOTD_BUSY_LABELS, useQOTD } from "./QOTDContext";

export function QOTDLayout() {
  const { notice, workspaceState } = useQOTD();

  return (
    <section className="page-shell qotd-page">
      <FlatPageLayout
        notice={notice}
        workspaceEyebrow={null}
        workspaceTitle={null}
        workspaceDescription={null}
      >
        <div className="qotd-page-intro">
          <div className="card-copy">
            <div className="qotd-page-title-row">
              <h1>QOTD</h1>
            </div>
          </div>
        </div>

        {workspaceState !== "ready" ? (
          <QOTDWorkspaceState />
        ) : (
          <Outlet />
        )}
      </FlatPageLayout>
    </section>
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
        description="Fetching the current settings for this server."
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
