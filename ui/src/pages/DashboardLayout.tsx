import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  appRoutes,
  sidebarHomeItem,
  sidebarNavigationSections,
  sidebarSettingsItem,
} from "../app/routes";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
} from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  AlertBanner,
  IdentityAvatar,
  SidebarSection,
  StatusBadge,
} from "../components/ui";

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
    selectedGuild,
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

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const accountTitle =
    session !== null
      ? formatSessionTitle(session)
      : formatAuthStateLabel(authState);
  const accountSupport = formatAuthSupportText(authState, manageableGuilds.length);
  const serverDescription =
    authState !== "signed_in"
      ? "Sign in to load available servers."
      : selectedGuild === null
        ? "Choose the active server."
        : "Current server for every feature area.";

  return (
    <main className="dashboard-shell">
      <aside className="shell-sidebar">
        <div className="sidebar-frame">
          <Link className="brand-card sidebar-brand" to={appRoutes.dashboardHome}>
            <span className="brand-mark" aria-hidden="true">
              <img src={siteBrandIconSrc} alt="" />
            </span>
            <span className="brand-copy">
              <span className="section-label">Dashboard</span>
              <strong>Discordcore</strong>
              <small>Server-scoped bot management</small>
            </span>
          </Link>

          <div className="sidebar-divider" />

          <SidebarSection
            className="sidebar-context"
            eyebrow="Server"
            title={selectedGuild?.name ?? "Select a server"}
            description={serverDescription}
          >
            <label className="field-stack field-stack-compact">
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
          </SidebarSection>

          <nav className="sidebar-nav" aria-label="Dashboard navigation">
            <Link
              className={`sidebar-link${
                isSidebarItemActive(
                  location.pathname,
                  location.hash,
                  sidebarHomeItem,
                )
                  ? " is-active"
                  : ""
              }`}
              to={sidebarHomeItem.to}
            >
              <span>{sidebarHomeItem.label}</span>
            </Link>

            {sidebarNavigationSections.map((section) => (
              <section className="sidebar-nav-section" key={section.label}>
                {section.label !== "Partner Board" ? (
                  <p className="section-label">{section.label}</p>
                ) : null}
                {section.items.map((item) => {
                  const isActive = isSidebarItemActive(
                    location.pathname,
                    location.hash,
                    item,
                  );

                  return (
                    <Link
                      key={item.label}
                      className={`sidebar-link${isActive ? " is-active" : ""}`}
                      to={item.to}
                    >
                      <span>{item.label}</span>
                    </Link>
                  );
                })}
              </section>
            ))}

            <div className="sidebar-divider sidebar-divider-spacer" />

            <Link
              className={`sidebar-link${
                isSidebarItemActive(
                  location.pathname,
                  location.hash,
                  sidebarSettingsItem,
                )
                  ? " is-active"
                  : ""
              }`}
              to={sidebarSettingsItem.to}
            >
              <span>{sidebarSettingsItem.label}</span>
            </Link>
          </nav>

          <div className="sidebar-divider sidebar-divider-spacer" />

          <SidebarSection
            className="sidebar-account"
            eyebrow="Account"
            title={accountTitle}
            description={accountSupport}
            footer={
              <div className="sidebar-actions sidebar-actions-compact">
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
            }
          >
            <div className="sidebar-account-row">
              <IdentityAvatar imageUrl={sessionAvatarURL} label={accountTitle} />
              <div className="identity-copy">
                <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
                  {formatAuthStateLabel(authState)}
                </StatusBadge>
              </div>
            </div>
          </SidebarSection>
        </div>
      </aside>

      <section className="shell-content">
        {notice ? (
          <AlertBanner
            notice={notice}
            busyLabel={sessionLoading ? busyLabel : undefined}
          />
        ) : sessionLoading ? (
          <AlertBanner busyLabel={busyLabel} />
        ) : null}

        <Outlet />
      </section>
    </main>
  );
}

function isSidebarItemActive(
  pathname: string,
  hash: string,
  item: {
    activePath?: string;
    hashes?: string[];
    matchPrefix?: string;
    to: string;
  },
) {
  const activePath = item.activePath ?? item.to;

  if (item.hashes !== undefined) {
    return pathname === activePath && item.hashes.includes(hash);
  }

  return pathname === activePath || (
    item.matchPrefix !== undefined && pathname.startsWith(item.matchPrefix)
  );
}
