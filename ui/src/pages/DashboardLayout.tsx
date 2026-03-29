import { useEffect, useState } from "react";
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  dashboardPartnerBoardNavigationItem,
  dashboardHomeNavigationItem,
  dashboardSidebarNavigationSections,
  getActiveNavigationSection,
  isNavigationItemActive,
} from "../app/navigation";
import { appRoutes } from "../app/routes";
import { formatAuthStateLabel, formatSessionTitle } from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { AlertBanner, IdentityAvatar, StatusBadge } from "../components/ui";
import "../shell.css";

const siteBrandIconSrc = `${import.meta.env.BASE_URL}brand/alicebot.webp`;

export function DashboardLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const activeSection = getActiveNavigationSection(location.pathname);
  const {
    authState,
    accessibleGuilds,
    beginLogin,
    busyLabel,
    notice,
    selectedGuildID,
    session,
    sessionAvatarURL,
    sessionLoading,
    setSelectedGuildID,
    logout,
  } = useDashboardSession();
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [openSectionID, setOpenSectionID] = useState<string | null>(
    activeSection?.id ?? null,
  );

  useEffect(() => {
    setAccountMenuOpen(false);
  }, [location.pathname, location.search, location.hash, authState]);

  useEffect(() => {
    setOpenSectionID(activeSection?.id ?? null);
  }, [activeSection?.id, location.pathname]);

  async function handleSignOut() {
    setAccountMenuOpen(false);
    await logout();
    navigate(appRoutes.landing);
  }

  async function handleSignIn() {
    setAccountMenuOpen(false);
    await beginLogin(getNextPath(location));
  }

  const accountTitle =
    session !== null
      ? formatSessionTitle(session)
      : formatAuthStateLabel(authState);
  const accountSubtitle =
    authState === "signed_in"
      ? "Discord account"
      : authState === "oauth_unavailable"
        ? "OAuth unavailable"
        : "Sign in to continue";
  const accountTone = authState === "signed_in" ? "success" : "info";
  const canSelectGuild =
    authState === "signed_in" && accessibleGuilds.length > 0;

  function toggleSection(sectionID: string) {
    setOpenSectionID((current) => (current === sectionID ? null : sectionID));
  }

  return (
    <main className="dashboard-layout-shell">
      <header className="shell-topbar" data-shell-topbar>
        <Link className="shell-brand" to={appRoutes.dashboardHome}>
          <span className="shell-brand-mark" aria-hidden="true">
            <img src={siteBrandIconSrc} alt="" />
          </span>
          <span className="shell-brand-copy">
            <strong>Discordcore</strong>
            <small>Dashboard</small>
          </span>
        </Link>

        <div className="shell-topbar-spacer" aria-hidden="true" />

        <label className="shell-server-select">
          <span className="shell-field-label">Server</span>
          <select
            value={selectedGuildID}
            onChange={(event) => setSelectedGuildID(event.target.value)}
            disabled={!canSelectGuild}
          >
            <option value="">
              {authState !== "signed_in"
                ? "Sign in to load servers"
                : "Choose a server"}
            </option>
            {accessibleGuilds.map((guild) => (
              <option key={guild.id} value={guild.id}>
                {guild.name}
              </option>
            ))}
          </select>
        </label>

        <div className="shell-account">
          <button
            className="shell-account-trigger"
            type="button"
            aria-haspopup="menu"
            aria-expanded={accountMenuOpen}
            aria-controls="shell-account-menu"
            disabled={sessionLoading}
            onClick={() => setAccountMenuOpen((current) => !current)}
          >
            <IdentityAvatar imageUrl={sessionAvatarURL} label={accountTitle} />
            <span className="shell-account-trigger-copy">
              <strong>{accountTitle}</strong>
              <span>{accountSubtitle}</span>
            </span>
            <span className="shell-account-trigger-meta">
              <StatusBadge tone={accountTone}>
                {formatAuthStateLabel(authState)}
              </StatusBadge>
              <span className="shell-account-trigger-caret" aria-hidden="true">
                ▾
              </span>
            </span>
          </button>

          {accountMenuOpen ? (
            <div className="shell-account-menu" id="shell-account-menu">
              <div className="shell-account-menu-header">
                <StatusBadge tone={accountTone}>
                  {formatAuthStateLabel(authState)}
                </StatusBadge>
                <strong>{accountTitle}</strong>
              </div>
              <p className="shell-account-menu-note">
                {authState === "signed_in"
                  ? "Signed in with Discord. Sign out to switch accounts."
                  : "Sign in with Discord to access the dashboard."}
              </p>
              <div className="shell-account-menu-actions">
                {authState === "signed_in" ? (
                  <button
                    className="button-secondary"
                    type="button"
                    disabled={sessionLoading}
                    onClick={() => void handleSignOut()}
                  >
                    Sign out
                  </button>
                ) : (
                  <button
                    className="button-primary"
                    type="button"
                    disabled={sessionLoading}
                    onClick={() => void handleSignIn()}
                  >
                    Sign in with Discord
                  </button>
                )}
              </div>
            </div>
          ) : null}
        </div>
      </header>

      <div className="shell-body">
        <aside
          className="dashboard-layout-sidebar"
          aria-label="Dashboard navigation"
        >
          <nav className="shell-nav" aria-label="Dashboard navigation">
            <Link
              className={`shell-nav-link shell-nav-link-root${
                isNavigationItemActive(location.pathname, dashboardHomeNavigationItem)
                  ? " is-active"
                  : ""
              }`}
              to={dashboardHomeNavigationItem.to}
              aria-current={
                isNavigationItemActive(location.pathname, dashboardHomeNavigationItem)
                  ? "page"
                  : undefined
              }
            >
              <span>{dashboardHomeNavigationItem.label}</span>
            </Link>

            {dashboardSidebarNavigationSections.map((section) => {
              const isOpen = openSectionID === section.id;
              const hasActiveItem = activeSection?.id === section.id;

              return (
                <section className="shell-nav-section" key={section.id}>
                  <button
                    className={`shell-nav-section-trigger${
                      hasActiveItem ? " is-active" : ""
                    }`}
                    type="button"
                    aria-expanded={isOpen}
                    aria-controls={`shell-nav-section-${section.id}`}
                    onClick={() => toggleSection(section.id)}
                  >
                    <span>{section.label}</span>
                    <span
                      className={`shell-nav-section-indicator${
                        isOpen ? " is-open" : ""
                      }`}
                      aria-hidden="true"
                    >
                      ▾
                    </span>
                  </button>

                  {isOpen ? (
                    <div
                      className="shell-nav-list"
                      id={`shell-nav-section-${section.id}`}
                    >
                      {section.items.map((item) => {
                        const isActive = isNavigationItemActive(location.pathname, item);

                        return (
                          <Link
                            key={item.label}
                            className={`shell-nav-link shell-nav-link-sub${
                              isActive ? " is-active" : ""
                            }`}
                            to={item.to}
                            aria-current={isActive ? "page" : undefined}
                          >
                            <span>{item.label}</span>
                          </Link>
                        );
                      })}
                    </div>
                  ) : null}

                  {section.id === "moderation" ? (
                    <Link
                      className={`shell-nav-link shell-nav-link-root${
                        isNavigationItemActive(
                          location.pathname,
                          dashboardPartnerBoardNavigationItem,
                        )
                          ? " is-active"
                          : ""
                      }`}
                      to={dashboardPartnerBoardNavigationItem.to}
                      aria-current={
                        isNavigationItemActive(
                          location.pathname,
                          dashboardPartnerBoardNavigationItem,
                        )
                          ? "page"
                          : undefined
                      }
                    >
                      <span>{dashboardPartnerBoardNavigationItem.label}</span>
                    </Link>
                  ) : null}
                </section>
              );
            })}
          </nav>
        </aside>

        <section className="shell-main">
          {notice ? (
            <div className="shell-main-notice">
              <AlertBanner
                notice={notice}
                busyLabel={sessionLoading ? busyLabel : undefined}
              />
            </div>
          ) : sessionLoading ? (
            <div className="shell-main-notice">
              <AlertBanner busyLabel={busyLabel} />
            </div>
          ) : null}
          <Outlet />
        </section>
      </div>
    </main>
  );
}

function getNextPath(location: ReturnType<typeof useLocation>) {
  return `${location.pathname}${location.search}${location.hash}`;
}
