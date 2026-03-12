import { Link } from "react-router-dom";
import { appRoutes } from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
} from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { IdentityAvatar, PageHeader, StatusBadge } from "../components/ui";

export function OverviewPage() {
  const {
    authState,
    manageableGuilds,
    selectedGuild,
    selectedGuildIconURL,
    session,
    sessionAvatarURL,
  } = useDashboardSession();

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Overview"
        title="Dashboard overview"
        description="Use the sidebar for global navigation, then manage each product area inside its own workspace."
        status={
          <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
            {formatAuthStateLabel(authState)}
          </StatusBadge>
        }
      />

      <div className="content-grid content-grid-single">
        <section className="surface-card">
          <div className="card-copy">
            <p className="section-label">Account access</p>
            <h2>Current session</h2>
            <p className="section-description">
              Authentication is app-level context. Features consume it instead of
              re-implementing sign-in logic.
            </p>
          </div>

          <div className="identity-row">
            <IdentityAvatar
              imageUrl={sessionAvatarURL}
              label={
                session !== null
                  ? formatSessionTitle(session)
                  : formatAuthStateLabel(authState)
              }
            />
            <div className="identity-copy">
              <strong>
                {session !== null
                  ? formatSessionTitle(session)
                  : formatAuthStateLabel(authState)}
              </strong>
              <small>
                {formatAuthSupportText(authState, manageableGuilds.length)}
              </small>
            </div>
          </div>
        </section>

        <section className="surface-card">
          <div className="card-copy">
            <p className="section-label">Server context</p>
            <h2>Selected server</h2>
            <p className="section-description">
              Dashboard pages inherit the same server selection from the sidebar.
            </p>
          </div>

          <div className="identity-row">
            <IdentityAvatar
              imageUrl={selectedGuildIconURL}
              label={selectedGuild?.name ?? "No server selected"}
            />
            <div className="identity-copy">
              <strong>{selectedGuild?.name ?? "No server selected"}</strong>
              <small>
                {selectedGuild === null
                  ? "Choose a server from the sidebar to load feature data."
                  : "Partner Board and future features will use this server context."}
              </small>
            </div>
          </div>
        </section>

        <section className="surface-card">
          <div className="card-copy">
            <p className="section-label">Engagement</p>
            <h2>Partner Board</h2>
            <p className="section-description">
              Manage board entries, board copy, and posting destination in a dedicated feature workspace.
            </p>
          </div>

          <div className="card-actions">
            <Link className="button-primary" to={appRoutes.partnerBoardEntries}>
              Open Partner Board
            </Link>
          </div>
        </section>

        <div className="tile-grid">
          <article className="surface-card summary-tile">
            <p className="section-label">Moderation</p>
            <h3>Planned area</h3>
            <p>Rules, actions, and reports will move into their own workspace.</p>
          </article>
          <article className="surface-card summary-tile">
            <p className="section-label">Automations</p>
            <h3>Planned area</h3>
            <p>Scheduled workflows and future automations will live here.</p>
          </article>
          <article className="surface-card summary-tile">
            <p className="section-label">Activity Log</p>
            <h3>Planned area</h3>
            <p>Cross-feature history arrives after feature-level events are exposed.</p>
          </article>
        </div>
      </div>
    </section>
  );
}
