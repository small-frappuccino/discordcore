import { Link, useLocation } from "react-router-dom";
import { appRoutes } from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
} from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { IdentityAvatar, StatusBadge } from "../components/ui";

const siteBrandIconSrc = `${import.meta.env.BASE_URL}brand/alicebot.webp`;

export function LandingPage() {
  const location = useLocation();
  const {
    authState,
    beginLogin,
    accessibleGuilds,
    session,
    sessionAvatarURL,
    sessionLoading,
  } = useDashboardSession();

  return (
    <main className="landing-shell">
      <header className="landing-topbar">
        <Link className="brand-card brand-card-landing" to={appRoutes.landing}>
          <span className="brand-mark" aria-hidden="true">
            <img src={siteBrandIconSrc} alt="" />
          </span>
          <span className="brand-copy">
            <span className="section-label">Discord bot admin</span>
            <strong>Discordcore Dashboard</strong>
            <small>Keep the public site separate from the admin workspace.</small>
          </span>
        </Link>

        <div className="landing-actions">
          {authState === "signed_in" ? (
            <Link className="button-primary" to={appRoutes.dashboardHome}>
              Open dashboard
            </Link>
          ) : (
            <button
              className="button-primary"
              type="button"
              disabled={sessionLoading}
              onClick={() => void beginLogin(location.pathname)}
            >
              Sign in with Discord
            </button>
          )}
        </div>
      </header>

      <section className="landing-hero">
        <div className="landing-copy">
          <p className="page-eyebrow">Public landing</p>
          <h1>Admin workflows belong in the dashboard, not in the homepage.</h1>
          <p className="page-description">
            The public surface handles onboarding and context. Server selection,
            Partner Board management, and technical settings stay in the authenticated
            dashboard shell.
          </p>
          <div className="landing-meta">
            <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
              {session !== null
                ? `Signed in as ${formatSessionTitle(session)}`
                : formatAuthStateLabel(authState)}
            </StatusBadge>
            <span className="meta-pill">
              {formatAuthSupportText(authState, accessibleGuilds.length)}
            </span>
          </div>
        </div>

        <section className="landing-profile-card surface-card">
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
              <p className="section-label">Dashboard access</p>
              <strong>
                {session !== null
                  ? formatSessionTitle(session)
                  : formatAuthStateLabel(authState)}
              </strong>
              <small>
                {formatAuthSupportText(authState, accessibleGuilds.length)}
              </small>
            </div>
          </div>
          <p className="section-description">
            Once you enter the dashboard, the shell holds global navigation,
            server context, and product workspaces separately.
          </p>
        </section>
      </section>

      <section className="landing-grid">
        <article className="surface-card landing-card">
          <p className="section-label">Global shell</p>
          <h2>Top bar and sidebar</h2>
          <p>
            The dashboard shell keeps server selection in the top bar and
            product navigation in the sidebar instead of repeating global
            context inside each page.
          </p>
        </article>
        <article className="surface-card landing-card">
          <p className="section-label">Feature workspace</p>
          <h2>Task-first pages</h2>
          <p>
            Partner Board is organized around entries, layout, posting destination,
            and future activity, instead of mirroring raw backend objects.
          </p>
        </article>
        <article className="surface-card landing-card">
          <p className="section-label">Technical details</p>
          <h2>Progressive disclosure</h2>
          <p>
            Connection URLs, permissions, IDs, and troubleshooting details stay
            behind Advanced panels so the default workspace remains readable.
          </p>
        </article>
      </section>
    </main>
  );
}
