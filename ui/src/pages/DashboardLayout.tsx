import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import { appRoutes, sidebarItems } from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
} from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { AlertBanner, IdentityAvatar } from "../components/ui";

const siteBrandIconSrc = `${import.meta.env.BASE_URL}brand/alicebot.webp`;

export function DashboardLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const {
    authState,
    beginLogin,
    busyLabel,
    manageableGuilds,
    notice,
    selectedGuildID,
    session,
    sessionAvatarURL,
    sessionLoading,
    setSelectedGuildID,
    logout,
  } = useDashboardSession();

  async function handleSignOut() {
    await logout();
    navigate(appRoutes.landing);
  }

  const nextPath = `${location.pathname}${location.search}`;

  return (
    <main className="dashboard-shell">
      <aside className="shell-sidebar">
        <Link className="brand-card" to={appRoutes.dashboardOverview}>
          <span className="brand-mark" aria-hidden="true">
            <img src={siteBrandIconSrc} alt="" />
          </span>
          <span className="brand-copy">
            <span className="section-label">Dashboard</span>
            <strong>Discordcore</strong>
            <small>Task-first control surface</small>
          </span>
        </Link>

        <section className="sidebar-card">
          <div className="card-copy">
            <p className="section-label">Current server</p>
            <h2>Workspace context</h2>
            <p className="section-description">
              Server selection applies across the whole dashboard.
            </p>
          </div>

          <label className="field-stack">
            <span className="field-label">Server</span>
            <select
              value={selectedGuildID}
              onChange={(event) => setSelectedGuildID(event.target.value)}
              disabled={authState !== "signed_in" || manageableGuilds.length === 0}
            >
              <option value="">
                {authState !== "signed_in"
                  ? "Sign in to load servers"
                  : "Choose a server"}
              </option>
              {manageableGuilds.map((guild) => (
                <option key={guild.id} value={guild.id}>
                  {guild.name}
                </option>
              ))}
            </select>
          </label>
        </section>

        <nav className="sidebar-nav" aria-label="Dashboard navigation">
          {sidebarItems.map((item) => {
            const isActive =
              location.pathname === item.path ||
              (item.matchPrefix !== undefined &&
                location.pathname.startsWith(item.matchPrefix));

            return (
              <Link
                key={item.label}
                className={`sidebar-link${isActive ? " is-active" : ""}`}
                to={item.path}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>

        <section className="sidebar-card sidebar-account">
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
              <p className="section-label">Account</p>
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

          <div className="sidebar-actions">
            <button
              className="button-secondary"
              type="button"
              disabled={sessionLoading}
              onClick={() => void beginLogin(nextPath)}
            >
              {authState === "signed_in" ? "Reconnect" : "Sign in"}
            </button>
            {authState === "signed_in" ? (
              <button
                className="button-ghost"
                type="button"
                disabled={sessionLoading}
                onClick={() => void handleSignOut()}
              >
                Sign out
              </button>
            ) : null}
          </div>
        </section>
      </aside>

      <section className="shell-content">
        {notice ? (
          <AlertBanner notice={notice} busyLabel={sessionLoading ? busyLabel : undefined} />
        ) : sessionLoading ? (
          <AlertBanner busyLabel={busyLabel} />
        ) : null}

        <Outlet />
      </section>
    </main>
  );
}
