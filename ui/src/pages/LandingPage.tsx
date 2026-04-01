import { Link } from "react-router-dom";
import type { AccessibleGuild } from "../api/control";
import { appRoutes } from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
} from "../app/utils";
import {
  IdentityAvatar,
  PageContentSurface,
  StatusBadge,
  AlertBanner,
} from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import "../shell.css";

const siteBrandIconSrc = `${import.meta.env.BASE_URL}brand/alicebot.webp`;
const signedOutNotice = "Sign in with Discord to continue.";
const signedOutConfirmationNotice = "Signed out.";

export function LandingPage() {
  const {
    accessibleGuilds,
    authState,
    beginLogin,
    busyLabel,
    manageableGuilds,
    notice,
    logout,
    selectedGuildID,
    session,
    sessionAvatarURL,
    sessionLoading,
  } = useDashboardSession();
  const sessionTitle =
    session !== null ? formatSessionTitle(session) : formatAuthStateLabel(authState);
  const sessionSupportText = formatAuthSupportText(
    authState,
    accessibleGuilds.length,
  );
  const navigableGuilds =
    accessibleGuilds.length > 0 ? accessibleGuilds : manageableGuilds;
  const controlPanelPath = getControlPanelPath(selectedGuildID, navigableGuilds);
  const landingNotice =
    notice?.message === signedOutNotice ||
    notice?.message === signedOutConfirmationNotice
      ? null
      : notice;
  const statusTone =
    authState === "signed_in"
      ? "success"
      : authState === "oauth_unavailable"
        ? "error"
        : "info";
  const summaryTitle =
    authState === "signed_in"
      ? "Authentication complete"
      : "Authentication required";
  const summaryDescription =
    authState === "signed_in"
      ? "Keep this page as the entry point, then open the dashboard shell only when you want to manage a server."
      : "Use Discord authentication here, then continue into the dashboard shell without forcing a redirect away from the landing page.";
  const canOpenDashboard = authState === "signed_in" && session !== null;

  return (
    <main className="dashboard-layout-shell landing-dashboard-shell">
      <h1 className="sr-only">Discordcore Dashboard</h1>

      <header className="shell-topbar landing-shell-topbar" data-shell-topbar>
        <Link
          className="shell-brand landing-shell-brand"
          to={appRoutes.landing}
          aria-label="Open landing page"
        >
          <span className="shell-brand-mark" aria-hidden="true">
            <img src={siteBrandIconSrc} alt="" />
          </span>
          <span className="landing-shell-brand-copy">
            <strong>Discordcore Dashboard</strong>
            <span>Discord bot control surface</span>
          </span>
        </Link>

        <div className="shell-topbar-spacer" aria-hidden="true" />

        <div className="landing-topbar-actions">
          {canOpenDashboard ? (
            <>
              <Link
                className="button-primary landing-topbar-button"
                to={controlPanelPath}
              >
                Control Panel
              </Link>
              <button
                className="button-primary landing-topbar-button"
                type="button"
                disabled={sessionLoading}
                onClick={() => void logout()}
              >
                Logout
              </button>
            </>
          ) : (
            <button
              className="button-primary landing-topbar-button"
              type="button"
              disabled={sessionLoading}
              onClick={() => void beginLogin(appRoutes.landing)}
            >
              Login with Discord
            </button>
          )}
        </div>
      </header>

      <section className="landing-dashboard-main">
        {landingNotice || sessionLoading ? (
          <div className="landing-dashboard-notice">
            <AlertBanner
              notice={landingNotice}
              busyLabel={sessionLoading ? busyLabel : undefined}
            />
          </div>
        ) : null}

        <section className="page-shell landing-page-shell">
          <PageContentSurface className="landing-page-surface">
            <section className="overview-section-block landing-page-panel">
              <div className="landing-page-copy">
                <div className="card-copy">
                  <p className="section-label">Dashboard access</p>
                  <h2>{summaryTitle}</h2>
                  <p className="section-description">{summaryDescription}</p>
                </div>

                <div className="landing-meta">
                  <StatusBadge tone={statusTone}>{sessionTitle}</StatusBadge>
                  <span className="meta-pill subtle-pill">{sessionSupportText}</span>
                </div>
              </div>

              <aside className="surface-card landing-session-card">
                <div className="identity-row">
                  <IdentityAvatar imageUrl={sessionAvatarURL} label={sessionTitle} />
                  <div className="identity-copy">
                    <p className="section-label">Session</p>
                    <strong>{sessionTitle}</strong>
                    <small>{sessionSupportText}</small>
                  </div>
                </div>

                <dl className="key-value-list landing-session-stats">
                  <div className="key-value-row">
                    <dt>Available servers</dt>
                    <dd>{accessibleGuilds.length}</dd>
                  </div>
                  <div className="key-value-row">
                    <dt>Manageable servers</dt>
                    <dd>{manageableGuilds.length}</dd>
                  </div>
                </dl>
              </aside>
            </section>
          </PageContentSurface>
        </section>
      </section>
    </main>
  );
}

function getControlPanelPath(
  selectedGuildID: string,
  navigableGuilds: AccessibleGuild[],
) {
  const preferredGuildID = selectedGuildID.trim();
  if (preferredGuildID !== "") {
    return appRoutes.dashboardHome(preferredGuildID);
  }

  const fallbackGuildID = navigableGuilds[0]?.id?.trim() ?? "";
  if (fallbackGuildID !== "") {
    return appRoutes.dashboardHome(fallbackGuildID);
  }

  return appRoutes.manage;
}
