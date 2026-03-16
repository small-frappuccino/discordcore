import type { ReactNode } from "react";
import { useLocation } from "react-router-dom";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardWorkspaceState() {
  const location = useLocation();
  const { beginLogin } = useDashboardSession();
  const { refreshBoard, workspaceState } = usePartnerBoard();

  if (workspaceState === "checking") {
    return (
      <WorkspaceStateMessage
        title="Checking dashboard access"
        description="The dashboard is verifying your current session before loading Partner Board settings."
      />
    );
  }

  if (workspaceState === "auth_required") {
    return (
      <WorkspaceStateMessage
        title="Sign in with Discord"
        description="Partner Board uses the global dashboard session. Sign in first, then choose a server from the sidebar."
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
        description="Select a server from the sidebar to load its Partner Board settings."
      />
    );
  }

  if (workspaceState === "loading") {
    return (
      <WorkspaceStateMessage
        title="Loading Partner Board"
        description="Fetching the current board configuration for the selected server."
      />
    );
  }

  return (
    <WorkspaceStateMessage
      title="Partner Board unavailable"
      description="The dashboard could not load this server's Partner Board configuration. Refresh the data or verify the server has a board configuration."
      action={
        <button
          className="button-primary"
          type="button"
          onClick={() => void refreshBoard()}
        >
          Refresh data
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
    <section className="workspace-state">
      <div className="card-copy">
        <p className="section-label">Workspace</p>
        <h2>{title}</h2>
        <p className="section-description">{description}</p>
      </div>
      {action ? <div className="workspace-state-actions">{action}</div> : null}
    </section>
  );
}
